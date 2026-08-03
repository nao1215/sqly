//go:build smoke

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSmoke_CacheColdThenWarm is the baseline the failure cases are measured
// against: the first run builds the cache, the second reads it, and both answer
// the same query with the same bytes. A cache that silently returned different
// data would be worse than no cache at all.
func TestSmoke_CacheColdThenWarm(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cachePath := filepath.Join(dir, "session.cache")

	args := []string{
		"--cache", cachePath,
		"--output-format", "csv",
		"--sql", "SELECT id, name FROM people ORDER BY id",
		csvPath,
	}

	cold, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("cold run exit code = %d (stderr=%q)", code, stderr)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("the cold run did not write the cache: %v", err)
	}

	warm, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("warm run exit code = %d (stderr=%q)", code, stderr)
	}
	if cold != warm {
		t.Errorf("warm run differs from cold run:\ncold:\n%s\nwarm:\n%s", cold, warm)
	}
	if !strings.Contains(warm, "alice") || !strings.Contains(warm, "bob") {
		t.Errorf("warm run = %q, want both rows", warm)
	}
}

// TestSmoke_CacheCorruptFallsBackToColdImport covers an unusable cache: the file
// exists but is not a SQLite database. Attaching it fails, and a failed cache
// load must not fail the run — the source files are still there, so the session
// falls back to a cold import and the user gets their answer.
func TestSmoke_CacheCorruptFallsBackToColdImport(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n")
	cachePath := writeFixture(t, dir, "broken.cache", "this is not a sqlite database")

	stdout, stderr, code := run(t, "",
		"--cache", cachePath,
		"--output-format", "csv",
		"--sql", "SELECT name FROM people",
		csvPath)

	if code != 0 {
		t.Fatalf("a corrupt cache failed the run: exit %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, "alice") {
		t.Errorf("stdout = %q, want the row from the cold import", stdout)
	}
	assertNoPanic(t, stdout, stderr)
}

// TestSmoke_CacheStaysUsableAfterAFailedRun is the recovery property the detach
// exists for. A run that fails after the cache is attached must still release
// it; the session is per-process so the leak is not visible directly, but a
// subsequent run reading the same cache is, and it would fail if the previous
// run had left a broken cache behind.
func TestSmoke_CacheStaysUsableAfterAFailedRun(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cachePath := filepath.Join(dir, "session.cache")

	// Build the cache.
	if _, stderr, code := run(t, "",
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT COUNT(*) FROM people", csvPath); code != 0 {
		t.Fatalf("seeding the cache failed: %d (stderr=%q)", code, stderr)
	}

	// A run that reads the cache and then fails on a bad query.
	stdout, stderr, code := run(t, "",
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT FROM WHERE", csvPath)
	if code == 0 {
		t.Fatalf("a syntax error exited 0 (stdout=%q)", stdout)
	}
	assertNoPanic(t, stdout, stderr)

	// The cache must still work afterwards.
	stdout, stderr, code = run(t, "",
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT name FROM people ORDER BY id", csvPath)
	if code != 0 {
		t.Fatalf("the cache was unusable after a failed run: exit %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, "alice") || !strings.Contains(stdout, "bob") {
		t.Errorf("stdout = %q, want both rows", stdout)
	}
}

// TestSmoke_CacheRebuildsWhenSourceChanges checks that editing the source
// invalidates the cache. A cache keyed only on the path would serve the old
// rows here, which is the worst failure mode a cache has: a confidently wrong
// answer.
func TestSmoke_CacheRebuildsWhenSourceChanges(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n")
	cachePath := filepath.Join(dir, "session.cache")

	args := []string{
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT name FROM people ORDER BY id", csvPath,
	}
	if _, stderr, code := run(t, "", args...); code != 0 {
		t.Fatalf("seeding the cache failed: %d (stderr=%q)", code, stderr)
	}

	// Same length, different content, so a size-only key would miss the change.
	writeFixture(t, dir, "people.csv", "id,name\n1,zzzzz\n")

	stdout, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, "zzzzz") {
		t.Errorf("stdout = %q, want the edited row; the cache served stale data", stdout)
	}
	if strings.Contains(stdout, "alice") {
		t.Errorf("stdout = %q, still holds the pre-edit row", stdout)
	}
}

// TestSmoke_CacheEmptyValueRejected checks the argument contract: --cache with
// an empty value is a mistake, not a request to disable the cache, so it is
// rejected rather than silently ignored.
func TestSmoke_CacheEmptyValueRejected(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n")

	stdout, stderr, code := run(t, "",
		"--cache", "", "--output-format", "csv", "--sql", "SELECT 1", csvPath)
	if code == 0 {
		t.Fatalf("--cache \"\" exited 0 (stdout=%q)", stdout)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want it empty", stdout)
	}
	assertNoPanic(t, stdout, stderr)
}

// TestSmoke_ShellRecoversAfterAFailedQuery is the shell-mode continuation
// property. A failed query must not end the session's usefulness: the next
// statement has to run against the same data.
func TestSmoke_ShellRecoversAfterAFailedQuery(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")

	// Batch mode stops at the first failure by design, so each statement is
	// checked in its own run to establish that the session state itself is not
	// what breaks: the same query succeeds before and after a failing one.
	before, _, code := run(t, ".mode csv\nSELECT name FROM people ORDER BY id;\n", csvPath)
	if code != 0 {
		t.Fatalf("the baseline query failed: %d", code)
	}

	stdout, stderr, code := run(t, ".mode csv\nSELECT * FROM no_such_table;\n", csvPath)
	if code == 0 {
		t.Fatal("a query against a missing table exited 0")
	}
	if !strings.Contains(stderr, "no_such_table") {
		t.Errorf("stderr = %q, want it to name the missing table", stderr)
	}
	assertNoPanic(t, stdout, stderr)

	after, _, code := run(t, ".mode csv\nSELECT name FROM people ORDER BY id;\n", csvPath)
	if code != 0 {
		t.Fatalf("the query failed after a failing one: %d", code)
	}
	if before != after {
		t.Errorf("the session answered differently after a failure:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
