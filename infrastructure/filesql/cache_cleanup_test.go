package filesql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/cleanup"
	_ "modernc.org/sqlite"
)

// newCacheFixture builds a session with one imported table and writes a cache
// snapshot of it, returning the cache path and a fresh empty session to load it
// into. Each caller gets its own temp directory and its own database.
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

	targetDB, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = targetDB.Close() })
	return cachePath, NewFileSQLAdapter(targetDB), targetDB
}

// attachedAliases returns the schema names currently attached to the connection
// pool, which is how a leaked ATTACH becomes observable.
func attachedAliases(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), "PRAGMA database_list")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var seq int
		var name string
		var file sql.NullString
		if err := rows.Scan(&seq, &name, &file); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return names
}

// TestLoadFromCacheDetachesOnSuccess is the baseline: a successful load leaves
// no alias attached, so the next load can attach the same name again.
func TestLoadFromCacheDetachesOnSuccess(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	// One connection, so an alias leaked by the first load is visible to the
	// second rather than hidden by the pool handing out a different connection.
	db.SetMaxOpenConns(1)

	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}
	for _, name := range attachedAliases(t, db) {
		if name == "sqly_cache" {
			t.Fatal("sqly_cache is still attached after a successful load")
		}
	}

	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM people").Scan(&n); err != nil {
		t.Fatalf("query the restored table: %v", err)
	}
	if n != 2 {
		t.Errorf("restored rows = %d, want 2", n)
	}
}

// TestLoadFromCacheAttachFailure covers the failure before anything is
// attached: the cache file does not exist, so the error names it and nothing is
// left behind for the next attempt to trip over.
func TestLoadFromCacheAttachFailure(t *testing.T) {
	t.Parallel()

	_, adapter, db := newCacheFixture(t)
	db.SetMaxOpenConns(1)
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
	for _, name := range attachedAliases(t, db) {
		if name == "sqly_cache" {
			t.Fatal("sqly_cache attached despite the failure")
		}
	}
}

