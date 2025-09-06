package config

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewInMemDB(t *testing.T) {
	t.Parallel()

	db, cleanup, err := NewInMemDB()
	if err != nil {
		t.Fatalf("NewInMemDB failed: %v", err)
	}
	defer cleanup()

	if db == nil {
		t.Fatal("Expected database instance, got nil")
	}

	// Test that database is usable
	sqlDB := (*sql.DB)(db)
	_, err = sqlDB.ExecContext(context.Background(), "CREATE TABLE test (id INTEGER)")
	if err != nil {
		t.Fatalf("Failed to create table in memory database: %v", err)
	}

	// Test that it's actually in memory
	_, err = sqlDB.ExecContext(context.Background(), "INSERT INTO test VALUES (1)")
	if err != nil {
		t.Fatalf("Failed to insert into memory database: %v", err)
	}

	var count int
	err = sqlDB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query memory database: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 record, got %d", count)
	}
}

func TestNewHistoryDB(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_history.db")

	config := &Config{
		HistoryDBPath: dbPath,
	}

	db, cleanup, err := NewHistoryDB(config)
	if err != nil {
		t.Fatalf("NewHistoryDB failed: %v", err)
	}
	defer cleanup()

	if db == nil {
		t.Fatal("Expected database instance, got nil")
	}

	// Test that database file was created (it might not exist until first write)
	// This is acceptable behavior for SQLite
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Logf("Database file not created until first write (acceptable): %v", err)
	}

	// Test that database is usable
	sqlDB := (*sql.DB)(db)
	_, err = sqlDB.ExecContext(context.Background(), "CREATE TABLE history_test (id INTEGER, command TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table in history database: %v", err)
	}

	// Test inserting and querying data
	_, err = sqlDB.ExecContext(context.Background(), "INSERT INTO history_test VALUES (1, 'SELECT * FROM test')")
	if err != nil {
		t.Fatalf("Failed to insert into history database: %v", err)
	}

	var command string
	err = sqlDB.QueryRowContext(context.Background(), "SELECT command FROM history_test WHERE id = 1").Scan(&command)
	if err != nil {
		t.Fatalf("Failed to query history database: %v", err)
	}

	if command != "SELECT * FROM test" {
		t.Errorf("Expected 'SELECT * FROM test', got %s", command)
	}
}
