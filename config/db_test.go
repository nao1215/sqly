package config

import (
	"context"
	"database/sql"
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

// TestSQLite3DriverEnablesForeignKeys checks the one pragma the driver sets. The
// busy timeout that used to sit beside it was for the history database, a file
// two sqly processes could hold at once; the session database is private to the
// process, so nothing waits on a lock here.
func TestSQLite3DriverEnablesForeignKeys(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var foreignKeys int
	if err := db.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}
}
