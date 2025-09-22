package interactor

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/mock"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"go.uber.org/mock/gomock"
	_ "modernc.org/sqlite"
)

func TestCSVInteractor_List(t *testing.T) {
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

	// Create mocks and adapter
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	csvRepo := mock.NewMockCSVRepository(ctrl)
	fileRepo := mock.NewMockFileRepository(ctrl)
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

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

	// Check that all expected headers are present (order may vary by platform)
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

func TestCSVInteractor_Dump(t *testing.T) {
	t.Parallel()

	// Create test table
	headers := []string{"name", "age", "city"}
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
	}
	table := model.NewTable("test", model.NewHeader(headers), records)

	// Create temporary output file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.csv")

	// Create shared database (needed for constructor but not used in Dump)
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories for dump testing
	csvRepo := persistence.NewCSVRepository()
	fileRepo := persistence.NewFileRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

	// Test Dump
	err = interactor.Dump(outputFile, table)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(filepath.Clean(outputFile))
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expectedContent := `name,age,city
John,25,New York
Jane,30,Los Angeles
`

	if string(content) != expectedContent {
		t.Errorf("Output content mismatch.\nExpected:\n%s\nActual:\n%s", expectedContent, string(content))
	}
}

func TestCSVInteractor_ListWithNilAdapter(t *testing.T) {
	t.Parallel()

	// Create interactor with nil adapter
	interactor := &csvInteractor{
		baseFileInteractor: &baseFileInteractor{filesqlAdapter: nil},
	}

	// Test List with nil adapter
	_, err := interactor.List("test.csv")
	if err == nil {
		t.Fatal("Expected error with nil adapter, got nil")
	}

	if !errors.Is(err, ErrFilesqlAdapterNotInitialized) {
		t.Errorf("Expected ErrFilesqlAdapterNotInitialized, got: %v", err)
	}
}

func TestNewCSVInteractor(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create mocks and adapter
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	csvRepo := mock.NewMockCSVRepository(ctrl)
	fileRepo := mock.NewMockFileRepository(ctrl)
	adapter := filesql.NewFileSQLAdapter(sharedDB)

	// Test NewCSVInteractor
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

	if interactor == nil {
		t.Fatal("NewCSVInteractor returned nil")
	}

	// Verify it implements the interface
	csvInteractor, ok := interactor.(*csvInteractor)
	if !ok {
		t.Fatal("NewCSVInteractor did not return *csvInteractor")
	}

	if csvInteractor.filesqlAdapter != adapter {
		t.Error("NewCSVInteractor did not set adapter correctly")
	}
}

func TestCSVInteractor_DumpInvalidPath(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories for dump testing
	csvRepo := persistence.NewCSVRepository()
	fileRepo := persistence.NewFileRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

	// Create test table
	header := model.NewHeader([]string{"id"})
	table := model.NewTable("test", header, nil)

	// Test Dump with invalid path
	err = interactor.Dump("/nonexistent/directory/file.csv", table)
	if err == nil {
		t.Fatal("Expected error when dumping to invalid path")
	}

	// Check for path-related errors (different messages on different platforms)
	if !strings.Contains(err.Error(), "no such file or directory") &&
		!strings.Contains(err.Error(), "The system cannot find the path specified") {
		t.Errorf("Expected path-related error, got: %v", err)
	}
}

func TestCSVInteractor_DumpEmptyTable(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories for dump testing
	csvRepo := persistence.NewCSVRepository()
	fileRepo := persistence.NewFileRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

	// Create empty table
	header := model.NewHeader([]string{"id", "name"})
	table := model.NewTable("empty_table", header, nil)

	// Create temporary file
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "empty.csv")

	// Test Dump with empty table
	err = interactor.Dump(csvFile, table)
	if err != nil {
		t.Fatalf("Dump of empty table failed: %v", err)
	}

	// Verify file was created with header only
	content, err := os.ReadFile(filepath.Clean(csvFile))
	if err != nil {
		t.Fatalf("Failed to read CSV file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "id,name") {
		t.Errorf("Expected header in empty table content, got: %s", contentStr)
	}

	// Should only have header line
	lines := strings.Split(strings.TrimSpace(contentStr), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected only header line, got %d lines", len(lines))
	}
}

func TestCSVInteractor_ListNonexistentFile(t *testing.T) {
	t.Parallel()

	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create mocks and adapter for List testing (not Dump)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	csvRepo := mock.NewMockCSVRepository(ctrl)
	fileRepo := mock.NewMockFileRepository(ctrl)
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

	// Test List with nonexistent file
	_, err = interactor.List("/nonexistent/path/file.csv")
	if err == nil {
		t.Fatal("Expected error when file doesn't exist")
	}

	if !strings.Contains(err.Error(), "failed to load CSV file") {
		t.Errorf("Expected 'failed to load CSV file' error, got: %v", err)
	}
}
