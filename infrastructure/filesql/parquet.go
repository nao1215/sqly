package filesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	libfilesql "github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/cleanup"
	"github.com/nao1215/sqly/domain/model"
	infra "github.com/nao1215/sqly/infrastructure"
	_ "modernc.org/sqlite" // register the "sqlite" driver used for the staging DB
)

// DumpTableToParquet writes a single table to a Parquet file at filePath.
//
// filesql can read Parquet but only exposes a whole-database dump
// (DumpDatabase writes one file per table into a directory), so sqly stages the
// table in a temporary single-table SQLite database, dumps it to Parquet, and
// moves the one produced file to filePath. A file-backed temporary database is
// used because filesql's dumper opens a second connection, which would deadlock
// a single-connection in-memory database and would not be shared otherwise.
func DumpTableToParquet(filePath string, table *model.Table) (err error) {
	// filesql's Parquet writer needs at least one row to infer the column
	// schema and rejects an empty source. Surface that limitation with a clear
	// message instead of filesql's internal error.
	if table.RowCount() == 0 {
		return errors.New("cannot export an empty result to parquet: parquet export requires at least one row")
	}

	// A Parquet schema names each column once, and the staging database below
	// rejects a repeat too — in its own words, which describe a database the user
	// never opened: "create staging table: SQL logic error: duplicate column name:
	// x (1)". Say it here instead, about the query that produced the columns.
	if name, dup := duplicateColumnName(table); dup {
		return fmt.Errorf("parquet: duplicate column name %q; a parquet schema names each column once, so alias one of them (SELECT a AS a1, b AS a2)", name)
	}

	tmpDir, err := os.MkdirTemp("", "sqly-parquet-")
	if err != nil {
		return fmt.Errorf("create temp dir for parquet dump: %w", err)
	}
	defer func() {
		// The staging directory holds the intermediate SQLite database; leaving
		// it behind is a leak the caller should hear about even when the export
		// itself failed.
		err = cleanup.Join(err, os.RemoveAll(tmpDir), "remove parquet staging directory")
	}()

	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "stage.db"))
	if err != nil {
		return fmt.Errorf("open staging database: %w", err)
	}
	defer func() {
		err = cleanup.Join(err, db.Close(), "close parquet staging database")
	}()

	ctx := context.Background()
	stagingTypes := parquetStagingTypes(table)
	if _, err = db.ExecContext(ctx, parquetStagingCreateTable(table, stagingTypes)); err != nil {
		return fmt.Errorf("create staging table: %w", err)
	}
	// Insert in a single transaction; one implicit transaction per row is slow
	// for large exports.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staging transaction: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, parquetInsertStatement(table))
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare staging insert: %w", err)
	}
	for rowIdx := range table.RowCount() {
		if _, err = stmt.ExecContext(ctx, parquetInsertArgs(table, rowIdx, stagingTypes)...); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return fmt.Errorf("insert into staging table: %w", err)
		}
	}
	if err = stmt.Close(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("close staging insert: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit staging transaction: %w", err)
	}

	outDir := filepath.Join(tmpDir, "out")
	opts := libfilesql.NewDumpOptions().WithFormat(libfilesql.OutputFormatParquet)
	if err = libfilesql.DumpDatabase(db, outDir, opts); err != nil {
		return fmt.Errorf("dump table to parquet: %w", err)
	}

	produced, err := filepath.Glob(filepath.Join(outDir, "*.parquet"))
	if err != nil {
		return fmt.Errorf("locate parquet output: %w", err)
	}
	if len(produced) != 1 {
		return fmt.Errorf("expected 1 parquet file, got %d", len(produced))
	}
	if err = copyFile(produced[0], filePath); err != nil {
		return fmt.Errorf("write parquet to %q: %w", filePath, err)
	}
	return nil
}

// parquetStagingCreateTable builds the staging CREATE TABLE for a parquet
// export, one declared type per column. The shared
// GenerateCreateTableStatement is not used because it infers an INTEGER column
// when all values parse as numbers, which makes SQLite's column affinity
// rewrite numeric-looking text such as a leading-zero code ("007") or a decimal
// string ("1.00") into a number before the parquet writer sees it.
// parquetStagingColumnType decides instead.
func parquetStagingCreateTable(t *model.Table, types []string) string {
	var b strings.Builder
	b.WriteString("CREATE TABLE " + infra.Quote(t.Name()) + " (")
	for i, col := range t.Header() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(infra.Quote(col) + " " + types[i])
	}
	b.WriteString(");")
	return b.String()
}

