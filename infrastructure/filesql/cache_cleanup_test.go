package filesql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nao1215/sqly/domain/cleanup"
	infra "github.com/nao1215/sqly/infrastructure"
	_ "modernc.org/sqlite"
)

// newCacheFixture builds a session with one imported table and writes a cache
// snapshot of it, returning the cache path and a fresh empty session to load it
// into. Each caller gets its own temp directory and its own database.
//
// The pool is deliberately allowed more than one connection. SQLite's ATTACH is
// per-connection state, so a single-connection pool would hide exactly the bug
// these tests exist to catch.
func newCacheFixture(t *testing.T) (cachePath string, target *FileSQLAdapter, targetDB *sql.DB) {
	t.Helper()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "people.csv")
	if err := os.WriteFile(csvPath, []byte("id,name\n1,alice\n2,bob\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	sourceDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sourceDB.Close() })
	source := NewFileSQLAdapter(sourceDB)
	if err := source.LoadFiles(t.Context(), csvPath); err != nil {
		t.Fatalf("seed import: %v", err)
	}

	cachePath = filepath.Join(dir, "cache.db")
	if err := source.SnapshotToCache(t.Context(), cachePath); err != nil {
		t.Fatalf("SnapshotToCache: %v", err)
	}

	// A file-backed session, so several pooled connections address the same
	// database. ":memory:" gives each connection its own private database, which
	// would make a multi-connection test meaningless.
	targetDB, err = sql.Open("sqlite", filepath.Join(dir, "session.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = targetDB.Close() })
	targetDB.SetMaxOpenConns(4)
	targetDB.SetMaxIdleConns(4)
	return cachePath, NewFileSQLAdapter(targetDB), targetDB
}

// warmPool forces the pool to hold several distinct idle connections, so a
// subsequent operation is genuinely liable to be handed a different one. Without
// this, a pool configured for four connections may still only ever have opened
// one, and a connection-affinity bug would go unnoticed.
func warmPool(t *testing.T, db *sql.DB, n int) {
	t.Helper()

	conns := make([]*sql.Conn, 0, n)
	for range n {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("open connection: %v", err)
		}
		if err := conn.PingContext(t.Context()); err != nil {
			t.Fatalf("ping: %v", err)
		}
		conns = append(conns, conn)
	}
	// Returning them all at once leaves n idle connections behind.
	for _, conn := range conns {
		if err := conn.Close(); err != nil {
			t.Fatalf("return connection: %v", err)
		}
	}
	if got := db.Stats().OpenConnections; got < n {
		t.Fatalf("pool holds %d connections, want at least %d", got, n)
	}
}

// assertNoConnectionHoldsAlias drains every connection the pool can open and
// checks each one. Asking the pool once would answer about whichever connection
// it happened to hand out, so a leak left on a different connection would go
// unseen — and that leak is the whole failure mode: it surfaces later as
// "database sqly_cache is already in use" in an unrelated run.
func assertNoConnectionHoldsAlias(t *testing.T, db *sql.DB, maxConns int) {
	t.Helper()

	conns := make([]*sql.Conn, 0, maxConns)
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()
	for i := range maxConns {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("open connection %d: %v", i, err)
		}
		conns = append(conns, conn)
		if aliasAttachedOn(t, conn) {
			t.Errorf("connection %d still holds %s attached", i, cacheAlias)
		}
	}
}

// aliasAttachedOn reports whether the cache alias is attached on this specific
// connection. Asking the pool instead would answer about an arbitrary
// connection, which is the confusion under test.
func aliasAttachedOn(t *testing.T, conn *sql.Conn) bool {
	t.Helper()

	rows, err := conn.QueryContext(t.Context(), "PRAGMA database_list")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var seq int
		var name string
		var file sql.NullString
		if err := rows.Scan(&seq, &name, &file); err != nil {
			t.Fatal(err)
		}
		if name == cacheAlias {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}

// sessionTables returns the user tables in the session database.
func sessionTables(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()

	rows, err := db.QueryContext(t.Context(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestLoadFromCacheOnAPooledDatabase is the connection-affinity regression.
//
// ATTACH is per-connection state and *sql.DB is a pool, so running the attach,
// the schema read, the copy, and the detach as separate pool calls lets them
// land on different connections. The copy then fails with "no such database:
// sqly_cache", or the detach releases a connection that had nothing attached
// while the real attachment stays behind on another. The pool here holds four
// warm connections, so an unpinned implementation has three ways to be wrong.
func TestLoadFromCacheOnAPooledDatabase(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 4)

	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}

	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM people").Scan(&n); err != nil {
		t.Fatalf("query the restored table: %v", err)
	}
	if n != 2 {
		t.Errorf("restored rows = %d, want 2", n)
	}

	// Every connection the pool can hand out must be free of the alias, not just
	// whichever one a single check happens to get.
	assertNoConnectionHoldsAlias(t, db, 4)

	// And running it again must not hit "already in use".
	if _, err := db.ExecContext(t.Context(), "DROP TABLE people"); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
			t.Fatalf("repeat %d: %v", i, err)
		}
		if _, err := db.ExecContext(t.Context(), "DROP TABLE people"); err != nil {
			t.Fatal(err)
		}
	}
}

