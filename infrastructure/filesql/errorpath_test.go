package filesql

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

// covErrFsqlClosedAdapter returns an adapter whose shared database has already
// been closed, so every statement it issues fails. It is the closed-DB variant of
// covFsqlNewAdapter used to drive the error branches.
func covErrFsqlClosedAdapter(t *testing.T) *FileSQLAdapter {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	a := NewFileSQLAdapter(db)
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	return a
}

// TestLoadFiles_EmptyJSONClosedDB covers the LoadFiles branch that surfaces a
// createEmptyJSONTable failure: an empty JSON input is handled by creating a
// zero-row table, and that CREATE/DROP fails against a closed database.
func TestLoadFiles_EmptyJSONClosedDB(t *testing.T) {
	t.Parallel()

	a := covErrFsqlClosedAdapter(t)
	emptyJSON := covFsqlWriteCSV(t, "empty.json", "[]")

	if err := a.LoadFiles(context.Background(), emptyJSON); err == nil {
		t.Fatal("LoadFiles(empty JSON) on closed DB = nil error, want error")
	}
}

// TestSnapshotToCache_RemoveStaleError covers the branch where the stale cache at
// the destination cannot be removed: pointing at a non-empty directory makes
// os.Remove fail with an error that is not os.ErrNotExist.
func TestSnapshotToCache_RemoveStaleError(t *testing.T) {
	t.Parallel()

	a := covFsqlNewAdapter(t)
	ctx := context.Background()
	csv := covFsqlWriteCSV(t, "snap.csv", "id\n1\n")
	if err := a.LoadFile(ctx, csv); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// A non-empty directory at the cache path cannot be removed by os.Remove,
	// so the stale-cache removal step fails.
	dir := filepath.Join(t.TempDir(), "cachedir")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed dir: %v", err)
	}

	if err := a.SnapshotToCache(ctx, dir); err == nil {
		t.Fatal("SnapshotToCache targeting a non-empty directory = nil error, want error")
	}
}

// TestLoadFromCache_AttachError covers the ATTACH failure branch: a file that
// exists but is not a SQLite database passes the stat check and then fails to
// attach.
func TestLoadFromCache_AttachError(t *testing.T) {
	t.Parallel()

	a := covFsqlNewAdapter(t)
	garbage := covFsqlWriteCSV(t, "garbage.db", "this is plain text, not a sqlite database")

	if err := a.LoadFromCache(context.Background(), garbage); err == nil {
		t.Fatal("LoadFromCache on a non-database file = nil error, want error")
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
