package filesql

import (
	"compress/gzip"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	libfilesql "github.com/nao1215/filesql"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/memory"
	_ "modernc.org/sqlite"
)

// testAdapter is a FileSQLAdapter plus the read helpers the tests need.
//
// The adapter's job is loading files; reading them back is the memory
// repository's, and that is the path a session actually takes. The adapter used
// to carry its own Query/Exec/GetTableHeader with a second scan loop that only
// tests reached, so a divergence between the two readers could not have been
// noticed by anything but a user. Keeping the helpers here reads the rows
// through the production repository and leaves the adapter with the one job.
type testAdapter struct {
	*FileSQLAdapter
	repo repository.SQLite3Repository
}

// newTestAdapter returns an adapter over db together with the repository a
// session reads that same database with.
func newTestAdapter(db *sql.DB) *testAdapter {
	return &testAdapter{
		FileSQLAdapter: NewFileSQLAdapter(db),
		repo:           memory.NewSQLite3Repository(config.MemoryDB(db)),
	}
}

// LoadFile loads one file, the single-path case of LoadFiles.
func (a *testAdapter) LoadFile(ctx context.Context, filePath string) error {
	return a.LoadFiles(ctx, filePath)
}

// Query reads rows the way the session does.
func (a *testAdapter) Query(ctx context.Context, query string) (*model.Table, error) {
	return a.repo.Query(ctx, query)
}

// GetTableNames lists the tables the way the session does. The adapter used to
// answer this with a second copy of the sqlite_master query, which is how the
// two listings were free to disagree about what counted as a table.
func (a *testAdapter) GetTableNames(ctx context.Context) ([]*model.Table, error) {
	return a.repo.TablesName(ctx)
}

// Exec runs a statement the way the session does.
func (a *testAdapter) Exec(ctx context.Context, statement string) (int64, error) {
	return a.repo.Exec(ctx, statement)
}

// GetTableHeader reads a table's column names the way the session does.
func (a *testAdapter) GetTableHeader(ctx context.Context, tableName string) (*model.Table, error) {
	return a.repo.Header(ctx, tableName)
}

func TestFileSQLAdapter_LoadFile(t *testing.T) {
	t.Parallel()

	// Create temporary test CSV file
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test.csv")

	csvContent := `name,age,city
John,25,New York
Jane,30,Los Angeles`

	if err := os.WriteFile(csvFile, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	// Create adapter
	adapter := newTestAdapter(sharedDB)

	// Test LoadFile
	ctx := context.Background()
	if err := adapter.LoadFile(ctx, csvFile); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Verify table was created
	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("No tables found after loading CSV file")
	}

	// Query the data
	table, err := adapter.Query(ctx, "SELECT * FROM "+tables[0].Name()+" ORDER BY name")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Verify data
	if len(table.Records()) != 2 {
		t.Errorf("Expected 2 records, got %d", len(table.Records()))
	}

	expectedHeaders := []string{"name", "age", "city"}
	actualHeaders := table.Header()
	if len(actualHeaders) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(actualHeaders))
	}

	for i, expected := range expectedHeaders {
		if i < len(actualHeaders) && actualHeaders[i] != expected {
			t.Errorf("Expected header %d to be %s, got %s", i, expected, actualHeaders[i])
		}
	}
}

func TestFileSQLAdapter_LoadFileWithReservedKeywords(t *testing.T) {
	t.Parallel()

	// Create temporary test CSV file with reserved keyword column names
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test_reserved.csv")

	csvContent := `Index,Order,Group,Select
1,100,A,X
2,200,B,Y`

	if err := os.WriteFile(csvFile, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	// Create adapter
	adapter := newTestAdapter(sharedDB)

	// Test LoadFile with reserved keywords
	ctx := context.Background()
	if err := adapter.LoadFile(ctx, csvFile); err != nil {
		t.Fatalf("LoadFile with reserved keywords failed: %v", err)
	}

	// Verify table was created
	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("No tables found after loading CSV file")
	}

	// Query the data - this should work with quoted column names
	table, err := adapter.Query(ctx, "SELECT * FROM "+tables[0].Name()+" ORDER BY \"Index\"")
	if err != nil {
		t.Fatalf("Query with reserved keywords failed: %v", err)
	}

	// Verify data
	if len(table.Records()) != 2 {
		t.Errorf("Expected 2 records, got %d", len(table.Records()))
	}

	expectedHeaders := []string{"Index", "Order", "Group", "Select"}
	actualHeaders := table.Header()
	if len(actualHeaders) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(actualHeaders))
	}
}

