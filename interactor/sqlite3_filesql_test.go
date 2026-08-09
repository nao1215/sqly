package interactor

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/memory"
	_ "modernc.org/sqlite"
)

func newTestSQLite3InteractorWithAdapter(t *testing.T) (*SQLite3Interactor, func()) {
	t.Helper()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	// The repository and the adapter read and write the same database, which is
	// what a session does: files are loaded through the adapter and listed back
	// through the repository.
	si := &SQLite3Interactor{
		r:       memory.NewSQLite3Repository(config.MemoryDB(sharedDB)),
		sql:     NewSQL(),
		adapter: filesql.NewFileSQLAdapter(sharedDB),
	}
	return si, func() {
		if err := sharedDB.Close(); err != nil {
			t.Logf("failed to close shared DB: %v", err)
		}
	}
}

func TestSQLite3Interactor_LoadFiles(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "users.csv")
	if err := os.WriteFile(csvPath, []byte("id,name\n1,Alice\n2,Bob\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := si.LoadFiles(ctx, csvPath); err != nil {
		t.Fatalf("LoadFiles: %v", err)
	}

	tables, err := si.TablesName(ctx)
	if err != nil {
		t.Fatalf("TablesName: %v", err)
	}
	if len(tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(tables))
	}
	if len(tables) > 0 && tables[0].Name() != "users" {
		t.Errorf("expected table name 'users', got %q", tables[0].Name())
	}
}

// TestSQLite3Interactor_ACHImportListsOnlyItsOwnTables pins that filesql's own
// bookkeeping table stays out of the session's listing. Loading an ACH or
// Fedwire file makes filesql record where it came from, in a table of its own
// inside the same database, and a session that listed it would offer the user a
// table that is not theirs, and that .save would then try to write out.
func TestSQLite3Interactor_ACHImportListsOnlyItsOwnTables(t *testing.T) {
	t.Parallel()

	achPath := filepath.Join("..", "testdata", "ppd-debit.ach")
	if _, err := os.Stat(achPath); os.IsNotExist(err) {
		t.Skip("ACH test data not available")
	}

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	ctx := context.Background()
	if err := si.LoadFiles(ctx, achPath); err != nil {
		t.Fatalf("LoadFiles: %v", err)
	}

	tables, err := si.TablesName(ctx)
	if err != nil {
		t.Fatalf("TablesName: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("TablesName returned nothing for an imported ACH file")
	}
	for _, table := range tables {
		if strings.HasPrefix(table.Name(), "_filesql_") {
			t.Errorf("TablesName listed filesql's own table %q", table.Name())
		}
	}
}

// TestSQLite3Interactor_TablesName_Empty checks the listing of a session that
// has loaded nothing: an empty result rather than an error.
func TestSQLite3Interactor_TablesName_Empty(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	tables, err := si.TablesName(context.Background())
	if err != nil {
		t.Fatalf("TablesName: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(tables))
	}
}

func TestSQLite3Interactor_IsSupportedFile(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"csv is supported", "test.csv", true},
		{"tsv is supported", "test.tsv", true},
		{"ltsv is supported", "test.ltsv", true},
		{"json is supported", "test.json", true},
		{"jsonl is supported", "test.jsonl", true},
		{"parquet is supported", "test.parquet", true},
		{"xlsx is supported", "test.xlsx", true},
		{"compressed csv is supported", "test.csv.gz", true},
		{"txt is not supported", "test.txt", false},
		{"no extension is not supported", "test", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := si.IsSupportedFile(tt.path); got != tt.want {
				t.Errorf("IsSupportedFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSQLite3Interactor_IsExcelFile(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"xlsx is excel", "test.xlsx", true},
		{"compressed xlsx is excel", "test.xlsx.gz", true},
		{"csv is not excel", "test.csv", false},
		{"json is not excel", "test.json", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := si.IsExcelFile(tt.path); got != tt.want {
				t.Errorf("IsExcelFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSQLite3Interactor_QuoteIdentifier(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	if got := si.QuoteIdentifier("table_name"); got != `"table_name"` {
		t.Errorf("QuoteIdentifier = %q, want %q", got, `"table_name"`)
	}
}

func TestSQLite3Interactor_GetTableNameFromFilePath(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	tests := []struct {
		path string
		want string
	}{
		{"users.csv", "users"},
		{"sales_q1.xlsx", "sales_q1"},
		{"/path/to/data.tsv", "data"},
	}
	for _, tt := range tests {
		if got := si.GetTableNameFromFilePath(tt.path); got != tt.want {
			t.Errorf("GetTableNameFromFilePath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestSQLite3Interactor_LoadFiles_PreservesFilesqlSchema locks the integration
// model for issue: filesql detects column types, and sqly copies the exact
// CREATE TABLE into the shared DB, so the detected types survive. This is the
// schema fidelity that .schema/.describe and export/write-back (,
// ) depend on.
func TestSQLite3Interactor_LoadFiles_PreservesFilesqlSchema(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open shared DB: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	si := &SQLite3Interactor{r: nil, sql: NewSQL(), adapter: filesql.NewFileSQLAdapter(sharedDB)}

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "typed.csv")
	if err := os.WriteFile(csvPath, []byte("id,price,name\n1,9.99,apple\n2,3.50,pear\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := si.LoadFiles(ctx, csvPath); err != nil {
		t.Fatalf("LoadFiles: %v", err)
	}

	rows, err := sharedDB.QueryContext(ctx, "PRAGMA table_info('typed')")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer func() { _ = rows.Close() }()

	got := map[string]string{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, colType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		got[name] = colType
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"id": "INTEGER", "price": "REAL", "name": "TEXT"}
	for col, wantType := range want {
		if got[col] != wantType {
			t.Errorf("column %q type = %q, want %q (filesql schema not preserved in shared DB)", col, got[col], wantType)
		}
	}
}