// TestLoadFromCacheConcurrentRunsDoNotCollide checks that the fixed alias is
// held for as short a time as possible and always released, by running several
// loads at once against a pool that can serve them.
func TestLoadFromCacheConcurrentRunsDoNotCollide(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 4)

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Concurrent loads race for the alias, so a collision is a legitimate
			// outcome; a leaked attachment is not. Failures are inspected below.
			errs[i] = adapter.LoadFromCache(t.Context(), cachePath)
		}()
	}
	wg.Wait()

	// Whatever happened, the alias must be free afterwards: a fresh load works.
	if _, err := db.ExecContext(t.Context(), "DROP TABLE IF EXISTS people"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after concurrent runs: %v; earlier errors: %v", err, errs)
	}
	assertNoConnectionHoldsAlias(t, db, 4)
}

// TestLoadFromCacheAttachFailure covers the failure before anything is
// attached: the cache file does not exist, so the error names it and no detach
// is claimed.
func TestLoadFromCacheAttachFailure(t *testing.T) {
	t.Parallel()

	_, adapter, db := newCacheFixture(t)
	warmPool(t, db, 2)
	missing := filepath.Join(t.TempDir(), "nope.db")

	err := adapter.LoadFromCache(t.Context(), missing)
	if err == nil {
		t.Fatal("LoadFromCache on a missing cache = nil, want an error")
	}
	if !strings.Contains(err.Error(), "nope.db") {
		t.Errorf("err = %v, want it to name the cache path", err)
	}
	if errors.Is(err, cleanup.ErrCleanup) {
		t.Errorf("a detach was reported though nothing was attached: %v", err)
	}
}

// TestLoadFromCacheDetachesAfterAFailedLoad is "primary failed, cleanup
// succeeded": the cache attaches but holds no tables, so the load fails and the
// alias is still released.
func TestLoadFromCacheDetachesAfterAFailedLoad(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 2)

	emptyPath := filepath.Join(t.TempDir(), "empty.db")
	emptyDB, err := sql.Open("sqlite", emptyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{"CREATE TABLE seed (a)", "DROP TABLE seed"} {
		if _, err := emptyDB.ExecContext(t.Context(), stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := emptyDB.Close(); err != nil {
		t.Fatal(err)
	}

	err = adapter.LoadFromCache(t.Context(), emptyPath)
	if err == nil {
		t.Fatal("LoadFromCache on a table-less cache = nil, want an error")
	}
	if !strings.Contains(err.Error(), "no tables") {
		t.Errorf("err = %v, want it to say the cache holds no tables", err)
	}
	if errors.Is(err, cleanup.ErrCleanup) {
		t.Errorf("the detach failed too: %v", err)
	}
	// The real cache still loads, which it could not if the alias leaked.
	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after a failed load: %v", err)
	}
}

// TestLoadFromCacheRetriesAfterFailure is the recovery property: a failed load
// must not poison the session.
func TestLoadFromCacheRetriesAfterFailure(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 2)

	missing := filepath.Join(t.TempDir(), "absent.db")
	for i := range 3 {
		if err := adapter.LoadFromCache(t.Context(), missing); err == nil {
			t.Fatalf("attempt %d on a missing cache = nil, want an error", i)
		}
	}

	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after three failures: %v", err)
	}
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM people").Scan(&n); err != nil {
		t.Fatalf("query the restored table: %v", err)
	}
	if n != 2 {
		t.Errorf("restored rows = %d, want 2", n)
	}
}

