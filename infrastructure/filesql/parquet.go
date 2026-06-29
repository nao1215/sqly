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
	if len(table.Records()) == 0 {
		return errors.New("cannot export an empty result to parquet: parquet export requires at least one row")
	}

	tmpDir, err := os.MkdirTemp("", "sqly-parquet-")
	if err != nil {
		return fmt.Errorf("create temp dir for parquet dump: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	db, err := sql.Open("sqlite", filepath.Join(tmpDir, "stage.db"))
	if err != nil {
		return fmt.Errorf("open staging database: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close staging database: %w", cerr)
		}
	}()

	ctx := context.Background()
	if _, err = db.ExecContext(ctx, parquetStagingCreateTable(table)); err != nil {
		return fmt.Errorf("create staging table: %w", err)
	}
	// Insert in a single transaction; one implicit transaction per row is slow
	// for large exports.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staging transaction: %w", err)
	}
	for rowIdx := range table.Records() {
		if _, err = tx.ExecContext(ctx, parquetInsertStatement(table, rowIdx)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert into staging table: %w", err)
		}
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

// parquetStagingCreateTable builds the staging CREATE TABLE for a parquet export
// with every column typed TEXT. The shared GenerateCreateTableStatement infers an
// INTEGER column when all values parse as numbers, which makes SQLite's column
// affinity rewrite numeric-looking text such as a leading-zero code ("007") or a
// decimal string ("1.00") into a number before the parquet writer sees it.
// Staging every column as TEXT keeps the original text verbatim through the
// round-trip; re-import still types a canonical number when the reader asks for
// typed output.
func parquetStagingCreateTable(t *model.Table) string {
	var b strings.Builder
	b.WriteString("CREATE TABLE " + infra.Quote(t.Name()) + " (")
	for i, col := range t.Header() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(infra.Quote(col) + " TEXT")
	}
	b.WriteString(");")
	return b.String()
}

// parquetInsertStatement builds the INSERT for one staged row, emitting SQL NULL
// for cells the table marks as NULL. The shared GenerateInsertStatement only sees
// the []string record and single-quotes every cell, which collapses a NULL into
// an empty string; consulting table.IsNull here keeps NULL and "" distinct
// through the parquet round-trip.
func parquetInsertStatement(t *model.Table, rowIdx int) string {
	record := t.Records()[rowIdx]
	var b strings.Builder
	b.WriteString("INSERT INTO " + infra.Quote(t.Name()) + " VALUES (")
	for col := range t.Header() {
		if col > 0 {
			b.WriteString(", ")
		}
		if t.IsNull(rowIdx, col) {
			b.WriteString("NULL")
			continue
		}
		var v string
		if col < len(record) {
			v = record[col]
		}
		b.WriteString(infra.SingleQuote(v))
	}
	b.WriteString(");")
	return b.String()
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
