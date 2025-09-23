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

// Helper function to create test files for testing
func createTestFiles(t *testing.T) (string, func()) {
	t.Helper()

	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test CSV file
	csvContent := "name,age,city\nAlice,30,New York\nBob,25,Tokyo\n"
	csvPath := filepath.Join(tmpDir, "users.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0600); err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}

	// Create test TSV file
	tsvContent := "product\tprice\tstock\nLaptop\t1000\t50\nMouse\t20\t100\n"
	tsvPath := filepath.Join(tmpDir, "products.tsv")
	if err := os.WriteFile(tsvPath, []byte(tsvContent), 0600); err != nil {
		t.Fatalf("Failed to create test TSV: %v", err)
	}

	cleanup := func() {
		// No-op since t.TempDir() handles cleanup automatically
	}

	return tmpDir, cleanup
}

func TestNewFileSQLInteractor(t *testing.T) {
	t.Parallel()

	t.Run("create new interactor successfully", func(t *testing.T) {
		t.Parallel()

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create FileSQLAdapter with the database
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		if interactor == nil {
			t.Error("Expected non-nil interactor")
		}

		// Verify it implements the interface
		_, ok := interactor.(*FileSQLInteractor)
		if !ok {
			t.Error("Expected *FileSQLInteractor type")
		}
	})
}

func TestFileSQLInteractor_LoadFiles(t *testing.T) {
	t.Parallel()

	t.Run("load single file successfully", func(t *testing.T) {
		t.Parallel()

		// Create test file
		tmpDir, cleanup := createTestFiles(t)
		defer cleanup()

		csvPath := filepath.Join(tmpDir, "users.csv")

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()
		err = interactor.LoadFiles(ctx, csvPath)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify table was created
		tables, err := interactor.GetTableNames(ctx)
		if err != nil {
			t.Errorf("Failed to get table names: %v", err)
		}

		if len(tables) != 1 {
			t.Errorf("Expected 1 table, got %d", len(tables))
		}

		if len(tables) > 0 && tables[0].Name() != "users" {
			t.Errorf("Expected table name 'users', got '%s'", tables[0].Name())
		}
	})

	t.Run("load multiple files successfully", func(t *testing.T) {
		t.Parallel()

		// Create test files
		tmpDir, cleanup := createTestFiles(t)
		defer cleanup()

		csvPath := filepath.Join(tmpDir, "users.csv")
		tsvPath := filepath.Join(tmpDir, "products.tsv")

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()
		err = interactor.LoadFiles(ctx, csvPath, tsvPath)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify tables were created
		tables, err := interactor.GetTableNames(ctx)
		if err != nil {
			t.Errorf("Failed to get table names: %v", err)
		}

		if len(tables) != 2 {
			t.Errorf("Expected 2 tables, got %d", len(tables))
		}

		// Check table names (order might vary)
		tableNames := make(map[string]bool)
		for _, table := range tables {
			tableNames[table.Name()] = true
		}

		if !tableNames["users"] {
			t.Error("Expected 'users' table not found")
		}
		if !tableNames["products"] {
			t.Error("Expected 'products' table not found")
		}
	})

	t.Run("load directory successfully", func(t *testing.T) {
		t.Parallel()

		// Create test files in directory
		tmpDir, cleanup := createTestFiles(t)
		defer cleanup()

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()
		err = interactor.LoadFiles(ctx, tmpDir)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify tables were created from directory
		tables, err := interactor.GetTableNames(ctx)
		if err != nil {
			t.Errorf("Failed to get table names: %v", err)
		}

		if len(tables) != 2 {
			t.Errorf("Expected 2 tables from directory, got %d", len(tables))
		}
	})

	t.Run("handle nonexistent file", func(t *testing.T) {
		t.Parallel()

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()
		err = interactor.LoadFiles(ctx, "nonexistent.csv")

		if err == nil {
			t.Error("Expected error for nonexistent file, got nil")
		}
	})

	t.Run("handle empty file paths", func(t *testing.T) {
		t.Parallel()

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()
		err = interactor.LoadFiles(ctx)

		if err != nil {
			t.Errorf("Expected no error for empty paths, got %v", err)
		}

		// Verify no tables were created
		tables, err := interactor.GetTableNames(ctx)
		if err != nil {
			t.Errorf("Failed to get table names: %v", err)
		}

		if len(tables) != 0 {
			t.Errorf("Expected 0 tables, got %d", len(tables))
		}
	})
}

func TestFileSQLInteractor_GetTableNames(t *testing.T) {
	t.Parallel()

	t.Run("get table names after loading files", func(t *testing.T) {
		t.Parallel()

		// Create test files
		tmpDir, cleanup := createTestFiles(t)
		defer cleanup()

		csvPath := filepath.Join(tmpDir, "users.csv")
		tsvPath := filepath.Join(tmpDir, "products.tsv")

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()

		// Load files first
		err = interactor.LoadFiles(ctx, csvPath, tsvPath)
		if err != nil {
			t.Fatalf("Failed to load files: %v", err)
		}

		// Get table names
		tables, err := interactor.GetTableNames(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(tables) != 2 {
			t.Errorf("Expected 2 tables, got %d", len(tables))
		}

		// Check table names (order might vary)
		tableNames := make(map[string]bool)
		for _, table := range tables {
			tableNames[table.Name()] = true
		}

		if !tableNames["users"] {
			t.Error("Expected 'users' table not found")
		}
		if !tableNames["products"] {
			t.Error("Expected 'products' table not found")
		}
	})

	t.Run("get empty table list from empty database", func(t *testing.T) {
		t.Parallel()

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()
		tables, err := interactor.GetTableNames(ctx)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(tables) != 0 {
			t.Errorf("Expected empty table list, got %d tables", len(tables))
		}
	})
}

func TestFileSQLInteractor_Integration(t *testing.T) {
	t.Parallel()

	t.Run("full workflow load files then get tables", func(t *testing.T) {
		t.Parallel()

		// Create test files
		tmpDir, cleanup := createTestFiles(t)
		defer cleanup()

		csvPath := filepath.Join(tmpDir, "users.csv")
		tsvPath := filepath.Join(tmpDir, "products.tsv")

		// Create shared database
		sharedDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to create shared database: %v", err)
		}
		defer sharedDB.Close()

		// Create adapter and interactor
		adapter := filesql.NewFileSQLAdapter(sharedDB)
		interactor := NewFileSQLInteractor(adapter)

		ctx := context.Background()

		// Verify initially no tables
		tables, err := interactor.GetTableNames(ctx)
		if err != nil {
			t.Errorf("Expected no error getting initial tables, got %v", err)
		}
		if len(tables) != 0 {
			t.Errorf("Expected 0 initial tables, got %d", len(tables))
		}

		// Load files
		err = interactor.LoadFiles(ctx, csvPath, tsvPath)
		if err != nil {
			t.Errorf("Expected no error loading files, got %v", err)
		}

		// Get table names after loading
		tables, err = interactor.GetTableNames(ctx)
		if err != nil {
			t.Errorf("Expected no error getting tables after load, got %v", err)
		}

		// Verify tables were created
		if len(tables) != 2 {
			t.Errorf("Expected 2 tables after loading, got %d", len(tables))
		}

		// Check table names (order might vary)
		tableNames := make(map[string]bool)
		for _, table := range tables {
			tableNames[table.Name()] = true
		}

		if !tableNames["users"] {
			t.Error("Expected 'users' table not found after loading")
		}
		if !tableNames["products"] {
			t.Error("Expected 'products' table not found after loading")
		}
	})
}
