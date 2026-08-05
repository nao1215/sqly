package testutil

import "database/sql"

// NewInMemHistoryDB returns a history database held in memory, and the function
// that closes it. The caller must have registered the "sqlite3" driver first
// (config.InitSQLite3 does it).
//
// It lives here rather than beside the file-backed constructor in config because
// nothing sqly ships ever wants one: the shell's history is a file so that it
// survives the session. Only a test wants history that disappears, and it wants
// it to skip the file I/O the real constructor performs, which is slow enough on
// Windows to show up in the suite's runtime.
//
// It returns *sql.DB rather than config.HistoryDB because config's own tests
// import this package, so importing config back would be a cycle. Nothing is
// lost: HistoryDB's underlying type is the unnamed *sql.DB, so the result is
// assignable to it without a conversion.
//
// The pool is pinned to one connection because SQLite gives each connection its
// own ":memory:" database, so a second connection would find no history table.
func NewInMemHistoryDB() (*sql.DB, func(), error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, func() { _ = db.Close() }, nil
}
