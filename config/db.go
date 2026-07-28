package config

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"

	"github.com/nao1215/filesql/dialect"
	"modernc.org/sqlite"
)

// MemoryDB is *sql.DB for excuting sql.
type MemoryDB *sql.DB

// HistoryDB is *sql.DB for sqly shell history.
type HistoryDB *sql.DB

// NewInMemDB create *sql.DB for SQLite3. SQLite3 store data in memory.
// The return function is the function to close the DB.
//
// The pool is pinned to a single connection because SQLite ":memory:" is private
// per connection: a second connection would see an empty database. This also lets
// filesql stream imported files directly into this database (filesql.LoadInto)
// instead of building a separate database and copying every row across.
func NewInMemDB() (MemoryDB, func(), error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	return MemoryDB(db), func() { _ = db.Close() }, nil // #nosec G104
}

// NewHistoryDB create *sql.DB for history.
// The return function is the function to close the DB.
func NewHistoryDB(c *Config) (HistoryDB, func(), error) {
	db, err := sql.Open("sqlite3", c.HistoryDBPath)
	if err != nil {
		return nil, nil, err
	}
	return HistoryDB(db), func() { _ = db.Close() }, nil // #nosec G104
}

// NewInMemHistoryDB creates an in-memory history DB for testing.
// This avoids file I/O overhead that is especially costly on Windows.
// The pool is pinned to a single connection because SQLite's ":memory:"
// creates a separate database per connection.
func NewInMemHistoryDB() (HistoryDB, func(), error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return HistoryDB(db), func() { _ = db.Close() }, nil // #nosec G104
}

// sqlite3RegisterOnce is package-level rather than function-local so repeated
// InitSQLite3 calls register the sqlite3 driver exactly once; database/sql
// panics with "Register called twice" otherwise.
var sqlite3RegisterOnce sync.Once

// InitSQLite3 registers the sqlite3 driver. It is safe to call multiple times.
//
// It also registers the dialect helper functions (NOW, SPLIT_PART, SAFE_DIVIDE,
// ...) so queries translated from MySQL/PostgreSQL/GoogleSQL resolve them. This
// runs before any database connection is opened, which is required because
// modernc exposes registered functions only to connections opened afterward.
// The helper functions are available under every dialect, including the default
// SQLite one.
func InitSQLite3() {
	sqlite3RegisterOnce.Do(func() {
		// A registration failure would be a programming error in the dialect
		// package (an invalid function definition); its own tests cover that, so
		// a failure here only leaves the helpers unavailable, which surfaces as a
		// clear "no such function" error at query time.
		_ = dialect.RegisterFunctions()
		sql.Register("sqlite3", sqliteDriver{Driver: moderncSQLiteDriver()})
	})
}

// moderncSQLiteDriver returns the *sqlite.Driver instance modernc registered
// under its "sqlite" name. The "sqlite3" driver wraps this instance rather than
// a fresh &sqlite.Driver{} because modernc binds functions registered via
// dialect.RegisterFunctions to that specific instance; a fresh instance would
// not expose them, so a translated NOW()/SPLIT_PART()/... would fail with "no
// such function".
func moderncSQLiteDriver() *sqlite.Driver {
	// sql.Open is lazy, so this does not open a real connection; it only exposes
	// the registered driver instance via (*sql.DB).Driver.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return &sqlite.Driver{}
	}
	defer func() { _ = db.Close() }()
	if d, ok := db.Driver().(*sqlite.Driver); ok {
		return d
	}
	return &sqlite.Driver{}
}

// sqliteDriver is a driver that enables foreign keys.
type sqliteDriver struct {
	*sqlite.Driver
}

// Open opens a database specified by its database driver name and a driver-specific data source name.
func (d sqliteDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return conn, err
	}
	c, ok := conn.(interface {
		Exec(stmt string, args []driver.Value) (driver.Result, error)
	})
	if !ok {
		return nil, errors.New("connection does not support Exec method")
	}

	if _, err := c.Exec("PRAGMA foreign_keys = on;", nil); err != nil {
		if err := conn.Close(); err != nil {
			return nil, fmt.Errorf("failed to close connection: %w", err)
		}
		return nil, fmt.Errorf("failed to enable enable foreign keys: %w", err)
	}

	// Wait up to 5s for a held lock instead of failing immediately with
	// SQLITE_BUSY. The history DB is a shared file, so two sqly processes can write
	// it concurrently; without a busy timeout one process would disable history on
	// transient lock contention and print a misleading "set a writable path"
	// warning even though the path is writable.
	if _, err := c.Exec("PRAGMA busy_timeout = 5000;", nil); err != nil {
		if err := conn.Close(); err != nil {
			return nil, fmt.Errorf("failed to close connection: %w", err)
		}
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	return conn, nil
}
