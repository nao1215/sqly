package interactor

import (
	"context"
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

func TestBaseFileInteractor_ListWithNilAdapter(t *testing.T) {
	t.Parallel()

	base := &baseFileInteractor{
		filesqlAdapter: nil,
		f:              nil,
	}

	_, err := base.list("test.csv", "CSV")
	if err == nil {
		t.Fatal("Expected error with nil adapter, got nil")
	}

	if !errors.Is(err, ErrFilesqlAdapterNotInitialized) {
		t.Errorf("Expected ErrFilesqlAdapterNotInitialized, got: %v", err)
	}
}

func TestBaseFileInteractor_ListWithNonexistentFile(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := filesql.NewFileSQLAdapter(sharedDB)
	fileRepo := persistence.NewFileRepository()

	base := newBaseFileInteractor(adapter, fileRepo)

	_, err = base.list("/nonexistent/path/file.csv", "CSV")
	if err == nil {
		t.Fatal("Expected error when file doesn't exist")
	}

	if !strings.Contains(err.Error(), "failed to load CSV file") {
		t.Errorf("Expected 'failed to load CSV file' error, got: %v", err)
	}
}

func TestBaseFileInteractor_LoadFileWithNilAdapter(t *testing.T) {
	t.Parallel()

	base := &baseFileInteractor{
		filesqlAdapter: nil,
		f:              nil,
	}

	_, err := base.loadFile("test.csv", "CSV")
	if err == nil {
		t.Fatal("Expected error with nil adapter, got nil")
	}

	if !errors.Is(err, ErrFilesqlAdapterNotInitialized) {
		t.Errorf("Expected ErrFilesqlAdapterNotInitialized, got: %v", err)
	}
}

func TestBaseFileInteractor_GetTableNames(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := filesql.NewFileSQLAdapter(sharedDB)
	fileRepo := persistence.NewFileRepository()

	base := newBaseFileInteractor(adapter, fileRepo)

	ctx := context.Background()
	tables, err := base.getTableNames(ctx)
	if err != nil {
		t.Fatalf("Failed to get table names: %v", err)
	}

	// Initially should be empty
	if len(tables) != 0 {
		t.Errorf("Expected 0 tables, got %d", len(tables))
	}
}

func TestBaseFileInteractor_QueryTableWithInvalidQuery(t *testing.T) {
	t.Parallel()

	// Create shared database
	sharedDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create shared database: %v", err)
	}
	defer sharedDB.Close()

	adapter := filesql.NewFileSQLAdapter(sharedDB)
	fileRepo := persistence.NewFileRepository()

	base := newBaseFileInteractor(adapter, fileRepo)

	ctx := context.Background()
	_, err = base.queryTable(ctx, "SELECT * FROM nonexistent_table", "CSV")
	if err == nil {
		t.Fatal("Expected error when querying nonexistent table")
	}

	if !strings.Contains(err.Error(), "failed to query CSV data") {
		t.Errorf("Expected 'failed to query CSV data' error, got: %v", err)
	}
}

func TestBaseFileInteractor_DumpWithNilFileRepository(t *testing.T) {
	t.Parallel()

	base := &baseFileInteractor{
		filesqlAdapter: nil,
		f:              nil,
	}

	// Create test table
	header := model.NewHeader([]string{"id"})
	table := model.NewTable("test", header, nil)

	// Mock dump function that should not be called due to nil repository
	dumpFunc := func(*os.File, *model.Table) error {
		t.Fatal("Dump function should not be called when file repository is nil")
		return nil
	}

	err := base.dump("test.csv", table, dumpFunc)
	if err == nil {
		t.Fatal("Expected error when file repository is nil")
	}

	if !errors.Is(err, ErrFileRepositoryNotInitialized) {
		t.Errorf("Expected ErrFileRepositoryNotInitialized, got: %v", err)
	}
}

func TestBaseFileInteractor_DumpWithInvalidPath(t *testing.T) {
	t.Parallel()

	fileRepo := persistence.NewFileRepository()
	base := newBaseFileInteractor(nil, fileRepo)

	// Create test table
	header := model.NewHeader([]string{"id"})
	table := model.NewTable("test", header, nil)

	// Mock dump function that should not be called due to file creation failure
	dumpFunc := func(*os.File, *model.Table) error {
		t.Fatal("Dump function should not be called when file creation fails")
		return nil
	}

	err := base.dump("/nonexistent/directory/file.csv", table, dumpFunc)
	if err == nil {
		t.Fatal("Expected error when dumping to invalid path")
	}

	// Check for path-related errors (different messages on different platforms)
	if !strings.Contains(err.Error(), "no such file or directory") &&
		!strings.Contains(err.Error(), "The system cannot find the path specified") {
		t.Errorf("Expected path-related error, got: %v", err)
	}
}

func TestBaseFileInteractor_DumpSuccess(t *testing.T) {
	t.Parallel()

	fileRepo := persistence.NewFileRepository()
	base := newBaseFileInteractor(nil, fileRepo)

	// Create test table with data
	header := model.NewHeader([]string{"id", "name"})
	records := []model.Record{
		{"1", "Alice"},
		{"2", "Bob"},
	}
	table := model.NewTable("test", header, records)

	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.csv")

	// Mock dump function that writes to the file
	dumpCalled := false
	dumpFunc := func(f *os.File, t *model.Table) error {
		dumpCalled = true
		if t.Name() != "test" {
			return errors.New("unexpected table name")
		}
		// Write a simple test content
		_, err := f.WriteString("id,name\n1,Alice\n2,Bob\n")
		return err
	}

	err := base.dump(testFile, table, dumpFunc)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	if !dumpCalled {
		t.Error("Dump function was not called")
	}

	// Verify file was created and has content
	content, err := os.ReadFile(filepath.Clean(testFile))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Alice") || !strings.Contains(contentStr, "Bob") {
		t.Errorf("Expected content not found in file: %s", contentStr)
	}
}

func TestBaseFileInteractor_DumpWithDumpFunctionError(t *testing.T) {
	t.Parallel()

	fileRepo := persistence.NewFileRepository()
	base := newBaseFileInteractor(nil, fileRepo)

	// Create test table
	header := model.NewHeader([]string{"id"})
	table := model.NewTable("test", header, nil)

	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.csv")

	// Mock dump function that returns an error
	expectedError := errors.New("dump function error")
	dumpFunc := func(*os.File, *model.Table) error {
		return expectedError
	}

	err := base.dump(testFile, table, dumpFunc)
	if err == nil {
		t.Fatal("Expected error from dump function")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected dump function error, got: %v", err)
	}
}