// TestWithAttachedCacheRunsEverythingOnOneConnection asserts the pinning
// directly: the connection fn receives is the one the cache is attached to, and
// the pool's other connections know nothing about it.
func TestWithAttachedCacheRunsEverythingOnOneConnection(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 4)

	err := adapter.withAttachedCache(t.Context(), cachePath, func(ctx context.Context, conn *sql.Conn) error {
		if !aliasAttachedOn(t, conn) {
			t.Error("the cache is not attached on the connection fn was given")
		}
		// The attachment is visible from this connection...
		var n int
		if err := conn.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM "+cacheAlias+".people").Scan(&n); err != nil {
			t.Errorf("read the cache on the pinned connection: %v", err)
		}
		// ...and a different connection must not see it, which is what makes
		// running any part of this through the pool a defect.
		other, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("open a second connection: %v", err)
		}
		defer func() { _ = other.Close() }()
		if aliasAttachedOn(t, other) {
			t.Error("a second pooled connection sees the attachment; the test cannot detect pinning")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withAttachedCache: %v", err)
	}
}

// TestWithAttachedCacheDetachesAfterCancellation is the case cleanup.Context
// exists for: the attach succeeds and the context dies afterwards. Detaching
// with that dead context fails immediately, so the alias would stay held for
// the life of the process.
func TestWithAttachedCacheDetachesAfterCancellation(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 2)

	ctx, cancel := context.WithCancel(t.Context())
	workErr := errors.New("interrupted mid-copy")

	var pinned *sql.Conn
	err := adapter.withAttachedCache(ctx, cachePath, func(_ context.Context, conn *sql.Conn) error {
		if !aliasAttachedOn(t, conn) {
			t.Fatal("precondition: the cache was not attached")
		}
		pinned = conn
		cancel()
		return workErr
	})
	if !errors.Is(err, workErr) {
		t.Errorf("errors.Is(err, workErr) = false; err = %v", err)
	}
	if errors.Is(err, cleanup.ErrCleanup) {
		t.Errorf("the detach failed despite the cleanup context: %v", err)
	}
	_ = pinned

	// The proof it was released: the same alias attaches again.
	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after a cancelled attach: %v", err)
	}
}

// TestWithAttachedCacheKeepsBothErrors is "primary failed and cleanup failed".
// The callback detaches the alias itself, so the deferred detach fails for a
// reason unrelated to the work; both must reach the caller.
func TestWithAttachedCacheKeepsBothErrors(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 2)

	workErr := errors.New("copy failed")
	err := adapter.withAttachedCache(t.Context(), cachePath, func(ctx context.Context, conn *sql.Conn) error {
		if _, derr := conn.ExecContext(ctx, "DETACH DATABASE "+cacheAlias); derr != nil {
			t.Fatalf("precondition detach: %v", derr)
		}
		return workErr
	})

	if err == nil {
		t.Fatal("err = nil, want both failures")
	}
	if !errors.Is(err, workErr) {
		t.Errorf("the primary error was lost; err = %v", err)
	}
	if !errors.Is(err, cleanup.ErrCleanup) {
		t.Errorf("the detach failure was lost; err = %v", err)
	}
	if !strings.Contains(err.Error(), "detach cache") {
		t.Errorf("err = %v, want it to name the failing cleanup step", err)
	}
}

// TestWithAttachedCacheReportsCleanupOnSuccess covers "primary succeeded,
// cleanup failed": the work is done but the alias could not be released, which
// the caller must hear about because the next load will fail.
func TestWithAttachedCacheReportsCleanupOnSuccess(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	warmPool(t, db, 2)

	err := adapter.withAttachedCache(t.Context(), cachePath, func(ctx context.Context, conn *sql.Conn) error {
		if _, derr := conn.ExecContext(ctx, "DETACH DATABASE "+cacheAlias); derr != nil {
			t.Fatalf("precondition detach: %v", derr)
		}
		return nil
	})

	if err == nil {
		t.Fatal("err = nil, want the cleanup failure even though the work succeeded")
	}
	if !errors.Is(err, cleanup.ErrCleanup) {
		t.Errorf("errors.Is(err, ErrCleanup) = false; err = %v", err)
	}
}

