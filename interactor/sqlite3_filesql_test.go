package interactor

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/infrastructure/filesql"
	_ "modernc.org/sqlite"
)

func newTestSQLite3InteractorWithAdapter(t *testing.T) (*sqlite3Interactor, func()) {
	t.Helper()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	adapter := filesql.NewFileSQLAdapter(sharedDB)
	si := &sqlite3Interactor{
		r:       nil,
		sql:     NewSQL(),
		adapter: adapter,
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
	if err := os.WriteFile(csvPath, []byte("id,name\n1,Alice\n2,Bob\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := si.LoadFiles(ctx, csvPath); err != nil {
		t.Fatalf("LoadFiles: %v", err)
	}

	tables, err := si.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames: %v", err)
	}
	if len(tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(tables))
	}
	if len(tables) > 0 && tables[0].Name() != "users" {
		t.Errorf("expected table name 'users', got %q", tables[0].Name())
	}
}

func TestSQLite3Interactor_GetTableNames_Empty(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	tables, err := si.GetTableNames(context.Background())
	if err != nil {
		t.Fatalf("GetTableNames: %v", err)
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

func TestSQLite3Interactor_SanitizeForSQL(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	if got := si.SanitizeForSQL("My Sheet-1"); got != "My_Sheet_1" {
		t.Errorf("SanitizeForSQL = %q, want %q", got, "My_Sheet_1")
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
