package config

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

// TestNewArgErrorNilPassthrough verifies that newArgError returns nil for a nil
// input so callers can wrap a result unconditionally, and wraps a non-nil error
// as an *ArgError that still unwraps to the original error.
func TestNewArgErrorNilPassthrough(t *testing.T) {
	t.Parallel()

	if got := newArgError(nil); got != nil {
		t.Errorf("newArgError(nil) = %v, want nil", got)
	}

	sentinel := errors.New("boom")
	wrapped := newArgError(sentinel)
	if wrapped == nil {
		t.Fatal("newArgError(non-nil) = nil, want non-nil")
	}
	var argErr *ArgError
	if !errors.As(wrapped, &argErr) {
		t.Fatalf("newArgError(non-nil) = %T, want *ArgError", wrapped)
	}
	if !errors.Is(wrapped, sentinel) {
		t.Errorf("wrapped error does not unwrap to the original sentinel")
	}
	if wrapped.Error() != sentinel.Error() {
		t.Errorf("Error() = %q, want %q", wrapped.Error(), sentinel.Error())
	}
}

// TestNewConfigCreateDirFailure covers the branch of NewConfig where creating
// the default config directory fails. Pointing xdg.ConfigHome at a path whose
// parent is a regular file makes os.MkdirAll fail, so NewConfig must return that
// error. This test mutates the process-global xdg.ConfigHome and must not run in
// parallel.
func TestNewConfigCreateDirFailure(t *testing.T) {
	// Ensure history falls back to the default location so CreateDir runs.
	t.Setenv("SQLY_HISTORY_DB_PATH", "")

	fileAsParent := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0o600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	orgConfigHome := xdg.ConfigHome
	xdg.ConfigHome = filepath.Join(fileAsParent, "child")
	t.Cleanup(func() { xdg.ConfigHome = orgConfigHome })

	if _, err := NewConfig(); err == nil {
		t.Fatal("NewConfig() = nil error, want error when config dir cannot be created")
	}
}

// TestNewConfigCreatesDefaultDir covers the success branch of NewConfig where
// history uses the default location and the XDG config directory is created.
func TestNewConfigCreatesDefaultDir(t *testing.T) {
	t.Setenv("SQLY_HISTORY_DB_PATH", "")

	configHome := t.TempDir()
	orgConfigHome := xdg.ConfigHome
	xdg.ConfigHome = configHome
	t.Cleanup(func() { xdg.ConfigHome = orgConfigHome })

	c, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	wantPath := filepath.Join(configHome, "sqly", "history.db")
	if c.HistoryDBPath != wantPath {
		t.Errorf("HistoryDBPath = %q, want %q", c.HistoryDBPath, wantPath)
	}
}

// TestNewInMemHistoryDBUsable covers NewInMemHistoryDB by opening the in-memory
// history database and exercising it end to end.
func TestNewInMemHistoryDBUsable(t *testing.T) {
	t.Parallel()

	db, cleanup, err := NewInMemHistoryDB()
	if err != nil {
		t.Fatalf("NewInMemHistoryDB() error = %v", err)
	}
	defer cleanup()

	sqlDB := (*sql.DB)(db)
	if _, err := sqlDB.ExecContext(context.Background(), "CREATE TABLE h (id INTEGER)"); err != nil {
		t.Fatalf("failed to use in-memory history DB: %v", err)
	}
}

// TestSqliteDriverOpenInvalidPath covers the error branch of sqliteDriver.Open:
// opening a database file whose directory does not exist makes the underlying
// driver Open fail, so no connection can be established.
func TestSqliteDriverOpenInvalidPath(t *testing.T) {
	t.Parallel()

	badPath := filepath.Join(t.TempDir(), "no-such-dir", "history.db")
	db, err := sql.Open("sqlite3", badPath)
	if err != nil {
		t.Fatalf("sql.Open returned error too early: %v", err)
	}
	defer func() { _ = db.Close() }()

	// The driver Open (and its PRAGMA setup) runs lazily on first use.
	if err := db.PingContext(context.Background()); err == nil {
		t.Fatal("PingContext() = nil error, want error for a file in a nonexistent directory")
	}
}

// TestSqliteDriverOpenSuccess covers the success path of sqliteDriver.Open,
// where the foreign_keys and busy_timeout PRAGMAs are applied to a fresh
// connection.
func TestSqliteDriverOpenSuccess(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "ok.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("PingContext() error = %v", err)
	}

	var fk int
	if err := db.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("failed to read foreign_keys pragma: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1 (driver Open should enable it)", fk)
	}
}
