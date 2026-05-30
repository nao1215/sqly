package filesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	if _, err = db.ExecContext(ctx, infra.GenerateCreateTableStatement(table)); err != nil {
		return fmt.Errorf("create staging table: %w", err)
	}
	// Insert in a single transaction; one implicit transaction per row is slow
	// for large exports.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staging transaction: %w", err)
	}
	for _, record := range table.Records() {
		if _, err = tx.ExecContext(ctx, infra.GenerateInsertStatement(table.Name(), record)); err != nil {
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