// duplicateColumnName returns the first column name that appears twice, and
// reports whether there was one.
func duplicateColumnName(t *model.Table) (string, bool) {
	seen := make(map[string]struct{}, t.ColumnCount())
	for _, name := range t.Header() {
		if _, ok := seen[name]; ok {
			return name, true
		}
		seen[name] = struct{}{}
	}
	return "", false
}

const parquetTextType = "TEXT"

// parquetStagingTypes is the staging type of every column, computed once because
// both the CREATE TABLE and the bind values depend on it.
func parquetStagingTypes(t *model.Table) []string {
	types := make([]string, t.ColumnCount())
	for i := range types {
		types[i] = parquetStagingColumnType(t, i)
	}
	return types
}

// parquetStagingColumnType is the SQL type the staging table declares for one
// column, taken from the values the query actually produced.
//
// Every column used to be declared TEXT, and the display strings were bound into
// it, so a numeric result reached Parquet as a column of digits typed as text —
// and a reader that trusts the schema, which is what a typed format is for, then
// compared and sorted it as text. A column is numeric only when every value in
// it is, because SQLite types values rather than columns, and a column that
// mixes a number with text has to keep the text.
func parquetStagingColumnType(t *model.Table, col int) string {
	const text = parquetTextType
	declared := ""
	for row := range t.RowCount() {
		cell, ok := t.NativeCell(row, col)
		if !ok {
			// No native values at all: the table came from strings, so TEXT is
			// what it holds.
			return text
		}
		if cell.IsNull() {
			continue
		}
		var kind string
		switch cell.Value().(type) {
		case int64:
			kind = "INTEGER"
		case float64:
			kind = "REAL"
		default:
			return text
		}
		switch {
		case declared == "":
			declared = kind
		case declared != kind:
			// INTEGER and REAL in one column widen to REAL, which holds both.
			declared = "REAL"
		}
	}
	if declared == "" {
		return text
	}
	return declared
}

// parquetInsertStatement builds the staging INSERT, one placeholder per column.
//
// The values are bound rather than written into the statement. Quoting them into
// SQL text made the export depend on every value being parsable as a SQL
// literal, and a value is data: a NUL byte ends a statement as far as SQLite's
// tokenizer is concerned, so a row carrying one — which CSV and every other
// format export without complaint — left the literal unclosed and failed the
// export with "unrecognized token". Binding also parses the statement once for
// the whole export instead of once per row.
func parquetInsertStatement(t *model.Table) string {
	var b strings.Builder
	b.WriteString("INSERT INTO " + infra.Quote(t.Name()) + " VALUES (")
	for col := range t.Header() {
		if col > 0 {
			b.WriteString(", ")
		}
		b.WriteString("?")
	}
	b.WriteString(");")
	return b.String()
}

// parquetInsertArgs returns one row's bind values, nil for the cells the table
// marks as NULL. The shared GenerateInsertStatement only sees the []string
// record and cannot tell a NULL from an empty string; consulting table.IsNull
// here keeps the two distinct through the parquet round-trip.
func parquetInsertArgs(t *model.Table, rowIdx int, types []string) []any {
	record, _ := t.Row(rowIdx)
	args := make([]any, 0, len(t.Header()))
	for col := range t.Header() {
		if t.IsNull(rowIdx, col) {
			args = append(args, nil)
			continue
		}
		// Binding the driver's value into a TEXT column would hand the
		// conversion to SQLite, whose rendering of a real is not sqly's: a
		// result printed as 100000 was exported as "100000.0".
		if types[col] == parquetTextType {
			args = append(args, record.At(col))
			continue
		}
		// Bind the driver's own value when there is one, so an INTEGER stays an
		// integer through the staging table and into the Parquet schema. The
		// display string is the fallback for a table built from strings, where
		// there is no other value to bind.
		if cell, ok := t.NativeCell(rowIdx, col); ok {
			args = append(args, cell.Value())
			continue
		}
		args = append(args, record.At(col))
	}
	return args
}

// copyFile copies src to dst. A copy (not rename) is used because the temporary
// directory and the destination may live on different filesystems.
func copyFile(src, dst string) (err error) {
	data, err := os.ReadFile(src) //nolint:gosec // src is a sqly-generated temp path
	if err != nil {
		return err
	}
	// dst is the user-chosen output path (already filepath.Clean'd by the caller);
	// writing there is the intended behavior of an export command.
	if err = os.WriteFile(dst, data, 0o600); err != nil { //nolint:gosec // user-specified output path
		return err
	}
	return nil
}