// multiTableCache writes a cache holding two tables and returns its path.
func multiTableCache(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range map[string]string{
		"table_a.csv": "id,name\n1,alice\n2,bob\n",
		"table_b.csv": "id,city\n1,tokyo\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	sourceDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sourceDB.Close() })
	source := NewFileSQLAdapter(sourceDB)
	if err := source.LoadFiles(t.Context(),
		filepath.Join(dir, "table_a.csv"), filepath.Join(dir, "table_b.csv")); err != nil {
		t.Fatalf("seed import: %v", err)
	}

	cachePath := filepath.Join(dir, "cache.db")
	if err := source.SnapshotToCache(t.Context(), cachePath); err != nil {
		t.Fatalf("SnapshotToCache: %v", err)
	}
	return cachePath
}

// newSessionDB returns an empty file-backed session with a multi-connection pool.
func newSessionDB(t *testing.T) (*FileSQLAdapter, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "session.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	return NewFileSQLAdapter(db), db
}

// TestLoadFromCacheIsAtomicAcrossTables is the partial-restore regression.
//
// The session already holds a table_b that the cache's DDL cannot recreate, so
// the copy fails on the second table. Before the copy ran in a transaction,
// table_a was already created and stayed behind; the caller then fell back to a
// cold import, which collided with the leftover. Either both tables arrive or
// neither does.
func TestLoadFromCacheIsAtomicAcrossTables(t *testing.T) {
	t.Parallel()

	cachePath := multiTableCache(t)
	adapter, db := newSessionDB(t)
	warmPool(t, db, 3)

	// A conflicting table_b, so recreating it from the cache DDL fails.
	if _, err := db.ExecContext(t.Context(), `CREATE TABLE table_b (totally, different, shape)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO table_b VALUES ('x','y','z')`); err != nil {
		t.Fatal(err)
	}

	err := adapter.LoadFromCache(t.Context(), cachePath)
	if err == nil {
		t.Fatal("LoadFromCache = nil, want the second table to fail")
	}
	if !strings.Contains(err.Error(), "table_b") {
		t.Errorf("err = %v, want it to name the table that failed", err)
	}

	tables := sessionTables(t, db)
	if tables["table_a"] {
		t.Error("table_a survived a failed cache load; the copy was not atomic")
	}
	if !tables["table_b"] {
		t.Error("the pre-existing table_b was removed")
	}
	// The pre-existing table is untouched, not emptied or rewritten.
	var v string
	if err := db.QueryRowContext(t.Context(), `SELECT totally FROM table_b`).Scan(&v); err != nil {
		t.Fatalf("pre-existing table_b unreadable: %v", err)
	}
	if v != "x" {
		t.Errorf("table_b row = %q, want %q", v, "x")
	}

	assertNoConnectionHoldsAlias(t, db, 3)

	// The failure is recoverable: with the conflict removed, the same cache loads.
	if _, err := db.ExecContext(t.Context(), `DROP TABLE table_b`); err != nil {
		t.Fatal(err)
	}
	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after removing the conflict: %v", err)
	}
	tables = sessionTables(t, db)
	if !tables["table_a"] || !tables["table_b"] {
		t.Errorf("tables after the retry = %v, want both", tables)
	}
}

