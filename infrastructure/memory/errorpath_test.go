package memory

import (
	"context"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/repository"
)

// covMemClosedRepo returns a repository whose backing in-memory database has
// already been closed. Every DB call the repository makes therefore hits the
// BeginTx/Query/Exec error-return branch, letting a subtest assert that the
// method reports the failure instead of panicking. Each caller gets its own
// closed database so parallel subtests never share state.
func covMemClosedRepo(t *testing.T) repository.SQLite3Repository {
	t.Helper()
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		t.Fatal(err)
	}
	// cleanup closes the database; call it now so subsequent method calls fail.
	cleanup()
	return NewSQLite3Repository(memoryDB)
}

func TestSqlite3Repository_ClosedDB_ReturnsErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("TablesName returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covMemClosedRepo(t)
		if _, err := r.TablesName(ctx); err == nil {
			t.Error("expected error from TablesName on a closed DB, got nil")
		}
	})

	t.Run("SchemaObjects returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covMemClosedRepo(t)
		if _, err := r.SchemaObjects(ctx); err == nil {
			t.Error("expected error from SchemaObjects on a closed DB, got nil")
		}
	})

	t.Run("List returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covMemClosedRepo(t)
		// List first resolves the table reference, which itself touches the DB.
		if _, err := r.List(ctx, "any_table"); err == nil {
			t.Error("expected error from List on a closed DB, got nil")
		}
	})

	t.Run("Header returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covMemClosedRepo(t)
		// Header also resolves the table reference before querying.
		if _, err := r.Header(ctx, "any_table"); err == nil {
			t.Error("expected error from Header on a closed DB, got nil")
		}
	})

	t.Run("Query returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covMemClosedRepo(t)
		if _, err := r.Query(ctx, "SELECT 1"); err == nil {
			t.Error("expected error from Query on a closed DB, got nil")
		}
	})

	t.Run("Exec returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covMemClosedRepo(t)
		if _, err := r.Exec(ctx, "CREATE TABLE t (a TEXT)"); err == nil {
			t.Error("expected error from Exec on a closed DB, got nil")
		}
	})
}
