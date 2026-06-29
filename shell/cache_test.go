package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runQuery builds a shell from args and returns its stdout.
func runQuery(t *testing.T, args []string) string {
	t.Helper()
	shell, cleanup, err := newShell(t, args)
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()
	return captureStdout(t, func() {
		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

func TestCache_WarmHitReusesSnapshot(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	cache := filepath.Join(dir, "snap.cache")
	if err := os.WriteFile(src, []byte("id,v\n1,a\n2,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Cold run populates the cache.
	cold := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT v FROM data WHERE id=1", src})
	if !strings.Contains(cold, "a") {
		t.Fatalf("cold run = %q, want value a", cold)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("expected cache file at %s: %v", cache, err)
	}

	// A second run over the unchanged source must reuse the cache and emit the
	// reuse banner on stderr.
	warmErr := captureStderr(t, func() {
		warm := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT v FROM data WHERE id=1", src})
		if !strings.Contains(warm, "a") {
			t.Errorf("warm run = %q, want value a", warm)
		}
	})
	if !strings.Contains(warmErr, "cache: reused") {
		t.Errorf("warm run stderr = %q, want it to include 'cache: reused'", warmErr)
	}
}

// TestCache_InsideImportedDirectory verifies that when the cache path lives
// inside the directory being imported, sqly's own cache artifacts (the database
// and its manifest sidecar) are not treated as dataset inputs: the second run is
// still a warm hit and no manifest-derived table appears.
func TestCache_InsideImportedDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	cache := filepath.Join(dir, "snap.cache") // cache sidecar lands inside the imported dir
	if err := os.WriteFile(src, []byte("id,name\n1,Alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Cold run imports the directory and writes the cache and manifest inside it.
	runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT COUNT(*) AS n FROM data", dir})

	// The second run over the same directory must reuse the cache, and the cache
	// manifest must not have been imported as an extra table.
	var tables string
	warmErr := captureStderr(t, func() {
		tables = runQuery(t, []string{"sqly", "--cache", cache, "--sql",
			"SELECT group_concat(name, ',') AS t FROM sqlite_master WHERE type='table' ORDER BY name", dir})
	})
	if !strings.Contains(warmErr, "cache: reused") {
		t.Errorf("second run should reuse the cache, stderr = %q", warmErr)
	}
	if strings.Contains(tables, "manifest") || strings.Contains(tables, "snap") {
		t.Errorf("cache artifact was imported as a table: %q", tables)
	}
}

// TestCache_InvalidatesWhenContentChangesSameSizeAndMTime reproduces issue #592:
// a source rewritten with different but same-length content and its original
// mtime restored must invalidate the cache, not reuse stale data.
func TestCache_InvalidatesWhenContentChangesSameSizeAndMTime(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	cache := filepath.Join(dir, "snap.cache")
	// Single-character values so the edit keeps the file's byte length identical.
	if err := os.WriteFile(src, []byte("id,v\n1,a\n2,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	origMTime := info.ModTime()

	// Cold run populates the cache.
	cold := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT v FROM data WHERE id=1", src})
	if !strings.Contains(cold, "a") {
		t.Fatalf("cold run = %q, want value a", cold)
	}

	// Rewrite with same length and restore the mtime: size and mtime are
	// unchanged, so only a content hash can detect the edit.
	if err := os.WriteFile(src, []byte("id,v\n1,Z\n2,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(src, origMTime, origMTime); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		got := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT v FROM data WHERE id=1", src})
		if !strings.Contains(got, "Z") {
			t.Errorf("after content change = %q, want the new value Z (cache must invalidate)", got)
		}
	})
	if strings.Contains(stderr, "cache: reused") {
		t.Errorf("stderr = %q, want no 'cache: reused' after a content change", stderr)
	}
}

func TestCache_InvalidatesWhenSourceChanges(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	cache := filepath.Join(dir, "snap.cache")
	if err := os.WriteFile(src, []byte("id\n1\n2\n3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cold := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT COUNT(*) AS n FROM data", src})
	if !strings.Contains(cold, "3") {
		t.Fatalf("cold run count = %q, want 3", cold)
	}

	// Append a row: size and mtime change, so the cache must be rebuilt.
	time.Sleep(10 * time.Millisecond) // ensure a distinct mtime on coarse clocks
	if err := os.WriteFile(src, []byte("id\n1\n2\n3\n4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT COUNT(*) AS n FROM data", src})
	if !strings.Contains(got, "4") {
		t.Errorf("after source change count = %q, want 4 (cache should have invalidated)", got)
	}
}

func TestCache_FailureFallsBackToColdImport(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(src, []byte("id\n1\n2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make the cache path a non-empty directory so the snapshot write cannot
	// succeed (it can neither be removed nor overwritten as a SQLite file).
	cache := filepath.Join(dir, "cache_is_a_dir")
	if err := os.Mkdir(cache, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The query must still succeed even though the cache cannot be written.
	got := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT COUNT(*) AS n FROM data", src})
	if !strings.Contains(got, "2") {
		t.Errorf("count = %q, want 2 despite an unwritable cache", got)
	}
}

func TestCache_ClearForcesRebuild(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	cache := filepath.Join(dir, "snap.cache")
	if err := os.WriteFile(src, []byte("id\n1\n2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Cold run writes the cache.
	_ = runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT COUNT(*) AS n FROM data", src})
	manifest := cacheManifestPath(cache)
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("expected manifest after cold run: %v", err)
	}

	// --cache-clear deletes the existing cache, then the run rebuilds it.
	got := runQuery(t, []string{"sqly", "--cache", cache, "--cache-clear", "--sql", "SELECT COUNT(*) AS n FROM data", src})
	if !strings.Contains(got, "2") {
		t.Errorf("count = %q, want 2 after --cache-clear", got)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Errorf("expected the cache to be rebuilt after --cache-clear: %v", err)
	}
}

func TestContainsInputOnlyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plain := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(plain, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if containsInputOnlyFile(plain) {
		t.Error("a plain CSV must not be input-only")
	}

	// A directory containing an ACH file must be detected so caching is skipped.
	sub := filepath.Join(dir, "mixed")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	// The check is extension-based, so a stub .ach file is enough.
	if err := os.WriteFile(filepath.Join(sub, "pay.ach"), []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "ok.csv"), []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !containsInputOnlyFile(sub) {
		t.Error("a directory containing an .ach file must be reported as input-only")
	}
}

func TestCollectCacheSignatures_DirectoryAndChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.csv")
	b := filepath.Join(dir, "b.csv")
	if err := os.WriteFile(a, []byte("x\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("y\n2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	sigs, err := collectCacheSignatures([]string{dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 2 {
		t.Fatalf("expected 2 signatures for the directory, got %d", len(sigs))
	}
	// Sorted by path: a.csv before b.csv.
	if !strings.HasSuffix(sigs[0].Path, "a.csv") || !strings.HasSuffix(sigs[1].Path, "b.csv") {
		t.Errorf("signatures not sorted by path: %+v", sigs)
	}

	again, err := collectCacheSignatures([]string{dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cacheSignaturesMatch(sigs, again) {
		t.Error("signatures of an unchanged directory should match")
	}
}
