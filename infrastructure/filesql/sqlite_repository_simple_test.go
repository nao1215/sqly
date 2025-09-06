package filesql

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewSQLite3Repository_Simple(t *testing.T) {
	t.Parallel()

	// Create test database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	adapter := NewFileSQLAdapter(db)
	repo := NewSQLite3Repository(adapter)

	if repo == nil {
		t.Fatal("NewSQLite3Repository returned nil")
	}
}

func TestSQLite3Repository_Simple_Methods(t *testing.T) {
	t.Parallel()

	// Create test database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	adapter := NewFileSQLAdapter(db)
	repo := NewSQLite3Repository(adapter)
	ctx := context.Background()

	// Test Exec method with simple CREATE TABLE
	_, err = repo.Exec(ctx, "CREATE TABLE simple_test (id INTEGER)")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	// Test CreateTable method (should not error, even if it's a no-op)
	err = repo.CreateTable(ctx, nil)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// Test Insert method (should not error, even if it's a no-op)
	err = repo.Insert(ctx, nil)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Test TablesName method (should not error)
	_, err = repo.TablesName(ctx)
	if err != nil {
		t.Fatalf("TablesName failed: %v", err)
	}

	// Test List method (should handle non-existent table gracefully)
	_, err = repo.List(ctx, "nonexistent_table")
	// This might error or return empty, both are acceptable
	if err != nil {
		t.Logf("List failed as expected for nonexistent table: %v", err)
	}

	// Test Header method (should handle non-existent table gracefully)
	_, err = repo.Header(ctx, "nonexistent_table")
	// This might error or return empty, both are acceptable
	if err != nil {
		t.Logf("Header failed as expected for nonexistent table: %v", err)
	}

	// Test Query method with simple query
	_, err = repo.Query(ctx, "SELECT 1 as test_column")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
}

func TestSQLite3Repository_InvalidSQL(t *testing.T) {
	t.Parallel()

	// Create test database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	adapter := NewFileSQLAdapter(db)
	repo := NewSQLite3Repository(adapter)
	ctx := context.Background()

	// Test with invalid SQL - should return error
	_, err = repo.Exec(ctx, "INVALID SQL SYNTAX")
	if err == nil {
		t.Fatal("Expected error for invalid SQL, got nil")
	}

	_, err = repo.Query(ctx, "INVALID SELECT SYNTAX")
	if err == nil {
		t.Fatal("Expected error for invalid query, got nil")
	}
}