// TestLoadFromCacheCancelledMidCopyLeavesNothing checks the same atomicity under
// cancellation rather than a SQL error, and that the result reports the
// cancellation rather than a manufactured cleanup failure.
func TestLoadFromCacheCancelledMidCopyLeavesNothing(t *testing.T) {
	t.Parallel()

	cachePath := multiTableCache(t)
	adapter, db := newSessionDB(t)
	warmPool(t, db, 3)

	ctx, cancel := context.WithCancel(t.Context())
	cancelled := false
	err := adapter.withAttachedCache(ctx, cachePath, func(innerCtx context.Context, conn *sql.Conn) error {
		// Create the first table inside the same transaction shape the real copy
		// uses, then cancel before the second one.
		tx, err := conn.BeginTx(innerCtx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(innerCtx, `CREATE TABLE table_a (id, name)`); err != nil {
			_ = tx.Rollback()
			return err
		}
		cancel()
		cancelled = true
		// The context is dead, so the commit fails and the transaction is
		// discarded along with table_a.
		if commitErr := tx.Commit(); commitErr != nil {
			return commitErr
		}
		return nil
	})

	if !cancelled {
		t.Fatal("precondition: the callback did not run")
	}
	if err == nil {
		t.Fatal("err = nil, want the cancellation")
	}
	if errors.Is(err, cleanup.ErrCleanup) {
		t.Errorf("cancellation was reported as a cleanup failure: %v", err)
	}
	if tables := sessionTables(t, db); tables["table_a"] {
		t.Error("table_a survived a cancelled copy")
	}
	// The alias was released despite the dead context.
	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after a cancelled copy: %v", err)
	}
}

// TestLoadFromCacheCommitFailureLeavesNothing drives a real commit failure with
// a deferred foreign key, so the copy transaction fails at COMMIT rather than
// during a statement. The commit error must be what the caller sees, no
// rollback may follow it, and no table may survive.
func TestLoadFromCacheCommitFailureLeavesNothing(t *testing.T) {
	t.Parallel()

	cachePath := multiTableCache(t)
	adapter, db := newSessionDB(t)
	// One connection, so the PRAGMA and the transaction share a session.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, stmt := range []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE parent (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE child (id INTEGER PRIMARY KEY,
		     pid INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED)`,
	} {
		if _, err := db.ExecContext(t.Context(), stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	err := adapter.withAttachedCache(t.Context(), cachePath, func(ctx context.Context, conn *sql.Conn) error {
		_, txErr := infra.WithTransaction(ctx, infra.SQLConnTxBeginner{Conn: conn}, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `CREATE TABLE table_a (id, name)`); err != nil {
				return err
			}
			// Valid until COMMIT checks the deferred constraint.
			_, err := tx.ExecContext(ctx, `INSERT INTO child (id, pid) VALUES (1, 999)`)
			return err
		})
		return txErr
	})

	if err == nil {
		t.Fatal("err = nil, want the commit failure")
	}
	if !strings.Contains(err.Error(), "commit transaction") {
		t.Errorf("err = %v, want it to name the commit", err)
	}
	if errors.Is(err, sql.ErrTxDone) {
		t.Errorf("err carries a manufactured sql.ErrTxDone: %v", err)
	}
	if tables := sessionTables(t, db); tables["table_a"] {
		t.Error("table_a survived a failed commit")
	}
	// The alias was still released.
	var n int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM child`).Scan(&n); err != nil {
		t.Fatalf("session unusable after the failed commit: %v", err)
	}
	if n != 0 {
		t.Errorf("child rows = %d, want 0", n)
	}
}

