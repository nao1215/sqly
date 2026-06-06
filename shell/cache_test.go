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
	// Two rows with single-character values so the file can be edited later
	// without changing its byte length (keeping the cache signature identical).
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
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("expected cache file at %s: %v", cache, err)
	}

	// Edit the source in place, keeping the same byte length, then restore the
	// modification time so the cache signature still matches. A warm run must
	// return the cached value 'a', proving it did not re-read the source.
	if err := os.WriteFile(src, []byte("id,v\n1,Z\n2,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(src, origMTime, origMTime); err != nil {
		t.Fatal(err)
	}

	warm := runQuery(t, []string{"sqly", "--cache", cache, "--sql", "SELECT v FROM data WHERE id=1", src})
	if !strings.Contains(warm, "a") || strings.Contains(warm, "Z") {
		t.Errorf("warm run = %q, want cached value a (not the edited Z)", warm)
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

	sigs, err := collectCacheSignatures([]string{dir})
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

	again, err := collectCacheSignatures([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if !cacheSignaturesMatch(sigs, again) {
		t.Error("signatures of an unchanged directory should match")
	}
}
