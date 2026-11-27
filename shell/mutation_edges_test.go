package shell

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestInspectColumnsReadsNullabilityAndPrimaryKey checks the two flags the
// --inspect report derives from PRAGMA table_info, on a table that has each.
func TestInspectColumnsReadsNullabilityAndPrimaryKey(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := getExecStdOutput(t, s.exec, "CREATE TABLE k (id INTEGER PRIMARY KEY, name TEXT NOT NULL, note TEXT)"); err != nil {
		t.Fatal(err)
	}

	got, err := s.inspectColumns(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	want := []inspectColumn{
		{Name: "id", Type: "INTEGER", Nullable: true, PrimaryKey: true},
		{Name: "name", Type: "TEXT", Nullable: false, PrimaryKey: false},
		{Name: "note", Type: "TEXT", Nullable: true, PrimaryKey: false},
	}
	if !slices.Equal(got, want) {
		t.Errorf("inspectColumns(k) = %+v, want %+v", got, want)
	}
}

// TestSplitArgsKeepsABackslashAtTheEnd checks a backslash with nothing after
// it: it is kept as typed, and inside an open quote the quote is still
// reported as unterminated.
func TestSplitArgsKeepsABackslashAtTheEnd(t *testing.T) {
	t.Parallel()

	got, err := splitArgs(`.cd C:\`)
	if err != nil || !slices.Equal(got, []string{".cd", `C:\`}) {
		t.Errorf("splitArgs(`.cd C:\\`) = (%q, %v), want [.cd C:\\]", got, err)
	}
	if _, err := splitArgs(`.import "a\`); err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("splitArgs with an open quote ending in a backslash = %v, want an unterminated quote error", err)
	}
}

// TestSaveTakesFollowSymlinksBeforeInPlace checks that the flag is recognized
// in either order.
func TestSaveTakesFollowSymlinksBeforeInPlace(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	target := writeCSV(t, dir, "real.csv", "name,age\nken,40\n")
	link := filepath.Join(dir, "link.csv")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create the symlink: %v", err)
	}

	_, stderr, err := runScriptStreams(t, "UPDATE link SET age = 41;\n.save --follow-symlinks --in-place\n", link)
	if err != nil {
		t.Fatalf(".save --follow-symlinks --in-place failed: %v (%s)", err, stderr)
	}
	if got := readFile(t, target); !strings.Contains(got, "41") {
		t.Errorf("the target file was not updated: %q", got)
	}
}

// TestSaveRefusesABlankDirectory checks that a destination of only spaces is
// refused rather than taken for a directory named by spaces.
func TestSaveRefusesABlankDirectory(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	err = s.commands.saveCommand(context.Background(), s, []string{"  "})
	if err == nil || !strings.Contains(err.Error(), "non-empty directory") {
		t.Fatalf(`.save "  " = %v, want a refusal asking for a non-empty directory`, err)
	}
}

// TestSameFilePathSeesThroughALink checks that a symlink and its target are
// one file, and that a path that does not exist is the same as no other.
func TestSameFilePathSeesThroughALink(t *testing.T) {
	requireSymlinks(t)
	t.Parallel()

	dir := t.TempDir()
	target := writeCSV(t, dir, "real.csv", "id\n1\n")
	link := filepath.Join(dir, "link.csv")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.csv")
	if !sameFilePath(target, link) {
		t.Error("a symlink and its target read as two files")
	}
	if sameFilePath(target, missing) || sameFilePath(missing, target) {
		t.Error("a missing path reads as the same file as an existing one")
	}
	if sameFilePath(missing, filepath.Join(dir, "other.csv")) {
		t.Error("two different missing paths read as the same file")
	}
}