// TestCacheCleanupErrorMatrix is the combination table for the cache lifecycle.
// Each row is a way the primary work and the cleanup can each succeed or fail,
// and what the caller must be able to see afterwards. The rule this pins is
// that neither error ever displaces the other, and that no error is invented:
// a commit is not followed by a rollback, and a cancellation is not reported as
// a broken transaction.
func TestCacheCleanupErrorMatrix(t *testing.T) {
	t.Parallel()

	workErr := errors.New("copy failed")

	tests := []struct {
		name string
		// missingCache makes the ATTACH itself fail.
		missingCache bool
		// detachEarly makes the deferred DETACH fail, by releasing the alias
		// from inside the callback.
		detachEarly bool
		// cancelInside cancels the operation's context inside the callback.
		cancelInside bool
		// commitFailure runs the work as a transaction whose COMMIT fails.
		commitFailure bool
		// workFails returns workErr from the callback.
		workFails bool

		wantNil     bool
		wantErrs    []error
		wantNotErrs []error
		wantMessage string
	}{
		{
			name:    "work succeeds, cleanup succeeds",
			wantNil: true,
		},
		{
			name:        "work fails, cleanup succeeds",
			workFails:   true,
			wantErrs:    []error{workErr},
			wantNotErrs: []error{cleanup.ErrCleanup},
		},
		{
			name:        "work succeeds, cleanup fails",
			detachEarly: true,
			wantErrs:    []error{cleanup.ErrCleanup},
			wantMessage: "detach cache",
		},
		{
			name:        "work fails and cleanup fails",
			workFails:   true,
			detachEarly: true,
			wantErrs:    []error{workErr, cleanup.ErrCleanup},
			wantMessage: "detach cache",
		},
		{
			// The dead context must not turn the detach into a reported failure,
			// and the cancellation is what the caller needs to see.
			name:         "cancellation is reported as cancellation",
			cancelInside: true,
			workFails:    true,
			wantErrs:     []error{workErr},
			wantNotErrs:  []error{cleanup.ErrCleanup, sql.ErrTxDone},
		},
		{
			// Commit ends the transaction, so no rollback follows and no
			// sql.ErrTxDone is manufactured on top of the real failure.
			name:          "commit failure stands alone",
			commitFailure: true,
			wantNotErrs:   []error{sql.ErrTxDone, cleanup.ErrCleanup},
			wantMessage:   "commit transaction",
		},
		{
			// Nothing was attached, so nothing is detached and no cleanup is
			// claimed. Note ATTACH creates a missing file, so the path has to be
			// one SQLite cannot open at all rather than merely absent.
			name:         "attach failure claims no cleanup",
			missingCache: true,
			wantNotErrs:  []error{cleanup.ErrCleanup},
			wantMessage:  "attach cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cachePath, adapter, db := newCacheFixture(t)
			if tt.commitFailure {
				// A deferred foreign key makes COMMIT the failing step.
				db.SetMaxOpenConns(1)
				db.SetMaxIdleConns(1)
				for _, stmt := range []string{
					`PRAGMA foreign_keys = ON`,
					`CREATE TABLE parent (id INTEGER PRIMARY KEY)`,
					`CREATE TABLE child (id INTEGER PRIMARY KEY,
					     pid INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED)`,
				} {
					if _, err := db.ExecContext(t.Context(), stmt); err != nil {
						t.Fatalf("setup %q: %v", stmt, err)
					}
				}
			} else {
				warmPool(t, db, 2)
			}
			if tt.missingCache {
				// A path whose parent directory does not exist: SQLite creates a
				// missing database file, but it cannot create the directory.
				cachePath = filepath.Join(t.TempDir(), "no-such-dir", "cache.db")
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			err := adapter.withAttachedCache(ctx, cachePath, func(inner context.Context, conn *sql.Conn) error {
				if tt.detachEarly {
					if _, derr := conn.ExecContext(inner, "DETACH DATABASE "+cacheAlias); derr != nil {
						t.Fatalf("precondition detach: %v", derr)
					}
				}
				if tt.commitFailure {
					_, txErr := infra.WithTransaction(inner, infra.SQLConnTxBeginner{Conn: conn},
						func(tx *sql.Tx) error {
							_, execErr := tx.ExecContext(inner, `INSERT INTO child (id, pid) VALUES (1, 999)`)
							return execErr
						})
					return txErr
				}
				if tt.cancelInside {
					cancel()
				}
				if tt.workFails {
					return workErr
				}
				return nil
			})

			if tt.wantNil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
			} else if err == nil {
				t.Fatalf("err = nil, want a failure")
			}
			for _, want := range tt.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("errors.Is(err, %v) = false; err = %v", want, err)
				}
			}
			for _, unwanted := range tt.wantNotErrs {
				if errors.Is(err, unwanted) {
					t.Errorf("errors.Is(err, %v) = true, want false; err = %v", unwanted, err)
				}
			}
			if tt.wantMessage != "" && !strings.Contains(err.Error(), tt.wantMessage) {
				t.Errorf("err = %v, want it to mention %q", err, tt.wantMessage)
			}

			// Whatever happened, the alias must be free: the next load works.
			if !tt.commitFailure {
				assertNoConnectionHoldsAlias(t, db, 2)
			}
		})
	}
}
