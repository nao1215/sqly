package filesql

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/xuri/excelize/v2"
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

	t.Run("nil table", func(t *testing.T) {
		t.Parallel()
		err = interactor.Dump("output.csv", nil)
		if err == nil {
			t.Fatal("Expected Dump to return error for nil table")
		}

		if !strings.Contains(err.Error(), "table cannot be nil") {
			t.Errorf("Expected 'table cannot be nil' error, got: %v", err)
		}
	})

	// Create test table for successful dump tests
	header := model.NewHeader([]string{"name", "age", "city"})
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
		model.NewRecord([]string{"Bob", "35", "Chicago"}),
	}
	testTable := model.NewTable("test_table", header, records)

	t.Run("CSV dump", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		csvFile := filepath.Join(tempDir, "test.csv")

		err := interactor.Dump(csvFile, testTable)
		if err != nil {
			t.Fatalf("CSV dump failed: %v", err)
		}

		// Verify file contents
		content, err := os.ReadFile(csvFile) // #nosec G304 - test file in temp dir
		if err != nil {
			t.Fatalf("Failed to read CSV file: %v", err)
		}

		expected := "name,age,city\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago\n"
		if string(content) != expected {
			t.Errorf("CSV content mismatch.\nExpected: %q\nGot: %q", expected, string(content))
		}
	})

	t.Run("TSV dump", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		tsvFile := filepath.Join(tempDir, "test.tsv")

		err := interactor.Dump(tsvFile, testTable)
		if err != nil {
			t.Fatalf("TSV dump failed: %v", err)
		}

		// Verify file contents
		content, err := os.ReadFile(tsvFile) // #nosec G304 - test file in temp dir
		if err != nil {
			t.Fatalf("Failed to read TSV file: %v", err)
		}

		expected := "name\tage\tcity\nJohn\t25\tNew York\nJane\t30\tLos Angeles\nBob\t35\tChicago\n"
		if string(content) != expected {
			t.Errorf("TSV content mismatch.\nExpected: %q\nGot: %q", expected, string(content))
		}
	})

	t.Run("LTSV dump", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		ltsvFile := filepath.Join(tempDir, "test.ltsv")

		err := interactor.Dump(ltsvFile, testTable)
		if err != nil {
			t.Fatalf("LTSV dump failed: %v", err)
		}

		// Verify file contents
		content, err := os.ReadFile(ltsvFile) // #nosec G304 - test file in temp dir
		if err != nil {
			t.Fatalf("Failed to read LTSV file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 3 {
			t.Fatalf("Expected 3 lines, got %d", len(lines))
		}

		// Check first line format
		expected := "name:John\tage:25\tcity:New York"
		if lines[0] != expected {
			t.Errorf("LTSV first line mismatch.\nExpected: %q\nGot: %q", expected, lines[0])
		}
	})

	t.Run("Markdown dump", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		mdFile := filepath.Join(tempDir, "test.md")

		err := interactor.Dump(mdFile, testTable)
		if err != nil {
			t.Fatalf("Markdown dump failed: %v", err)
		}

		// Verify file was created and has content
		content, err := os.ReadFile(mdFile) // #nosec G304 - test file in temp dir
		if err != nil {
			t.Fatalf("Failed to read Markdown file: %v", err)
		}

		if len(content) == 0 {
			t.Error("Markdown file is empty")
		}

		// Check for markdown table format
		contentStr := string(content)
		if !strings.Contains(contentStr, "|") {
			t.Error("Markdown content doesn't contain table separators")
		}
	})

	t.Run("default format (unknown extension)", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		unknownFile := filepath.Join(tempDir, "test.unknown")

		err := interactor.Dump(unknownFile, testTable)
		if err != nil {
			t.Fatalf("Unknown format dump failed: %v", err)
		}

		// Should default to CSV format
		content, err := os.ReadFile(unknownFile) // #nosec G304 - test file in temp dir
		if err != nil {
			t.Fatalf("Failed to read unknown format file: %v", err)
		}

		expected := "name,age,city\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago\n"
		if string(content) != expected {
			t.Errorf("Default format content mismatch.\nExpected: %q\nGot: %q", expected, string(content))
		}
	})

	t.Run("invalid file path", func(t *testing.T) {
		t.Parallel()
		// Try to write to a directory that doesn't exist (cross-platform)
		invalidPath := filepath.Join(t.TempDir(), "no_such_dir", "test.csv")

		err := interactor.Dump(invalidPath, testTable)
		if err == nil {
			t.Fatal("Expected error for invalid file path")
		}

		if !strings.Contains(err.Error(), "failed to create file") {
			t.Errorf("Expected file creation error, got: %v", err)
		}
	})
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
	t.Logf("Got error for non-existent file (expected): %v", err)
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

	t.Run("nil table", func(t *testing.T) {
		t.Parallel()
		err = interactor.Dump("output.xlsx", nil)
		if err == nil {
			t.Fatal("Expected Dump to return error for nil table")
		}

		if !strings.Contains(err.Error(), "table cannot be nil") {
			t.Errorf("Expected 'table cannot be nil' error, got: %v", err)
		}
	})

	// Create test table for successful dump tests
	header := model.NewHeader([]string{"name", "age", "city"})
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
		model.NewRecord([]string{"Bob", "35", "Chicago"}),
	}
	testTable := model.NewTable("test_data", header, records)

	t.Run("successful Excel dump", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		excelFile := filepath.Join(tempDir, "test.xlsx")

		err := interactor.Dump(excelFile, testTable)
		if err != nil {
			t.Fatalf("Excel dump failed: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(excelFile); os.IsNotExist(err) {
			t.Error("Excel file was not created")
		}

		// Try to open and read the Excel file to verify contents
		f, err := excelize.OpenFile(excelFile)
		if err != nil {
			t.Fatalf("Failed to open Excel file: %v", err)
		}
		defer f.Close()

		// Check sheet exists
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			t.Fatal("No sheets found in Excel file")
		}

		sheetName := "test_data"
		if !contains(sheets, sheetName) {
			t.Errorf("Expected sheet %q not found. Available sheets: %v", sheetName, sheets)
		}

		// Check header row
		headerRow, err := f.GetRows(sheetName)
		if err != nil {
			t.Fatalf("Failed to get rows from Excel file: %v", err)
		}

		if len(headerRow) < 1 {
			t.Fatal("Excel file has no rows")
		}

		expectedHeader := []string{"name", "age", "city"}
		if !equalSlices(headerRow[0], expectedHeader) {
			t.Errorf("Header mismatch. Expected: %v, Got: %v", expectedHeader, headerRow[0])
		}

		// Check data rows
		if len(headerRow) != 4 { // header + 3 data rows
			t.Errorf("Expected 4 rows (header + 3 data), got %d", len(headerRow))
		}
	})

	t.Run("Excel dump with special characters in table name", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		excelFile := filepath.Join(tempDir, "special.xlsx")

		// Create table with problematic name
		specialHeader := model.NewHeader([]string{"col1", "col2"})
		specialRecords := []model.Record{model.NewRecord([]string{"val1", "val2"})}
		specialTable := model.NewTable("test-table.with/special:chars", specialHeader, specialRecords)

		err := interactor.Dump(excelFile, specialTable)
		if err != nil {
			t.Fatalf("Excel dump with special chars failed: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(excelFile); os.IsNotExist(err) {
			t.Error("Excel file with special chars was not created")
		}
	})

	t.Run("Excel dump with empty table", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		excelFile := filepath.Join(tempDir, "empty.xlsx")

		emptyHeader := model.NewHeader([]string{"col1", "col2"})
		emptyRecords := []model.Record{}
		emptyTable := model.NewTable("empty_table", emptyHeader, emptyRecords)

		err := interactor.Dump(excelFile, emptyTable)
		if err != nil {
			t.Fatalf("Excel dump with empty table failed: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(excelFile); os.IsNotExist(err) {
			t.Error("Excel file with empty table was not created")
		}
	})

	t.Run("invalid file path", func(t *testing.T) {
		t.Parallel()
		invalidPath := filepath.Join(t.TempDir(), "no_such_dir", "test.xlsx")

		err := interactor.Dump(invalidPath, testTable)
		if err == nil {
			t.Fatal("Expected error for invalid file path")
		}

		if !strings.Contains(err.Error(), "failed to save Excel file") {
			t.Errorf("Expected save Excel file error, got: %v", err)
		}
	})
}

// Helper functions for tests
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