func TestFileSQLAdapter_LoadFileEmptyColumnName(t *testing.T) {
	t.Parallel()

	// Create temporary test CSV file with empty column name
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test_empty_col.csv")

	csvContent := `name,,city
John,25,New York
Jane,30,Los Angeles`

	if err := os.WriteFile(csvFile, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	// Create adapter
	adapter := newTestAdapter(sharedDB)

	// Test LoadFile with empty column name - this should handle gracefully
	ctx := context.Background()
	err = adapter.LoadFile(ctx, csvFile)
	// Note: This test depends on how filesql handles empty column names
	// It may succeed with auto-generated column names or fail
	// We test that it doesn't panic and handles the error gracefully
	if err != nil {
		// If it fails, the error should be informative
		if !strings.Contains(err.Error(), "column") {
			t.Errorf("Error should mention column issue, got: %v", err)
		}
	}
}

// TestFileSQLAdapter_LoadFileTableNamedQueryResult pins the import of a file
// whose table name begins with query_result_. sqly once materialized results
// into tables of that name and filtered the prefix out of the listing import
// diffs against, so such a file reported that it had produced no table.
func TestFileSQLAdapter_LoadFileTableNamedQueryResult(t *testing.T) {
	t.Parallel()

	csvFile := filepath.Join(t.TempDir(), "query_result_report.csv")
	if err := os.WriteFile(csvFile, []byte("id,amount\n1,100\n"), 0o600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()
	if err := adapter.LoadFile(ctx, csvFile); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}
	if len(tables) != 1 || tables[0].Name() != "query_result_report" {
		t.Errorf("GetTableNames() = %v, want [query_result_report]", tables)
	}
}

func TestNewFileSQLAdapter(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	// Test NewFileSQLAdapter
	adapter := newTestAdapter(sharedDB)

	if adapter == nil {
		t.Fatal("NewFileSQLAdapter returned nil")
	}

	if adapter.sharedDB != sharedDB {
		t.Error("NewFileSQLAdapter did not set sharedDB correctly")
	}
}

func TestGetTableNameFromFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "simple csv file",
			filePath: "/path/to/data.csv",
			expected: "data",
		},
		{
			name:     "csv with gz compression",
			filePath: "/path/to/data.csv.gz",
			expected: "data",
		},
		{
			name:     "tsv file",
			filePath: "/path/to/data.tsv",
			expected: "data",
		},
		{
			name:     "ltsv file",
			filePath: "/path/to/data.ltsv",
			expected: "data",
		},
		{
			name:     "xlsx file",
			filePath: "/path/to/data.xlsx",
			expected: "data",
		},
		{
			name:     "multiple compression extensions",
			filePath: "/path/to/data.csv.bz2",
			expected: "data",
		},
		{
			name:     "no extension",
			filePath: "/path/to/data",
			expected: "data",
		},
		{
			name:     "complex path with multiple dots",
			filePath: "/path/to/my.data.file.csv.gz",
			expected: "my_data_file",
		},
		{
			name:     "filename with hyphen (syntax error case)",
			filePath: "/path/to/bug-syntax-error.csv",
			expected: "bug_syntax_error",
		},
		{
			name:     "filename with dots and hyphens",
			filePath: "/path/to/my-data.file-test.csv",
			expected: "my_data_file_test",
		},
		{
			name:     "filename starting with number",
			filePath: "/path/to/2023-data.csv",
			expected: "sheet_2023_data",
		},
		{
			name:     "filename with special characters",
			filePath: "/path/to/data@file#test$.csv",
			expected: "datafiletest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := GetTableNameFromFilePath(tt.filePath)
			if actual != tt.expected {
				t.Errorf("GetTableNameFromFilePath(%s) = %s, expected %s", tt.filePath, actual, tt.expected)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		identifier string
		expected   string
	}{
		{
			name:       "simple identifier",
			identifier: "column_name",
			expected:   `"column_name"`,
		},
		{
			name:       "identifier with double quote",
			identifier: `foo"bar`,
			expected:   `"foo""bar"`,
		},
		{
			name:       "identifier with multiple double quotes",
			identifier: `foo"bar"baz`,
			expected:   `"foo""bar""baz"`,
		},
		{
			name:       "empty identifier",
			identifier: "",
			expected:   `""`,
		},
		{
			name:       "identifier with only double quotes",
			identifier: `""`,
			expected:   `""""""`,
		},
		{
			name:       "reserved SQL keyword",
			identifier: "SELECT",
			expected:   `"SELECT"`,
		},
		{
			name:       "identifier with spaces",
			identifier: "my column",
			expected:   `"my column"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := QuoteIdentifier(tt.identifier)
			if actual != tt.expected {
				t.Errorf("QuoteIdentifier(%q) = %q, expected %q", tt.identifier, actual, tt.expected)
			}
		})
	}
}

func TestFileSQLAdapter_LoadFileWithQuotesInColumnNames(t *testing.T) {
	t.Parallel()

	// Create temporary test CSV file with double quotes in column names
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test_quotes.csv")

	// Note: This CSV content simulates what could happen if column names contain quotes
	// In practice, this would be unusual but we need to handle it safely
	csvContent := `name,data"field,city
John,value1,New York
Jane,value2,Los Angeles`

	if err := os.WriteFile(csvFile, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	// Create adapter
	adapter := newTestAdapter(sharedDB)

	// Test LoadFile with quotes in column names
	ctx := context.Background()
	err = adapter.LoadFile(ctx, csvFile)
	// The behavior depends on how filesql handles this case
	// The important thing is that our quoting function handles it safely
	if err != nil {
		// If there's an error, it should be a meaningful one, not a SQL syntax error
		if strings.Contains(err.Error(), "syntax error") {
			t.Errorf("SQL syntax error suggests unsafe identifier quoting: %v", err)
		}
		// Other errors are acceptable as this is an edge case
		t.Logf("Expected error for unusual column names: %v", err)
		return
	}

	// If it succeeds, verify we can query the table safely
	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) > 0 {
		// Try to query the table - this should not cause SQL injection
		_, err = adapter.Query(ctx, "SELECT * FROM "+QuoteIdentifier(tables[0].Name())+" ORDER BY ROWID")
		if err != nil {
			t.Logf("Query failed (acceptable for edge case): %v", err)
		}
	}
}

func TestFileSQLAdapter_Close(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)

	// Test Close - should not return error
	err = adapter.Close()
	if err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}

func TestFileSQLAdapter_LoadFilesEmpty(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	// Test LoadFiles with empty file list
	err = adapter.LoadFiles(ctx)
	if err != nil {
		t.Errorf("LoadFiles with empty list should not return error, got: %v", err)
	}
}

func TestFileSQLAdapter_LoadFilesNilDB(t *testing.T) {
	t.Parallel()

	adapter := newTestAdapter(nil)
	ctx := context.Background()

	// Test LoadFiles with nil database
	err := adapter.LoadFiles(ctx, "test.csv")
	if err == nil {
		t.Fatal("Expected LoadFiles to fail with nil database")
	}

	if !strings.Contains(err.Error(), "shared database is not initialized") {
		t.Errorf("Expected 'shared database is not initialized' error, got: %v", err)
	}
}

func TestFileSQLAdapter_LoadFileNonexistent(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	// Test LoadFile with nonexistent file
	err = adapter.LoadFile(ctx, "/nonexistent/path/file.csv")
	if err == nil {
		t.Fatal("Expected LoadFile to fail with nonexistent file")
	}
}

func TestFileSQLError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *FileSQLError
		expected string
	}{
		{
			name:     "query error",
			err:      &FileSQLError{Op: "query", Err: "syntax error"},
			expected: "filesql query: syntax error",
		},
		{
			name:     "connection error",
			err:      &FileSQLError{Op: "connect", Err: "database locked"},
			expected: "filesql connect: database locked",
		},
		{
			name:     "empty operation",
			err:      &FileSQLError{Op: "", Err: "unknown error"},
			expected: "filesql : unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := tt.err.Error()
			if actual != tt.expected {
				t.Errorf("FileSQLError.Error() = %q, expected %q", actual, tt.expected)
			}
		})
	}
}

func TestSanitizeForSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "Sheet1",
			expected: "Sheet1",
		},
		{
			name:     "name with space",
			input:    "A test",
			expected: "A_test",
		},
		{
			name:     "name with multiple spaces",
			input:    "My Test Sheet",
			expected: "My_Test_Sheet",
		},
		{
			name:     "name with accented character e",
			input:    "Café",
			expected: "Café",
		},
		{
			name:     "name with accented character n",
			input:    "Español",
			expected: "Español",
		},
		{
			name:     "name with hyphen",
			input:    "Sheet-1",
			expected: "Sheet_1",
		},
		{
			name:     "name with dot",
			input:    "Sheet.1",
			expected: "Sheet_1",
		},
		{
			name:     "name with special characters",
			input:    "Data@2024!",
			expected: "Data2024",
		},
		{
			name:     "name with underscore preserved",
			input:    "test_sheet",
			expected: "test_sheet",
		},
		{
			name:     "empty name",
			input:    "",
			expected: "sheet",
		},
		{
			name:     "name with only spaces",
			input:    "   ",
			expected: "___",
		},
		{
			// These three used to expect the non-Latin characters to be dropped,
			// which is not what filesql does: it names the table after the file.
			// sqly works out table names in advance to detect collisions, so a
			// rule that disagreed reported two Japanese-named files as colliding
			// on the fallback name "sheet" while filesql was loading both.
			name:     "unicode characters are kept, as filesql keeps them",
			input:    "日本語シート",
			expected: "日本語シート",
		},
		{
			name:     "mixed alphanumeric and special",
			input:    "Test (2024)",
			expected: "Test_2024",
		},
		{
			name:     "numeric prefix gets sheet_ prefix",
			input:    "2023-data",
			expected: "sheet_2023_data",
		},
		{
			name:     "numeric only",
			input:    "123",
			expected: "sheet_123",
		},
		{
			name:     "numeric prefix with underscore",
			input:    "1_test",
			expected: "sheet_1_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := SanitizeForSQL(tt.input)
			if actual != tt.expected {
				t.Errorf("SanitizeForSQL(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

// Regression tests

func TestGetTableNameFromFilePath_AdditionalCompressions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{"snappy compression", "data.csv.snappy", "data"},
		{"s2 compression", "data.tsv.s2", "data"},
		{"lz4 compression", "data.ltsv.lz4", "data"},
		{"z compression", "data.json.z", "data"},
		{"json file", "data.json", "data"},
		{"jsonl file", "data.jsonl", "data"},
		{"parquet file", "data.parquet", "data"},
		{"compressed parquet", "data.parquet.gz", "data"},
		{"compressed json", "data.json.zst", "data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := GetTableNameFromFilePath(tt.filePath)
			if actual != tt.expected {
				t.Errorf("GetTableNameFromFilePath(%s) = %s, expected %s", tt.filePath, actual, tt.expected)
			}
		})
	}
}

func TestFileSQLAdapter_NumericPrefixFilename(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "2023-data.csv")
	csvContent := "id,name,value\n1,alpha,100\n2,beta,200\n"

	if err := os.WriteFile(csvFile, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	if err := adapter.LoadFile(ctx, csvFile); err != nil {
		t.Fatalf("LoadFile failed for numeric-prefix filename: %v", err)
	}

	// Verify we can query the table (table name starts with digit)
	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected at least one table after import")
	}

	// Query using QuoteIdentifier to handle numeric prefix safely
	query := "SELECT * FROM " + QuoteIdentifier(tables[0].Name())
	result, err := adapter.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query with numeric-prefix table name failed: %v", err)
	}

	if len(result.Records()) != 2 {
		t.Errorf("Expected 2 records, got %d", len(result.Records()))
	}
}

// TestGetTableNameFromFilePath_MatchesFilesqlNaming verifies that sqly's table name
// derivation matches what filesql actually creates in the database. This is a regression
// test for the naming mismatch bug where sheet-table naming failed on numeric filenames.
func TestGetTableNameFromFilePath_MatchesFilesqlNaming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
	}{
		{"simple name", "data.csv"},
		{"hyphenated name", "my-data.csv"},
		{"numeric prefix", "2023-data.csv"},
		{"dotted name", "my.data.csv"},
		{"with spaces", "my data.csv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			csvFile := filepath.Join(tempDir, tt.filename)
			if err := os.WriteFile(csvFile, []byte("a,b\n1,2\n"), 0o600); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}

			sharedDB, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("Failed to create database: %v", err)
			}
			defer func() { _ = sharedDB.Close() }()

			adapter := newTestAdapter(sharedDB)
			if err := adapter.LoadFile(context.Background(), csvFile); err != nil {
				t.Fatalf("LoadFile failed: %v", err)
			}

			tables, err := adapter.GetTableNames(context.Background())
			if err != nil {
				t.Fatalf("GetTableNames failed: %v", err)
			}

			if len(tables) == 0 {
				t.Fatal("No tables created")
			}

			actualTableName := tables[0].Name()
			expectedPrefix := GetTableNameFromFilePath(csvFile)

			if actualTableName != expectedPrefix {
				t.Errorf("Naming mismatch: filesql created %q but GetTableNameFromFilePath returned %q",
					actualTableName, expectedPrefix)
			}
		})
	}
}

func TestFileSQLAdapter_JSONFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "test.json")
	jsonContent := `[{"name":"Alice","age":30},{"name":"Bob","age":25}]`

	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0o600); err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	if err := adapter.LoadFile(ctx, jsonFile); err != nil {
		t.Fatalf("LoadFile failed for JSON file: %v", err)
	}

	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected at least one table after JSON import")
	}

	// JSON data is stored in a 'data' column
	query := "SELECT * FROM " + QuoteIdentifier(tables[0].Name())
	result, err := adapter.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query JSON table failed: %v", err)
	}

	if len(result.Records()) != 2 {
		t.Errorf("Expected 2 records from JSON array, got %d", len(result.Records()))
	}
}

func TestFileSQLAdapter_JSONLFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	jsonlFile := filepath.Join(tempDir, "test.jsonl")
	jsonlContent := "{\"name\":\"Alice\",\"age\":30}\n{\"name\":\"Bob\",\"age\":25}\n{\"name\":\"Charlie\",\"age\":35}\n"

	if err := os.WriteFile(jsonlFile, []byte(jsonlContent), 0o600); err != nil {
		t.Fatalf("Failed to create test JSONL file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	if err := adapter.LoadFile(ctx, jsonlFile); err != nil {
		t.Fatalf("LoadFile failed for JSONL file: %v", err)
	}

	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected at least one table after JSONL import")
	}

	query := "SELECT * FROM " + QuoteIdentifier(tables[0].Name())
	result, err := adapter.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query JSONL table failed: %v", err)
	}

	if len(result.Records()) != 3 {
		t.Errorf("Expected 3 records from JSONL file, got %d", len(result.Records()))
	}
}

func TestFileSQLAdapter_ExcelWithoutSheetName(t *testing.T) {
	t.Parallel()

	// Use the testdata sample.xlsx file
	xlsxFile := filepath.Join("..", "..", "testdata", "sample.xlsx")

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	if err := adapter.LoadFile(ctx, xlsxFile); err != nil {
		t.Fatalf("LoadFile failed for Excel file: %v", err)
	}

	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected at least one table after Excel import without sheet name")
	}

	// Should be able to query the first table
	query := "SELECT * FROM " + QuoteIdentifier(tables[0].Name())
	result, err := adapter.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query Excel table failed: %v", err)
	}

	if len(result.Records()) == 0 {
		t.Error("Expected records from Excel import")
	}
}

func TestFileSQLAdapter_ReservedWordTableName(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	// "select" is a SQL reserved word
	csvFile := filepath.Join(tempDir, "select.csv")
	csvContent := "id,name\n1,test\n"

	if err := os.WriteFile(csvFile, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	if err := adapter.LoadFile(ctx, csvFile); err != nil {
		t.Fatalf("LoadFile failed for reserved-word filename: %v", err)
	}

	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected at least one table")
	}

	// Query using QuoteIdentifier to handle reserved word safely
	query := "SELECT * FROM " + QuoteIdentifier(tables[0].Name())
	result, err := adapter.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query with reserved-word table name failed: %v", err)
	}

	if len(result.Records()) != 1 {
		t.Errorf("Expected 1 record, got %d", len(result.Records()))
	}
}

func TestFileSQLAdapter_ACHFile(t *testing.T) {
	t.Parallel()

	achFile := filepath.Join("..", "..", "testdata", "ppd-debit.ach")
	if _, err := os.Stat(achFile); os.IsNotExist(err) {
		t.Skip("ACH test data not available")
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	if err := adapter.LoadFile(ctx, achFile); err != nil {
		t.Fatalf("LoadFile failed for ACH file: %v", err)
	}

	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected ACH tables to be created")
	}

	// ACH files create multiple tables (file_header, batches, entries, etc.)
	tableNames := make(map[string]bool)
	for _, tbl := range tables {
		tableNames[tbl.Name()] = true
	}

	// Verify at least the entries table exists (main transaction data)
	baseName := GetTableNameFromFilePath(achFile)
	entriesTable := baseName + "_entries"
	if !tableNames[entriesTable] {
		t.Errorf("Expected entries table %q, got tables: %v", entriesTable, tableNames)
	}

	// Verify we can query the entries table
	query := "SELECT * FROM " + QuoteIdentifier(entriesTable)
	result, err := adapter.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query ACH entries failed: %v", err)
	}

	if len(result.Records()) == 0 {
		t.Error("Expected at least one entry record in ACH file")
	}
}

func TestFileSQLAdapter_FedWireFile(t *testing.T) {
	t.Parallel()

	fedFile := filepath.Join("..", "..", "testdata", "customer-transfer.fed")
	if _, err := os.Stat(fedFile); os.IsNotExist(err) {
		t.Skip("FED test data not available")
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	ctx := context.Background()

	if err := adapter.LoadFile(ctx, fedFile); err != nil {
		t.Fatalf("LoadFile failed for Fedwire file: %v", err)
	}

	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected Fedwire tables to be created")
	}

	// Fedwire creates a _message table
	baseName := GetTableNameFromFilePath(fedFile)
	messageTable := baseName + "_message"
	tableNames := make(map[string]bool)
	for _, tbl := range tables {
		tableNames[tbl.Name()] = true
	}

	if !tableNames[messageTable] {
		t.Errorf("Expected message table %q, got tables: %v", messageTable, tableNames)
	}

	// Verify we can query the message table
	query := "SELECT * FROM " + QuoteIdentifier(messageTable)
	result, err := adapter.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query Fedwire message failed: %v", err)
	}

	if len(result.Records()) == 0 {
		t.Error("Expected at least one message record in Fedwire file")
	}
}

// TestLoadFile_ACHWriteBackWorksAfterImport verifies that an ACH file imported
// through LoadFile can still be written back by DumpACHFile.
func TestLoadFile_ACHWriteBackWorksAfterImport(t *testing.T) {
	t.Parallel()

	achFile := filepath.Join("..", "..", "testdata", "ppd-debit.ach")
	if _, err := os.Stat(achFile); os.IsNotExist(err) {
		t.Skip("ACH test data not available")
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	if err := adapter.LoadFile(context.Background(), achFile); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	out := filepath.Join(t.TempDir(), "out.ach")
	if err := adapter.DumpACHFile(context.Background(), "ppd_debit", out); err != nil {
		t.Fatalf("DumpACHFile after import failed: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("Stat(%s): %v", out, err)
	}
	if info.Size() == 0 {
		t.Error("write-back produced an empty ACH file")
	}
}

// TestLoadFile_WireWriteBackWorksAfterImport is the Fedwire half: an imported
// .fed file must still be reconstructible by DumpFedWireFile.
func TestLoadFile_WireWriteBackWorksAfterImport(t *testing.T) {
	t.Parallel()

	fedFile := filepath.Join("..", "..", "testdata", "customer-transfer.fed")
	if _, err := os.Stat(fedFile); os.IsNotExist(err) {
		t.Skip("FED test data not available")
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = sharedDB.Close() }()

	adapter := newTestAdapter(sharedDB)
	if err := adapter.LoadFile(context.Background(), fedFile); err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	out := filepath.Join(t.TempDir(), "out.fed")
	if err := adapter.DumpFedWireFile(context.Background(), "customer_transfer", out); err != nil {
		t.Fatalf("DumpFedWireFile after import failed: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("Stat(%s): %v", out, err)
	}
	if info.Size() == 0 {
		t.Error("write-back produced an empty Fedwire file")
	}
}

func TestIsSupportedFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"csv", "data.csv", true},
		{"tsv", "data.tsv", true},
		{"ltsv", "data.ltsv", true},
		{"json", "data.json", true},
		{"jsonl", "data.jsonl", true},
		{"parquet", "data.parquet", true},
		{"xlsx", "data.xlsx", true},
		{"ach", "payment.ach", true},
		{"fed", "payment.fed", true},
		{"csv.gz", "data.csv.gz", true},
		{"tsv.bz2", "data.tsv.bz2", true},
		{"xlsx.xz", "data.xlsx.xz", true},
		{"csv.zst", "data.csv.zst", true},
		{"csv.z", "data.csv.z", true},
		{"csv.snappy", "data.csv.snappy", true},
		{"csv.s2", "data.csv.s2", true},
		{"csv.lz4", "data.csv.lz4", true},
		{"uppercase ACH", "PAYMENT.ACH", true},
		{"txt unsupported", "data.txt", false},
		{"no extension", "data", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsSupportedFile(tt.path); got != tt.expected {
				t.Errorf("IsSupportedFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsExcelFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"xlsx", "data.xlsx", true},
		{"xlsx.gz", "data.xlsx.gz", true},
		{"xlsx.bz2", "data.xlsx.bz2", true},
		{"uppercase", "DATA.XLSX", true},
		{"csv", "data.csv", false},
		{"ach", "payment.ach", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsExcelFile(tt.path); got != tt.expected {
				t.Errorf("IsExcelFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

// benchCSVPath returns the shared 100k-row benchmark CSV, relative to this
// package directory.
func benchCSVPath() string {
	return filepath.Join("..", "..", "testdata", "benchmark", "customers100000.csv")
}

// BenchmarkFilesqlOpenOnly measures filesql's own load cost: parse the file and
// build its temporary in-memory database, with no copy into a shared DB.
func BenchmarkFilesqlOpenOnly(b *testing.B) {
	csv := benchCSVPath()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		db, err := libfilesql.Open(context.Background(), csv)
		if err != nil {
			b.Fatal(err)
		}
		_ = db.Close()
	}
}

// BenchmarkAdapterLoadFiles measures the full sqly import. Since the refactor
// that streams files directly into the shared DB (filesql.LoadInto), this should
// track BenchmarkFilesqlOpenOnly closely instead of roughly doubling it.
func BenchmarkAdapterLoadFiles(b *testing.B) {
	csv := benchCSVPath()
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	// Match production: ":memory:" is private per connection, so pin the pool.
	sharedDB.SetMaxOpenConns(1)
	defer func() { _ = sharedDB.Close() }()
	adapter := newTestAdapter(sharedDB)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := adapter.LoadFiles(context.Background(), csv); err != nil {
			b.Fatal(err)
		}
	}
}

// TestFileSQLAdapter_EmptyJSONLikeInputs verifies that an empty JSON array and an
// empty JSONL file import as zero-row tables (with filesql's "data" column)
// instead of failing as an empty data source.
func TestFileSQLAdapter_EmptyJSONLikeInputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		file    string
		content string
		table   string
	}{
		{"empty JSON array", "empty.json", "[]", "empty"},
		{"whitespace-only JSON", "blank.json", "   \n", "blank"},
		{"empty JSONL", "empty.jsonl", "", "empty"},
		{"blank-line-only JSONL", "blank.jsonl", "\n\n", "blank"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, tc.file)
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}

			sharedDB, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = sharedDB.Close() }()

			adapter := newTestAdapter(sharedDB)
			ctx := context.Background()
			if err := adapter.LoadFile(ctx, path); err != nil {
				t.Fatalf("LoadFile of empty JSON input failed: %v", err)
			}

			var count int
			if err := sharedDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+QuoteIdentifier(tc.table)).Scan(&count); err != nil {
				t.Fatalf("query zero-row table: %v", err)
			}
			if count != 0 {
				t.Errorf("row_count = %d, want 0", count)
			}

			var col string
			if err := sharedDB.QueryRowContext(ctx, "SELECT name FROM pragma_table_info("+QuoteIdentifier(tc.table)+")").Scan(&col); err != nil {
				t.Fatalf("read column: %v", err)
			}
			if col != "data" {
				t.Errorf("column = %q, want \"data\"", col)
			}
		})
	}
}

// gzipFile writes data gzip-compressed to path, for building compressed test
// inputs.
func gzipFile(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // test temp path
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	gw := gzip.NewWriter(f)
	if _, err := gw.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
}

// TestFileSQLAdapter_EmptyCompressedJSONLike verifies that an empty compressed
// JSON array (.json.gz) and an empty compressed JSONL file (.jsonl.gz) import as a
// zero-row table with the single "data" column, matching the uncompressed empty
// inputs.
func TestFileSQLAdapter_EmptyCompressedJSONLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		raw  []byte
	}{
		{"empty JSON array gz", "empty.json.gz", []byte("[]")},
		{"whitespace-only JSON gz", "blank.json.gz", []byte("   \n")},
		{"empty JSONL gz", "empty.jsonl.gz", []byte("")},
		{"blank-only JSONL gz", "blank.jsonl.gz", []byte("\n\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			gzipFile(t, path, tt.raw)

			sharedDB, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("open db: %v", err)
			}
			defer func() { _ = sharedDB.Close() }()
			adapter := newTestAdapter(sharedDB)

			ctx := context.Background()
			if err := adapter.LoadFile(ctx, path); err != nil {
				t.Fatalf("LoadFile(%s) error = %v, want nil (zero-row import)", tt.file, err)
			}

			name := GetTableNameFromFilePath(path)
			table, err := adapter.Query(ctx, "SELECT COUNT(*) AS c FROM "+QuoteIdentifier(name))
			if err != nil {
				t.Fatalf("count query error: %v", err)
			}
			if len(table.Records()) != 1 || table.Records()[0][0] != "0" {
				t.Errorf("expected a zero-row table, got records %v", table.Records())
			}
			// The zero-row table uses filesql's single "data" column contract.
			hdr, err := adapter.GetTableHeader(ctx, name)
			if err != nil {
				t.Fatalf("header error: %v", err)
			}
			if h := hdr.Header(); len(h) != 1 || h[0] != "data" {
				t.Errorf("expected single 'data' column, got header %v", h)
			}
		})
	}
}

// TestFileSQLAdapter_LTSVDuplicateLabelsRejected verifies that an LTSV input whose
// row repeats a label is rejected rather than silently keeping only the last
// value.
func TestFileSQLAdapter_LTSVDuplicateLabelsRejected(t *testing.T) {
	t.Parallel()

	t.Run("plain ltsv with duplicate label fails", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "dup.ltsv")
		if err := os.WriteFile(path, []byte("x:1\tx:2\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		sharedDB, _ := sql.Open("sqlite", ":memory:")
		defer func() { _ = sharedDB.Close() }()
		adapter := newTestAdapter(sharedDB)
		if err := adapter.LoadFile(context.Background(), path); err == nil {
			t.Error("LoadFile of duplicate-label LTSV returned nil error, want a duplicate-label rejection")
		}
	})

	t.Run("compressed ltsv with duplicate label fails", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "dup.ltsv.gz")
		gzipFile(t, path, []byte("a:1\tb:2\ta:3\n"))
		sharedDB, _ := sql.Open("sqlite", ":memory:")
		defer func() { _ = sharedDB.Close() }()
		adapter := newTestAdapter(sharedDB)
		if err := adapter.LoadFile(context.Background(), path); err == nil {
			t.Error("LoadFile of duplicate-label compressed LTSV returned nil error, want rejection")
		}
	})

	t.Run("unique labels still import", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "ok.ltsv")
		if err := os.WriteFile(path, []byte("x:1\ty:2\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		sharedDB, _ := sql.Open("sqlite", ":memory:")
		defer func() { _ = sharedDB.Close() }()
		adapter := newTestAdapter(sharedDB)
		if err := adapter.LoadFile(context.Background(), path); err != nil {
			t.Errorf("LoadFile of unique-label LTSV error = %v, want nil", err)
		}
	})
}
