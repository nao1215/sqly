package memory

import (
	"context"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// covMemNewRepo returns a fresh in-memory repository backed by its own database
// so that subtests can run in parallel without sharing state.
func covMemNewRepo(t *testing.T) repository.SQLite3Repository {
	t.Helper()
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return NewSQLite3Repository(memoryDB)
}

func TestSqlite3Repository_List_MissingTableReturnsError(t *testing.T) {
	t.Parallel()

	r := covMemNewRepo(t)
	if _, err := r.List(context.Background(), "no_such_table"); err == nil {
		t.Error("expected error listing a missing table, got nil")
	}
}

func TestSqlite3Repository_Header_MissingTableReturnsError(t *testing.T) {
	t.Parallel()

	r := covMemNewRepo(t)
	if _, err := r.Header(context.Background(), "no_such_table"); err == nil {
		t.Error("expected error reading header of a missing table, got nil")
	}
}

func TestSqlite3Repository_Header_EmptyTableReturnsHeaderOnly(t *testing.T) {
	t.Parallel()

	r := covMemNewRepo(t)
	if _, err := r.Exec(context.Background(), "CREATE TABLE empty_t (a TEXT, b TEXT)"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Header(context.Background(), "empty_t")
	if err != nil {
		t.Fatalf("Header error = %v, want nil", err)
	}
	want := model.Header{"a", "b"}
	if len(got.Header()) != len(want) || got.Header()[0] != "a" || got.Header()[1] != "b" {
		t.Errorf("Header = %v, want %v", got.Header(), want)
	}
	if len(got.Records()) != 0 {
		t.Errorf("Records = %v, want empty", got.Records())
	}
}

func TestSqlite3Repository_Query_NullValuesFlagged(t *testing.T) {
	t.Parallel()

	r := covMemNewRepo(t)
	if _, err := r.Exec(context.Background(), "CREATE TABLE nt (a TEXT, b TEXT)"); err != nil {
		t.Fatal(err)
	}
	// One row with a real NULL and one with an empty string, so the NULL tracking
	// branch in Query is exercised.
	if _, err := r.Exec(context.Background(), "INSERT INTO nt (a, b) VALUES (NULL, '')"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Query(context.Background(), "SELECT a, b FROM nt")
	if err != nil {
		t.Fatalf("Query error = %v, want nil", err)
	}
	if len(got.Records()) != 1 {
		t.Fatalf("record count = %d, want 1", len(got.Records()))
	}
	// Column a is a real NULL; column b is an empty string, not NULL.
	if !got.IsNull(0, 0) {
		t.Error("IsNull(0,0) = false, want true for the NULL cell")
	}
	if got.IsNull(0, 1) {
		t.Error("IsNull(0,1) = true, want false for the empty-string cell")
	}
}

func TestSqlite3Repository_ResolveTableRef_LiteralDottedName(t *testing.T) {
	t.Parallel()

	r := covMemNewRepo(t)
	// A table whose literal name contains a dot must be resolved by List without
	// being misread as a schema-qualified reference.
	if _, err := r.Exec(context.Background(), `CREATE TABLE "main.dotted" (id INTEGER, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Exec(context.Background(), `INSERT INTO "main.dotted" VALUES (1, 'x')`); err != nil {
		t.Fatal(err)
	}
	got, err := r.List(context.Background(), "main.dotted")
	if err != nil {
		t.Fatalf("List(main.dotted) error = %v, want nil", err)
	}
	if len(got.Records()) != 1 || got.Records()[0][1] != "x" {
		t.Errorf("records = %v, want one row with x", got.Records())
	}
}
