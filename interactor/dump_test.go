package interactor

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/persistence"

	_ "modernc.org/sqlite"
)

func TestCSVInteractor_DumpError(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories and adapter for dump testing
	csvRepo := persistence.NewCSVRepository()
	fileRepo := persistence.NewFileRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

	// Create test table
	headers := []string{"name", "age", "city"}
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
	}
	table := model.NewTable("test", model.NewHeader(headers), records)

	// Test Dump to non-writable directory (should fail)
	err = interactor.Dump("/root/nonexistent/output.csv", table)
	if err == nil {
		t.Fatal("Expected Dump to fail when writing to non-existent directory")
	}

	// Should contain some indication it's a file/directory error
	if !strings.Contains(err.Error(), "no such file or directory") &&
		!strings.Contains(err.Error(), "permission denied") &&
		!strings.Contains(err.Error(), "cannot create") {
		t.Logf("Got expected error for invalid path: %v", err)
	}
}

func TestCSVInteractor_DumpSuccess(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories and adapter for dump testing
	csvRepo := persistence.NewCSVRepository()
	fileRepo := persistence.NewFileRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewCSVInteractor(adapter, csvRepo, fileRepo)

	// Create test table
	headers := []string{"name", "age", "city"}
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
	}
	table := model.NewTable("test", model.NewHeader(headers), records)

	// Test Dump to temporary file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.csv")

	err = interactor.Dump(outputFile, table)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	// Verify file was created and has content
	content, err := os.ReadFile(outputFile) //nolint:gosec // Reading from temporary test file
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "name,age,city") {
		t.Errorf("Expected CSV header in output, got: %s", contentStr)
	}

	if !strings.Contains(contentStr, "John,25,New York") {
		t.Errorf("Expected first record in output, got: %s", contentStr)
	}
}

func TestTSVInteractor_DumpSuccess(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories and adapter for dump testing
	tsvRepo := persistence.NewTSVRepository()
	fileRepo := persistence.NewFileRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewTSVInteractor(adapter, tsvRepo, fileRepo)

	// Create test table
	headers := []string{"name", "age", "city"}
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
	}
	table := model.NewTable("test", model.NewHeader(headers), records)

	// Test Dump to temporary file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.tsv")

	err = interactor.Dump(outputFile, table)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	// Verify file was created and has content
	content, err := os.ReadFile(outputFile) //nolint:gosec // Reading from temporary test file
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)
	// TSV should use tabs as separators
	if !strings.Contains(contentStr, "name\tage\tcity") {
		t.Errorf("Expected TSV header with tabs in output, got: %s", contentStr)
	}

	if !strings.Contains(contentStr, "John\t25\tNew York") {
		t.Errorf("Expected first record with tabs in output, got: %s", contentStr)
	}
}

func TestLTSVInteractor_DumpSuccess(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories and adapter for dump testing
	ltsvRepo := persistence.NewLTSVRepository()
	fileRepo := persistence.NewFileRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewLTSVInteractor(adapter, ltsvRepo, fileRepo)

	// Create test table
	headers := []string{"name", "age", "city"}
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
	}
	table := model.NewTable("test", model.NewHeader(headers), records)

	// Test Dump to temporary file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.ltsv")

	err = interactor.Dump(outputFile, table)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	// Verify file was created and has content
	content, err := os.ReadFile(outputFile) //nolint:gosec // Reading from temporary test file
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)
	// LTSV should use key:value format
	if !strings.Contains(contentStr, "name:John") {
		t.Errorf("Expected LTSV format in output, got: %s", contentStr)
	}

	if !strings.Contains(contentStr, "age:25") {
		t.Errorf("Expected age field in LTSV output, got: %s", contentStr)
	}

	if !strings.Contains(contentStr, "city:New York") {
		t.Errorf("Expected city field in LTSV output, got: %s", contentStr)
	}
}

func TestExcelInteractor_DumpSuccess(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	// Create real repositories and adapter for dump testing
	excelRepo := persistence.NewExcelRepository()
	adapter := filesql.NewFileSQLAdapter(sharedDB)
	interactor := NewExcelInteractor(adapter, excelRepo)

	// Create test table
	headers := []string{"name", "age", "city"}
	records := []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
	}
	table := model.NewTable("test", model.NewHeader(headers), records)

	// Test Dump to temporary file (Excel uses CSV fallback)
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.xlsx")

	err = interactor.Dump(outputFile, table)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	// Verify Excel file was created and has correct format
	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("Excel file was not created: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("Excel file is empty")
	}

	// Read first few bytes to verify it's an Excel file (starts with PK for zip format)
	content := make([]byte, 2)
	file, err := os.Open(outputFile) //nolint:gosec // Reading from test file
	if err != nil {
		t.Fatalf("Failed to open Excel file: %v", err)
	}
	defer file.Close()

	_, err = file.Read(content)
	if err != nil {
		t.Fatalf("Failed to read Excel file header: %v", err)
	}

	if len(content) < 2 || content[0] != 'P' || content[1] != 'K' {
		t.Errorf("Expected Excel file to start with PK (zip format), got: %v", content)
	}
}
