package filesql

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestFileSQLAdapter_LoadFile(t *testing.T) {
	t.Parallel()

	// Create temporary test CSV file
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test.csv")

	csvContent := `name,age,city
John,25,New York
Jane,30,Los Angeles`

	if err := os.WriteFile(csvFile, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter
	adapter := NewFileSQLAdapter(sharedDB)

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

	if err := os.WriteFile(csvFile, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter
	adapter := NewFileSQLAdapter(sharedDB)

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

	if err := os.WriteFile(csvFile, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter
	adapter := NewFileSQLAdapter(sharedDB)

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

func TestFileSQLAdapter_Query(t *testing.T) {
	t.Parallel()

	// Create shared database with test data
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create test table
	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE test_table (id INTEGER, name TEXT, age INTEGER)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data
	_, err = sharedDB.ExecContext(context.Background(), `INSERT INTO test_table VALUES (1, 'John', 25), (2, 'Jane', 30)`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Create adapter
	adapter := NewFileSQLAdapter(sharedDB)

	// Test Query
	ctx := context.Background()
	table, err := adapter.Query(ctx, "SELECT * FROM test_table ORDER BY id")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Verify results
	if len(table.Records()) != 2 {
		t.Errorf("Expected 2 records, got %d", len(table.Records()))
	}

	expectedHeaders := []string{"id", "name", "age"}
	actualHeaders := table.Header()
	if len(actualHeaders) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(actualHeaders))
	}

	// Verify first record
	if len(table.Records()) > 0 {
		firstRecord := table.Records()[0]
		if len(firstRecord) >= 3 {
			if firstRecord[0] != "1" || firstRecord[1] != "John" || firstRecord[2] != "25" {
				t.Errorf("First record data mismatch: got %v", firstRecord)
			}
		}
	}
}

func TestFileSQLAdapter_GetTableNames(t *testing.T) {
	t.Parallel()

	// Create shared database with multiple test tables
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create test tables
	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE table1 (id INTEGER)`)
	if err != nil {
		t.Fatalf("Failed to create table1: %v", err)
	}

	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE table2 (name TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table2: %v", err)
	}

	// Create adapter
	adapter := NewFileSQLAdapter(sharedDB)

	// Test GetTableNames
	ctx := context.Background()
	tables, err := adapter.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("GetTableNames failed: %v", err)
	}

	// Verify results
	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}

	// Verify table names (order may vary)
	tableNames := make(map[string]bool)
	for _, table := range tables {
		tableNames[table.Name()] = true
	}

	if !tableNames["table1"] || !tableNames["table2"] {
		t.Errorf("Expected tables table1 and table2, got: %v", tableNames)
	}
}

func TestFileSQLAdapter_Exec(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter
	adapter := NewFileSQLAdapter(sharedDB)

	// Test Exec - CREATE TABLE
	ctx := context.Background()
	rowsAffected, err := adapter.Exec(ctx, "CREATE TABLE test_exec (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatalf("Exec CREATE TABLE failed: %v", err)
	}

	// CREATE TABLE typically returns 0 rows affected
	if rowsAffected != 0 {
		t.Logf("CREATE TABLE returned %d rows affected (expected 0, but this may vary)", rowsAffected)
	}

	// Test Exec - INSERT
	rowsAffected, err = adapter.Exec(ctx, "INSERT INTO test_exec VALUES (1, 'test')")
	if err != nil {
		t.Fatalf("Exec INSERT failed: %v", err)
	}

	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected by INSERT, got %d", rowsAffected)
	}

	// Test Exec - UPDATE
	rowsAffected, err = adapter.Exec(ctx, "UPDATE test_exec SET name = 'updated' WHERE id = 1")
	if err != nil {
		t.Fatalf("Exec UPDATE failed: %v", err)
	}

	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected by UPDATE, got %d", rowsAffected)
	}
}

func TestNewFileSQLAdapter(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Test NewFileSQLAdapter
	adapter := NewFileSQLAdapter(sharedDB)

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

	if err := os.WriteFile(csvFile, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter
	adapter := NewFileSQLAdapter(sharedDB)

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
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)

	// Test Close - should not return error
	err = adapter.Close()
	if err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}

func TestFileSQLAdapter_ExecNilDB(t *testing.T) {
	t.Parallel()

	adapter := NewFileSQLAdapter(nil)
	ctx := context.Background()

	// Test Exec with nil database
	_, err := adapter.Exec(ctx, "SELECT 1")
	if err == nil {
		t.Fatal("Expected Exec to fail with nil database")
	}

	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Expected 'database not initialized' error, got: %v", err)
	}
}

func TestFileSQLAdapter_QueryNilDB(t *testing.T) {
	t.Parallel()

	adapter := NewFileSQLAdapter(nil)
	ctx := context.Background()

	// Test Query with nil database
	_, err := adapter.Query(ctx, "SELECT 1")
	if err == nil {
		t.Fatal("Expected Query to fail with nil database")
	}

	if !strings.Contains(err.Error(), "shared database not initialized") {
		t.Errorf("Expected 'shared database not initialized' error, got: %v", err)
	}
}

func TestFileSQLAdapter_GetTableNamesNilDB(t *testing.T) {
	t.Parallel()

	adapter := NewFileSQLAdapter(nil)
	ctx := context.Background()

	// Test GetTableNames with nil database
	_, err := adapter.GetTableNames(ctx)
	if err == nil {
		t.Fatal("Expected GetTableNames to fail with nil database")
	}

	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Expected 'database not initialized' error, got: %v", err)
	}
}

func TestFileSQLAdapter_LoadFilesEmpty(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	ctx := context.Background()

	// Test LoadFiles with empty file list
	err = adapter.LoadFiles(ctx)
	if err != nil {
		t.Errorf("LoadFiles with empty list should not return error, got: %v", err)
	}
}

func TestFileSQLAdapter_LoadFilesNilDB(t *testing.T) {
	t.Parallel()

	adapter := NewFileSQLAdapter(nil)
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
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	ctx := context.Background()

	// Test LoadFile with nonexistent file
	err = adapter.LoadFile(ctx, "/nonexistent/path/file.csv")
	if err == nil {
		t.Fatal("Expected LoadFile to fail with nonexistent file")
	}
}

func TestFileSQLAdapter_GetTableHeaderNilDB(t *testing.T) {
	t.Parallel()

	adapter := NewFileSQLAdapter(nil)
	ctx := context.Background()

	// Test GetTableHeader with nil database
	_, err := adapter.GetTableHeader(ctx, "test_table")
	if err == nil {
		t.Fatal("Expected GetTableHeader to fail with nil database")
	}

	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Expected 'database not initialized' error, got: %v", err)
	}
}

func TestFileSQLAdapter_GetTableHeader(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create test table with various column types
	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE test_header (id INTEGER, name TEXT, balance REAL, active BOOLEAN)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	adapter := NewFileSQLAdapter(sharedDB)
	ctx := context.Background()

	// Test GetTableHeader
	table, err := adapter.GetTableHeader(ctx, "test_header")
	if err != nil {
		t.Fatalf("GetTableHeader failed: %v", err)
	}

	if table.Name() != "test_header" {
		t.Errorf("Expected table name 'test_header', got %s", table.Name())
	}

	expectedHeaders := []string{"id", "name", "balance", "active"}
	actualHeaders := table.Header()
	if len(actualHeaders) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(actualHeaders))
	}

	for i, expected := range expectedHeaders {
		if i < len(actualHeaders) && actualHeaders[i] != expected {
			t.Errorf("Expected header %d to be %s, got %s", i, expected, actualHeaders[i])
		}
	}

	// Records should be nil for header-only queries
	if table.Records() != nil {
		t.Errorf("Expected no records in header-only table, got %d", len(table.Records()))
	}
}

func TestFileSQLAdapter_GetTableHeaderNonexistent(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	ctx := context.Background()

	// Test GetTableHeader with nonexistent table
	table, err := adapter.GetTableHeader(ctx, "nonexistent_table")
	if err != nil {
		// Error is expected for nonexistent tables
		t.Logf("Expected error for nonexistent table: %v", err)
		return
	}

	// If no error, the table should have no columns
	if table != nil && len(table.Header()) > 0 {
		t.Errorf("Expected empty headers for nonexistent table, got: %v", table.Header())
	}
}

func TestFileSQLAdapter_QueryWithDifferentDataTypes(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create test table with various data types
	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE test_types (id INTEGER, name TEXT, balance REAL, data BLOB)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data with different types
	_, err = sharedDB.ExecContext(context.Background(), `INSERT INTO test_types VALUES (1, 'test', 123.45, X'48656C6C6F')`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Insert row with NULL values
	_, err = sharedDB.ExecContext(context.Background(), `INSERT INTO test_types VALUES (2, NULL, NULL, NULL)`)
	if err != nil {
		t.Fatalf("Failed to insert NULL test data: %v", err)
	}

	adapter := NewFileSQLAdapter(sharedDB)
	ctx := context.Background()

	// Test Query with different data types
	table, err := adapter.Query(ctx, "SELECT * FROM test_types ORDER BY id")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(table.Records()) != 2 {
		t.Errorf("Expected 2 records, got %d", len(table.Records()))
	}

	// Check first record data types conversion
	if len(table.Records()) > 0 {
		firstRecord := table.Records()[0]
		if len(firstRecord) >= 4 {
			// INTEGER -> string
			if firstRecord[0] != "1" {
				t.Errorf("Expected id '1', got %s", firstRecord[0])
			}
			// TEXT -> string
			if firstRecord[1] != "test" {
				t.Errorf("Expected name 'test', got %s", firstRecord[1])
			}
			// REAL -> string
			if firstRecord[2] != "123.45" {
				t.Errorf("Expected balance '123.45', got %s", firstRecord[2])
			}
			// BLOB -> string
			if firstRecord[3] != "Hello" {
				t.Errorf("Expected data 'Hello', got %s", firstRecord[3])
			}
		}
	}

	// Check second record with NULL values
	if len(table.Records()) > 1 {
		secondRecord := table.Records()[1]
		if len(secondRecord) >= 4 {
			// NULL values should become empty strings
			if secondRecord[1] != "" {
				t.Errorf("Expected NULL name to be empty string, got %s", secondRecord[1])
			}
			if secondRecord[2] != "" {
				t.Errorf("Expected NULL balance to be empty string, got %s", secondRecord[2])
			}
			if secondRecord[3] != "" {
				t.Errorf("Expected NULL data to be empty string, got %s", secondRecord[3])
			}
		}
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
			expected: "Caf",
		},
		{
			name:     "name with accented character n",
			input:    "Español",
			expected: "Espaol",
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
			name:     "unicode characters",
			input:    "日本語シート",
			expected: "sheet",
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

	if err := os.WriteFile(csvFile, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
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
// test for the naming mismatch bug where --sheet filtering failed on numeric filenames.
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
			if err := os.WriteFile(csvFile, []byte("a,b\n1,2\n"), 0600); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}

			sharedDB, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("Failed to create database: %v", err)
			}
			defer sharedDB.Close()

			adapter := NewFileSQLAdapter(sharedDB)
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

	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0600); err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
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

	if err := os.WriteFile(jsonlFile, []byte(jsonlContent), 0600); err != nil {
		t.Fatalf("Failed to create test JSONL file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
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
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
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

	if err := os.WriteFile(csvFile, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
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
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
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
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
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

func TestIsACHTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		table    string
		expected bool
	}{
		{"entries table", "payment_entries", true},
		{"file_header table", "payment_file_header", true},
		{"batches table", "payment_batches", true},
		{"addenda table", "payment_addenda", true},
		{"iat_batches table", "payment_iat_batches", true},
		{"iat_entries table", "payment_iat_entries", true},
		{"iat_addenda table", "payment_iat_addenda", true},
		{"regular table", "users", false},
		{"message table (wire)", "payment_message", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsACHTable(tt.table); got != tt.expected {
				t.Errorf("IsACHTable(%q) = %v, want %v", tt.table, got, tt.expected)
			}
		})
	}
}

func TestIsWireTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		table    string
		expected bool
	}{
		{"message table", "payment_message", true},
		{"entries table (ACH)", "payment_entries", false},
		{"regular table", "users", false},
		{"empty string", "", false},
		{"only suffix", "_message", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsWireTable(tt.table); got != tt.expected {
				t.Errorf("IsWireTable(%q) = %v, want %v", tt.table, got, tt.expected)
			}
		})
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
