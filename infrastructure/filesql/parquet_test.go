package filesql

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	libfilesql "github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

// reimportRowCount writes the parquet file back into a fresh database via
// filesql and returns the row count of the given table.
func reimportRowCount(t *testing.T, parquetPath, tableName string) int {
	t.Helper()
	db, err := libfilesql.OpenContext(context.Background(), parquetPath)
	if err != nil {
		t.Fatalf("reimport parquet: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+tableName).Scan(&n); err != nil {
		t.Fatalf("count after reimport: %v", err)
	}
	return n
}

// TestDumpTableToParquet_RoundTrip locks the export target: a table
// written to Parquet must re-import into sqly with the same rows and columns.
func TestDumpTableToParquet_RoundTrip(t *testing.T) {
	t.Parallel()

	table := model.NewTable("people", model.Header{"id", "name"}, []model.Record{
		{"1", "alice"},
		{"2", "bob"},
		{"3", "carol"},
	})
	out := filepath.Join(t.TempDir(), "people.parquet")

	if err := DumpTableToParquet(out, table); err != nil {
		t.Fatalf("DumpTableToParquet: %v", err)
	}

	if got := reimportRowCount(t, out, "people"); got != 3 {
		t.Errorf("reimported rows = %d, want 3", got)
	}

	// Schema fidelity: the reimported table exposes the same columns.
	db, err := libfilesql.OpenContext(context.Background(), out)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info('people')")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 || cols[0] != "id" || cols[1] != "name" {
		t.Errorf("reimported columns = %v, want [id name]", cols)
	}
}

// TestDumpTableToParquet_EmptyResult covers the empty-result behavior: Parquet
// needs at least one row to infer its schema, so exporting an empty result
// returns a clear error rather than writing an unreadable file.
func TestDumpTableToParquet_EmptyResult(t *testing.T) {
	t.Parallel()

	table := model.NewTable("empty", model.Header{"a", "b"}, []model.Record{})
	out := filepath.Join(t.TempDir(), "empty.parquet")

	err := DumpTableToParquet(out, table)
	if err == nil {
		t.Fatal("DumpTableToParquet on empty result = nil error, want error")
	}
	if !strings.Contains(err.Error(), "empty result") {
		t.Errorf("error = %q, want it to mention the empty result", err.Error())
	}
}
