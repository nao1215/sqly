package filesql

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

// covErrFsqlClosedAdapter returns an adapter whose shared database has already
// been closed, so every statement it issues fails. It is the closed-DB variant of
// covFsqlNewAdapter used to drive the error branches.
func covErrFsqlClosedAdapter(t *testing.T) *testAdapter {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	a := newTestAdapter(db)
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	return a
}

// TestLoadFiles_EmptyJSONClosedDB covers the LoadFiles error path for an empty
// JSON input when the target database is already closed.
func TestLoadFiles_EmptyJSONClosedDB(t *testing.T) {
	t.Parallel()

	a := covErrFsqlClosedAdapter(t)
	emptyJSON := covFsqlWriteCSV(t, "empty.json", "[]")

	if err := a.LoadFiles(context.Background(), emptyJSON); err == nil {
		t.Fatal("LoadFiles(empty JSON) on closed DB = nil error, want error")
	}
}

// TestDumpTableToParquet_CreateStagingTableError covers the staging CREATE TABLE
// failure branch: a table that has rows but no columns produces an invalid
// "CREATE TABLE x ()" statement, which SQLite rejects.
func TestDumpTableToParquet_CreateStagingTableError(t *testing.T) {
	t.Parallel()

	table := model.NewTable("x", model.Header{}, []model.Record{{"1"}})
	out := filepath.Join(t.TempDir(), "x.parquet")

	if err := DumpTableToParquet(out, table); err == nil {
		t.Fatal("DumpTableToParquet on a headerless table = nil error, want error")
	}
}