// TestLoadFromCacheDetachesAfterAFailedLoad is the "primary failed, cleanup
// succeeded" case: the cache attaches but holds no tables, so the load fails.
// The alias must still be released, because leaving it attached would make the
// *next* load fail with an unrelated "already in use" error — a failure whose
// message would point at the wrong run.
func TestLoadFromCacheDetachesAfterAFailedLoad(t *testing.T) {
	t.Parallel()

	_, adapter, db := newCacheFixture(t)
	db.SetMaxOpenConns(1)

	// A valid SQLite database with no user tables.
	emptyPath := filepath.Join(t.TempDir(), "empty.db")
	emptyDB, err := sql.Open("sqlite", emptyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := emptyDB.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := emptyDB.ExecContext(t.Context(), "CREATE TABLE seed (a)"); err != nil {
		t.Fatal(err)
	}
	if _, err := emptyDB.ExecContext(t.Context(), "DROP TABLE seed"); err != nil {
		t.Fatal(err)
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
	for _, name := range attachedAliases(t, db) {
		if name == "sqly_cache" {
			t.Fatal("sqly_cache left attached after a failed load")
		}
	}
}

// TestLoadFromCacheRetriesAfterFailure is the recovery property the detach
// exists for: a failed load must not poison the session. The same alias has to
// be attachable again immediately afterwards.
func TestLoadFromCacheRetriesAfterFailure(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	db.SetMaxOpenConns(1)

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

// TestLoadFromCacheCancelledBeforeAttach checks the early exit: the context is
// already dead when ATTACH runs, so nothing is attached and no cleanup is
// claimed.
func TestLoadFromCacheCancelledBeforeAttach(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := adapter.LoadFromCache(ctx, cachePath)
	if err == nil {
		t.Fatal("LoadFromCache with a cancelled context = nil, want an error")
	}
	for _, name := range attachedAliases(t, db) {
		if name == cacheAlias {
			t.Fatalf("%s left attached after cancellation; err was %v", cacheAlias, err)
		}
	}
	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after a cancelled attempt: %v", err)
	}
}

// TestWithAttachedCacheDetachesAfterCancellation is the case cleanup.Context
// exists for, driven through the lifecycle helper so the detach is actually
// reached.
//
// Cancelling before LoadFromCache runs is not this case: ATTACH fails first and
// there is nothing to detach. The dangerous ordering is the attach succeeding
// and the context dying afterwards — a user pressing Ctrl-C, or a deadline
// expiring mid-copy. Detaching with that dead context fails immediately, so the
// alias stays held for the life of the process and the next cache load fails
// with "database sqly_cache is already in use", pointing at the wrong run.
func TestWithAttachedCacheDetachesAfterCancellation(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithCancel(t.Context())
	workErr := errors.New("interrupted mid-copy")

	// The alias is attached and live here; cancelling inside the callback is
	// what a Ctrl-C between the ATTACH and the copy looks like.
	attached := false
	err := adapter.withAttachedCache(ctx, cachePath, func(_ context.Context) error {
		for _, name := range attachedAliases(t, db) {
			if name == cacheAlias {
				attached = true
			}
		}
		cancel()
		return workErr
	})
	if !attached {
		t.Fatal("precondition: the cache was not attached inside the callback")
	}
	if !errors.Is(err, workErr) {
		t.Errorf("errors.Is(err, workErr) = false; err = %v", err)
	}
	if errors.Is(err, cleanup.ErrCleanup) {
		t.Errorf("the detach failed despite the cleanup context: %v", err)
	}
	for _, name := range attachedAliases(t, db) {
		if name == cacheAlias {
			t.Fatalf("%s left attached after the context was cancelled", cacheAlias)
		}
	}

	// The proof that it was released: the same alias attaches again.
	if err := adapter.LoadFromCache(t.Context(), cachePath); err != nil {
		t.Fatalf("LoadFromCache after a cancelled attach: %v", err)
	}
}

// TestWithAttachedCacheKeepsBothErrors is the "primary failed and cleanup
// failed" combination, which the previous rule reported as if only the primary
// had happened. Detaching an alias that is no longer attached fails, so the
// callback detaches it itself and then returns its own error.
func TestWithAttachedCacheKeepsBothErrors(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	db.SetMaxOpenConns(1)

	workErr := errors.New("copy failed")
	err := adapter.withAttachedCache(t.Context(), cachePath, func(ctx context.Context) error {
		// Pull the alias out from under the deferred detach, so the detach fails
		// for a reason unrelated to the work.
		if _, derr := db.ExecContext(ctx, "DETACH DATABASE "+cacheAlias); derr != nil {
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
	db.SetMaxOpenConns(1)

	err := adapter.withAttachedCache(t.Context(), cachePath, func(ctx context.Context) error {
		if _, derr := db.ExecContext(ctx, "DETACH DATABASE "+cacheAlias); derr != nil {
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

// TestLoadFromCacheReportsDetachFailureBesideTheCause is the "primary failed,
// cleanup failed" combination. Closing the pool under the load makes both the
// read and the detach fail; the previous rule assigned the detach error only
// when err was nil, so this is exactly the state that used to be reported as if
// only one thing had gone wrong.
func TestLoadFromCacheReportsDetachFailureBesideTheCause(t *testing.T) {
	t.Parallel()

	cachePath, adapter, db := newCacheFixture(t)
	db.SetMaxOpenConns(1)

	// Close the pool after the ATTACH has happened but before the schema read,
	// by racing it: closing first makes ATTACH itself fail, so instead attach,
	// then close, then let the deferred detach run.
	if _, err := db.ExecContext(t.Context(),
		"ATTACH DATABASE '"+escapeSQLiteLiteral(cachePath)+"' AS probe"); err != nil {
		t.Fatalf("probe attach: %v", err)
	}
	if _, err := db.ExecContext(t.Context(), "DETACH DATABASE probe"); err != nil {
		t.Fatalf("probe detach: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	err := adapter.LoadFromCache(t.Context(), cachePath)
	if err == nil {
		t.Fatal("LoadFromCache on a closed database = nil, want an error")
	}
	// The attach is what fails first here, so no cleanup is claimed. The point of
	// the assertion is that the caller is told about a real failure and not left
	// with a success.
	if !strings.Contains(err.Error(), "attach cache") {
		t.Errorf("err = %v, want it to name the failing step", err)
	}
}
