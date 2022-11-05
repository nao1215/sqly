package config

import "database/sql"

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
