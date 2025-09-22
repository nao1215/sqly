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
	"github.com/nao1215/sqly/infrastructure/persistence"
	_ "modernc.org/sqlite"
)

func TestLTSVInteractor_List(t *testing.T) {
	t.Parallel()

	// Create temporary test LTSV file
	tempDir := t.TempDir()
	ltsvFile := filepath.Join(tempDir, "test.ltsv")

	ltsvContent := `name:John	age:25	city:New York
name:Jane	age:30	city:Los Angeles`

	if err := os.WriteFile(ltsvFile, []byte(ltsvContent), 0600); err != nil {
		t.Fatalf("Failed to create test LTSV file: %v", err)
	}

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter and interactor
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	// Create real repositories for dump testing
	ltsvRepo := persistence.NewLTSVRepository()
	fileRepo := persistence.NewFileRepository()
	interactor := NewLTSVInteractor(adapter, ltsvRepo, fileRepo)

	// Test List
	table, err := interactor.List(ltsvFile)
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

func TestLTSVInteractor_Dump(t *testing.T) {
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
	outputFile := filepath.Join(tempDir, "output.ltsv")

	// Create shared database (needed for constructor but not used in Dump)
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter and interactor
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	// Create real repositories for dump testing
	ltsvRepo := persistence.NewLTSVRepository()
	fileRepo := persistence.NewFileRepository()
	interactor := NewLTSVInteractor(adapter, ltsvRepo, fileRepo)

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

	expectedContent := `name:John	age:25	city:New York
name:Jane	age:30	city:Los Angeles
`

	if string(content) != expectedContent {
		t.Errorf("Output content mismatch.\nExpected:\n%s\nActual:\n%s", expectedContent, string(content))
	}

	// Verify LTSV format (key:value pairs separated by tabs)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for i, line := range lines {
		if !strings.Contains(line, "\t") {
			t.Errorf("Line %d does not contain tab separator: %s", i, line)
		}
		if !strings.Contains(line, ":") {
			t.Errorf("Line %d does not contain key:value separator: %s", i, line)
		}

		// Check that each field follows key:value format
		fields := strings.Split(line, "\t")
		for j, field := range fields {
			if !strings.Contains(field, ":") {
				t.Errorf("Field %d in line %d does not follow key:value format: %s", j, i, field)
			}
		}
	}
}

func TestLTSVInteractor_ListWithNilAdapter(t *testing.T) {
	t.Parallel()

	// Create interactor with nil adapter
	interactor := &ltsvInteractor{
		baseFileInteractor: &baseFileInteractor{filesqlAdapter: nil},
	}

	// Test List with nil adapter
	_, err := interactor.List("test.ltsv")
	if err == nil {
		t.Fatal("Expected error with nil adapter, got nil")
	}

	if !errors.Is(err, ErrFilesqlAdapterNotInitialized) {
		t.Errorf("Expected ErrFilesqlAdapterNotInitialized, got: %v", err)
	}
}

func TestNewLTSVInteractor(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create adapter
	adapter := filesql.NewFileSQLAdapter(sharedDB)

	// Test NewLTSVInteractor
	// Create real repositories for dump testing
	ltsvRepo := persistence.NewLTSVRepository()
	fileRepo := persistence.NewFileRepository()
	interactor := NewLTSVInteractor(adapter, ltsvRepo, fileRepo)

	if interactor == nil {
		t.Fatal("NewLTSVInteractor returned nil")
	}

	// Verify it implements the interface
	ltsvInteractor, ok := interactor.(*ltsvInteractor)
	if !ok {
		t.Fatal("NewLTSVInteractor did not return *ltsvInteractor")
	}

	if ltsvInteractor.filesqlAdapter != adapter {
		t.Error("NewLTSVInteractor did not set adapter correctly")
	}
}
