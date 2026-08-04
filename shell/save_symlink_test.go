package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// `.save --in-place` overwrites the file it read. A symlink makes "the file it
// read" two different answers — the link and what the link points at — and the
// difference only shows up on the write. sqly follows the link, which is what
// makes it worth refusing by default: following it writes through to a path the
// user never named, which may sit outside the directory they are working in and
// may be shared with something else.
//
// The refusal is the default and not the rule; --follow-symlinks is how a user
// who meant it says so. These tests pin both halves, because a guard with no way
// past it would just move the problem to a copy step the user does by hand.
//
// None of them call t.Parallel: runScriptStreams swaps config.Stdout and
// config.Stderr, which are process-wide.

func TestSaveInPlace_RefusesASymlinkedSourceByDefault(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	target := writeCSV(t, dir, "real.csv", "name,age\nken,40\n")
	link := filepath.Join(dir, "link.csv")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create the symlink: %v", err)
	}

	_, stderr, err := runScriptStreams(t, "UPDATE link SET age = 41;\n.save --in-place\n", link)
	if err == nil {
		t.Fatal("an in-place save through a symlink was accepted")
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("the refusal should say the source is a symlink, got: %s", stderr)
	}
	// The point of refusing is that nothing was written. A guard that reports an
	// error after the write has landed protects nothing.
	if got := readFile(t, target); got != "name,age\nken,40\n" {
		t.Errorf("the target file was modified despite the refusal: %q", got)
	}
}

func TestSaveInPlace_FollowsASymlinkWhenAskedTo(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	target := writeCSV(t, dir, "real.csv", "name,age\nken,40\n")
	link := filepath.Join(dir, "link.csv")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create the symlink: %v", err)
	}

	_, stderr, err := runScriptStreams(t, "UPDATE link SET age = 41;\n.save --in-place --follow-symlinks\n", link)
	if err != nil {
		t.Fatalf("an explicit --follow-symlinks save failed: %v (%s)", err, stderr)
	}
	if got := readFile(t, target); !strings.Contains(got, "41") {
		t.Errorf("the target file was not updated: %q", got)
	}
	// The link must still be a link afterwards. A rename replaces the name, not
	// the file behind it, so a write that forgets to resolve first turns the
	// symlink into a regular file and leaves the target one holding the old rows.
	info, lerr := os.Lstat(link)
	if lerr != nil {
		t.Fatalf("lstat the link: %v", lerr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a regular file")
	}
	// A user who asks sqly to write through a link is told where that write went,
	// because the whole reason the default refuses is that the destination is not
	// the path they typed.
	//
	// The expected path is resolved the same way sqly resolves it. On Windows
	// t.TempDir can hand back an 8.3 short path (C:\Users\RUNNER~1\...) while
	// EvalSymlinks returns the long form, so comparing against the raw path fails
	// on a difference that has nothing to do with what is being tested.
	wantPath := target
	if resolved, rerr := filepath.EvalSymlinks(target); rerr == nil {
		wantPath = resolved
	}
	if !strings.Contains(stderr, wantPath) {
		t.Errorf("stderr should name the resolved target %q, got: %s", wantPath, stderr)
	}
}

// TestSaveInPlace_AllowsAPlainFile keeps the guard from costing the ordinary
// case: the overwhelming majority of sources are not symlinks and must not have
// to opt into anything.
func TestSaveInPlace_AllowsAPlainFile(t *testing.T) {
	dir := t.TempDir()
	plain := writeCSV(t, dir, "plain.csv", "name,age\nken,40\n")

	_, stderr, err := runScriptStreams(t, "UPDATE plain SET age = 41;\n.save --in-place\n", plain)
	if err != nil {
		t.Fatalf("an ordinary in-place save failed: %v (%s)", err, stderr)
	}
	if got := readFile(t, plain); !strings.Contains(got, "41") {
		t.Errorf("the file was not updated: %q", got)
	}
}

// TestSaveDir_IsUnaffectedBySymlinkPolicy scopes the guard. `.save DIR` writes
// somewhere else and leaves every source alone, so a symlinked source is not a
// hazard there and must not be refused.
func TestSaveDir_IsUnaffectedBySymlinkPolicy(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	target := writeCSV(t, dir, "real.csv", "name,age\nken,40\n")
	link := filepath.Join(dir, "link.csv")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create the symlink: %v", err)
	}
	destDir := filepath.Join(dir, "out")

	_, stderr, err := runScriptStreams(t, "UPDATE link SET age = 41;\n.save "+destDir+"\n", link)
	if err != nil {
		t.Fatalf(".save DIR with a symlinked source failed: %v (%s)", err, stderr)
	}
	if got := readFile(t, target); got != "name,age\nken,40\n" {
		t.Errorf(".save DIR modified the source: %q", got)
	}
	if _, serr := os.Stat(filepath.Join(destDir, "link.csv")); serr != nil {
		t.Errorf("the export was not written: %v", serr)
	}
}

// TestSave_RejectsFollowSymlinksWithADirectory keeps the option attached to the
// operation it modifies. `.save DIR --follow-symlinks` reads as though it
// changes something about the export, and it does not.
func TestSave_RejectsFollowSymlinksWithADirectory(t *testing.T) {
	dir := t.TempDir()
	plain := writeCSV(t, dir, "plain.csv", "name,age\nken,40\n")
	destDir := filepath.Join(dir, "out")

	_, stderr, err := runScriptStreams(t, "UPDATE plain SET age = 41;\n.save "+destDir+" --follow-symlinks\n", plain)
	if err == nil {
		t.Fatal(".save DIR --follow-symlinks was accepted")
	}
	if !strings.Contains(stderr, "--follow-symlinks") {
		t.Errorf("the error should name the misplaced option, got: %s", stderr)
	}
}

// requireSymlinks skips a test where symlinks cannot be created. Unprivileged
// Windows accounts without developer mode cannot make them, and a skip there is
// honest where a failure would only report the runner's configuration.
func requireSymlinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "a"), filepath.Join(dir, "b")); err != nil {
		t.Skipf("this account cannot create symlinks: %v", err)
	}
}
