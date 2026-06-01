package shell

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnsureNotDirectory covers rejecting directory-like output destinations:
// an existing directory and a path ending with a path separator.
func TestEnsureNotDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	t.Run("existing directory is rejected", func(t *testing.T) {
		t.Parallel()
		if err := ensureNotDirectory(dir); err == nil {
			t.Errorf("want error for existing directory, got nil")
		}
	})

	t.Run("path ending with a separator is rejected", func(t *testing.T) {
		t.Parallel()
		if err := ensureNotDirectory(filepath.Join(dir, "outdir") + "/"); err == nil {
			t.Errorf("want error for trailing-separator path, got nil")
		}
	})

	t.Run("plain non-existent file path is accepted", func(t *testing.T) {
		t.Parallel()
		if err := ensureNotDirectory(filepath.Join(dir, "out.csv")); err != nil {
			t.Errorf("want nil for plain file path, got %v", err)
		}
	})
}

// TestSameFilePathSymlink verifies that a symlink alias to a file is recognized
// as the same file, so the overwrite guard cannot be bypassed.
func TestSameFilePathSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "user.csv")
	if err := os.WriteFile(src, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.csv")
	if err := os.Symlink(src, alias); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if !sameFilePath(alias, src) {
		t.Errorf("sameFilePath(symlink, target) = false, want true")
	}
	other := filepath.Join(dir, "other.csv")
	if err := os.WriteFile(other, []byte("b\n2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if sameFilePath(other, src) {
		t.Errorf("sameFilePath(unrelated, target) = true, want false")
	}
}
