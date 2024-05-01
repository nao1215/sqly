package config

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"

	"modernc.org/sqlite"
)

// MemoryDB is *sql.DB for excuting sql.
type MemoryDB *sql.DB

// HistoryDB is *sql.DB for sqly shell history.
type HistoryDB *sql.DB

// NewInMemDB create *sql.DB for SQLite3. SQLite3 store data in memory.
// The return function is the function to close the DB.
func NewInMemDB() (MemoryDB, func(), error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, nil, err
	}
	return MemoryDB(db), func() { db.Close() }, nil
}

// NewHistoryDB create *sql.DB for history.
// The return function is the function to close the DB.
func NewHistoryDB(c *Config) (HistoryDB, func(), error) {
	db, err := sql.Open("sqlite3", c.HistoryDBPath)
	if err != nil {
		return nil, nil, err
	}
	return HistoryDB(db), func() { db.Close() }, nil
}

// InitSQLite3 registers the sqlite3 driver.
func InitSQLite3() {
	var once sync.Once
	once.Do(func() {
		sql.Register("sqlite3", sqliteDriver{Driver: &sqlite.Driver{}})
	})
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
	c := conn.(interface {
		Exec(stmt string, args []driver.Value) (driver.Result, error)
	})
	if _, err := c.Exec("PRAGMA foreign_keys = on;", nil); err != nil {
		if err := conn.Close(); err != nil {
			return nil, fmt.Errorf("failed to close connection: %w", err)
		}
		return nil, fmt.Errorf("failed to enable enable foreign keys: %w", err)
	}
	return conn, nil
}
