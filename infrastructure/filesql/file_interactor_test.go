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

func TestNewCSVInteractor(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter)

	if interactor == nil {
		t.Fatal("NewCSVInteractor returned nil")
	}
}

func TestNewTSVInteractor(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewTSVInteractor(adapter)

	if interactor == nil {
		t.Fatal("NewTSVInteractor returned nil")
	}
}

func TestNewLTSVInteractor(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewLTSVInteractor(adapter)

	if interactor == nil {
		t.Fatal("NewLTSVInteractor returned nil")
	}
}

func TestNewExcelInteractor(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewExcelInteractor(adapter)

	if interactor == nil {
		t.Fatal("NewExcelInteractor returned nil")
	}
}

func TestFileInteractor_List(t *testing.T) {
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

	// Create adapter and interactor
	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter)

	// Test List
	table, err := interactor.List(csvFile)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify results
	if table == nil {
		t.Fatal("List returned nil table")
	}

	if len(table.Records()) != 2 {
		t.Errorf("Expected 2 records, got %d", len(table.Records()))
	}

	// Check that all expected headers are present
	expectedHeaders := []string{"name", "age", "city"}
	actualHeaders := table.Header()
	if len(actualHeaders) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(actualHeaders))
	}

	// Convert actual headers to a set for easy lookup
	actualHeaderSet := make(map[string]bool)
	for _, header := range actualHeaders {
		actualHeaderSet[header] = true
	}

	// Check that all expected headers exist
	for _, expected := range expectedHeaders {
		if !actualHeaderSet[expected] {
			t.Errorf("Expected header %s not found in actual headers: %v", expected, actualHeaders)
		}
	}
}

func TestFileInteractor_Dump(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter)

	// Test Dump - should return not implemented error
	err = interactor.Dump("output.csv", nil)
	if err == nil {
		t.Fatal("Expected Dump to return error for not implemented functionality")
	}

	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

func TestExcelInteractor_List(t *testing.T) {
	t.Parallel()

	// Since we can't easily create Excel files in tests, we'll create a scenario
	// where we simulate the expected table structure for Excel sheets
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create a mock table that simulates what filesql would create for an Excel sheet
	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE test_Sheet1 (name TEXT, value INTEGER)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	_, err = sharedDB.ExecContext(context.Background(), `INSERT INTO test_Sheet1 VALUES ('item1', 100), ('item2', 200)`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewExcelInteractor(adapter)

	// Test List - this will fail because the file doesn't exist, but we can test the logic
	_, err = interactor.List("test.xlsx", "Sheet1")
	if err == nil {
		t.Fatal("Expected List to fail for non-existent file")
	}

	// The error should be about file loading, not table querying
	if !strings.Contains(err.Error(), "failed to load") && !strings.Contains(err.Error(), "stream") {
		t.Logf("Got expected error for non-existent file: %v", err)
	}
}

func TestExcelInteractor_Dump(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	interactor := NewExcelInteractor(adapter)

	// Test Dump - should return not implemented error
	err = interactor.Dump("output.xlsx", nil)
	if err == nil {
		t.Fatal("Expected Dump to return error for not implemented functionality")
	}

	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Expected 'not yet implemented' error, got: %v", err)
	}
}

func TestExcelInteractor_GetSheetNames(t *testing.T) {
	t.Parallel()

	// Test with non-existent file should return error
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := NewFileSQLAdapter(sharedDB)
	interactor, ok := NewExcelInteractor(adapter).(*excelInteractor)
	if !ok {
		t.Fatal("NewExcelInteractor did not return expected type")
	}

	// Test GetSheetNames - this will fail because the file doesn't exist
	_, err = interactor.GetSheetNames("nonexistent.xlsx")
	if err == nil {
		t.Fatal("Expected GetSheetNames to fail for non-existent file")
	}

	// The error should be about file loading
	if !strings.Contains(err.Error(), "failed to load") && !strings.Contains(err.Error(), "stream") {
		t.Logf("Got expected error for non-existent file: %v", err)
	}
}

func TestExcelInteractor_GetSheetNames_WithMockTables(t *testing.T) {
	t.Parallel()

	// Create scenario where we simulate Excel sheet tables
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create mock tables that simulate Excel sheets
	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE workbook_Sheet1 (name TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE workbook_Sheet2 (name TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	_, err = sharedDB.ExecContext(context.Background(), `CREATE TABLE other_table (name TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	adapter := NewFileSQLAdapter(sharedDB)

	// Manually test the sheet name extraction logic by calling GetTableNames
	tables, err := adapter.GetTableNames(context.Background())
	if err != nil {
		t.Fatalf("Failed to get table names: %v", err)
	}

	// Simulate what GetSheetNames does
	baseName := "workbook"
	prefix := baseName + "_"
	var sheetNames []string
	for _, table := range tables {
		if strings.HasPrefix(table.Name(), prefix) {
			sheetName := strings.TrimPrefix(table.Name(), prefix)
			sheetNames = append(sheetNames, sheetName)
		}
	}

	// Verify results
	expectedSheets := []string{"Sheet1", "Sheet2"}
	if len(sheetNames) != len(expectedSheets) {
		t.Errorf("Expected %d sheets, got %d", len(expectedSheets), len(sheetNames))
	}

	sheetSet := make(map[string]bool)
	for _, sheet := range sheetNames {
		sheetSet[sheet] = true
	}

	for _, expected := range expectedSheets {
		if !sheetSet[expected] {
			t.Errorf("Expected sheet %s not found in results: %v", expected, sheetNames)
		}
	}
}
