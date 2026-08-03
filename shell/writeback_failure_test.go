package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
)

// A write-back covering several files is all-or-nothing, and the interesting
// failures are the ones that happen part-way: after one file has been written
// and before the next one has. These tests drive those points directly, because
// an end-to-end run cannot choose which target fails.

// writeBackShell imports paths into a session and returns the shell, ready for
// a direct writeBack call. It bypasses Run so a test can arrange a failure that
// only appears once writing starts.
func writeBackShell(t *testing.T, args ...string) (*Shell, func()) {
	t.Helper()
	shell, cleanup, err := newShell(t, append([]string{"sqly"}, args...))
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	if err := shell.init(context.Background()); err != nil {
		cleanup()
		t.Fatalf("import: %v", err)
	}
	return shell, cleanup
}

// markChanged makes a table look modified without running SQL, so a test can
// reach the write path with a known set of dirty tables.
func markChanged(t *testing.T, s *Shell, tables ...string) {
	t.Helper()
	s.dataChanged = true
	for _, name := range tables {
		delete(s.importBaseline, name)
	}
}

// silenceStderr redirects the status stream for the duration of a test, so a
// save's confirmations do not pollute the test log.
func silenceStderr(t *testing.T) {
	t.Helper()
	backup := config.Stderr
	config.Stderr = &strings.Builder{}
	t.Cleanup(func() { config.Stderr = backup })
}

// leftoverFiles returns the names in dir that are not in expected. Staging files
// and backups are dot-prefixed, so this catches them as well as stray exports.
func leftoverFiles(t *testing.T, dir string, expected ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	want := make(map[string]bool, len(expected))
	for _, name := range expected {
		want[name] = true
	}
	var extra []string
	for _, e := range entries {
		if !want[e.Name()] {
			extra = append(extra, e.Name())
		}
	}
	return extra
}

// TestWriteBack_SerializeFailureLeavesEverySourceUntouched is the central
// guarantee: nothing is moved onto a source file until every target has been
// serialized. The second table's directory is made read-only, so staging it
// fails after the first one has already been written to its scratch file.
func TestWriteBack_SerializeFailureLeavesEverySourceUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not block file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permissions")
	}

	root := t.TempDir()
	firstDir := filepath.Join(root, "first")
	secondDir := filepath.Join(root, "second")
	for _, d := range []string{firstDir, secondDir} {
		if err := os.Mkdir(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	const firstContent = "id,name\n1,alice\n"
	const secondContent = "id,city\n1,tokyo\n"
	first := writeCSV(t, firstDir, "people.csv", firstContent)
	second := writeCSV(t, secondDir, "places.csv", secondContent)

	shell, cleanup := writeBackShell(t, first, second)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people", "places")

	// Staging happens in the destination's own directory, so a read-only
	// directory fails the second target while the first is already staged.
	//nolint:gosec // a directory needs its execute bit to stay traversable
	if err := os.Chmod(secondDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secondDir, 0o750) }) //nolint:gosec // restoring the directory mode t.TempDir created

	err := shell.writeBack(context.Background(), "")
	if err == nil {
		t.Fatal("writeBack succeeded although the second target could not be staged")
	}
	if !strings.Contains(err.Error(), second) {
		t.Errorf("error should name the target that failed, got: %v", err)
	}

	if got := readFile(t, first); got != firstContent {
		t.Errorf("the first source was replaced although the save failed:\n got %q\nwant %q", got, firstContent)
	}
	if got := readFile(t, second); got != secondContent {
		t.Errorf("the second source changed:\n got %q\nwant %q", got, secondContent)
	}
	if extra := leftoverFiles(t, firstDir, "people.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind in %s: %v", firstDir, extra)
	}
}

// TestWriteBack_CommitFailureRestoresTheCommittedFiles covers the other half of
// the window: staging succeeded for everything, and a move fails after an
// earlier one landed. The destination that was already replaced must be put back
// from its backup, and no backup or staging file may survive.
func TestWriteBack_CommitFailureRestoresTheCommittedFiles(t *testing.T) {
	dir := t.TempDir()
	const firstContent = "id,name\n1,alice\n"
	first := writeCSV(t, dir, "people.csv", firstContent)
	second := writeCSV(t, dir, "places.csv", "id,city\n1,tokyo\n")

	shell, cleanup := writeBackShell(t, first, second)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people", "places")

	targets, err := shell.planWriteBack(context.Background(), "", true)
	if err != nil {
		t.Fatalf("planWriteBack: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("planned %d targets, want 2", len(targets))
	}
	// Order the targets so people.csv commits first, then point the second target
	// at a path that cannot be committed to: a directory.
	if targets[0].table != "people" {
		targets[0], targets[1] = targets[1], targets[0]
	}
	blocked := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocked, 0o750); err != nil {
		t.Fatal(err)
	}
	targets[1].dest = blocked

	err = shell.executeWriteBack(context.Background(), "", targets)
	if err == nil {
		t.Fatal("executeWriteBack succeeded although a commit target was a directory")
	}

	if got := readFile(t, first); got != firstContent {
		t.Errorf("the committed file was not rolled back:\n got %q\nwant %q", got, firstContent)
	}
	if extra := leftoverFiles(t, dir, "people.csv", "places.csv", "blocked"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_InPlacePreservesSourcePermissions pins that saving a file does
// not change who can read it. The staged file is created 0600, so a rename that
// carried its own mode across would silently tighten a shared CSV.
func TestWriteBack_InPlacePreservesSourcePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}

	dir := t.TempDir()
	src := writeCSV(t, dir, "shared.csv", "id,name\n1,alice\n")
	const mode os.FileMode = 0o644
	if err := os.Chmod(src, mode); err != nil {
		t.Fatal(err)
	}

	shell, cleanup := writeBackShell(t, src)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "shared")

	if err := shell.writeBack(context.Background(), ""); err != nil {
		t.Fatalf("writeBack: %v", err)
	}

	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != mode {
		t.Errorf("source permissions changed: got %o, want %o", got, mode)
	}
	if extra := leftoverFiles(t, dir, "shared.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_DuplicateInputPathWritesOnce checks that naming the same file
// twice does not plan two writes to it. Two targets sharing a destination would
// mean the second overwrites what the first just committed, and the backup taken
// for the second would be the already-replaced file.
func TestWriteBack_DuplicateInputPathWritesOnce(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "twice.csv", "id,name\n1,alice\n")

	shell, cleanup := writeBackShell(t, src, src)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "twice")

	targets, err := shell.planWriteBack(context.Background(), "", true)
	if err != nil {
		t.Fatalf("planWriteBack: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("planned %d targets for one file named twice, want 1: %+v", len(targets), targets)
	}
	if err := shell.writeBack(context.Background(), ""); err != nil {
		t.Fatalf("writeBack: %v", err)
	}
	if extra := leftoverFiles(t, dir, "twice.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_OnlyChangedTablesAreWritten is what "every table the session
// changed" in --help has to mean. Two sources are imported and one is modified;
// the untouched one must keep its exact bytes, including the trailing newline it
// was written without.
func TestWriteBack_OnlyChangedTablesAreWritten(t *testing.T) {
	dir := t.TempDir()
	const untouchedContent = "id,city\n1,tokyo"
	changed := writeCSV(t, dir, "people.csv", "id,name\n1,alice\n")
	untouched := writeCSV(t, dir, "places.csv", untouchedContent)

	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "UPDATE people SET name = 'bob'", "--save-in-place", changed, untouched})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }
	silenceStderr(t)

	if runErr := shell.Run(context.Background()); runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}

	if got := readFile(t, changed); !strings.Contains(got, "bob") {
		t.Errorf("the changed table was not written back: %q", got)
	}
	if got := readFile(t, untouched); got != untouchedContent {
		t.Errorf("an unchanged table was rewritten:\n got %q\nwant %q", got, untouchedContent)
	}
}

// TestRollbackCommitted_RestoreFailureIsNotReportedOverTheCause documents the
// one place a failure is deliberately not surfaced. A rollback runs because
// something already failed; reporting a failed restore instead of that original
// error would hide the one the user has to act on, so rollbackCommitted is best
// effort and the caller's error is the one returned.
func TestRollbackCommitted_RestoreFailureIsNotReportedOverTheCause(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "gone.csv")

	// A backup path that does not exist makes the restore fail.
	rollbackCommitted([]stagedWrite{{
		target: writeTarget{dest: dest},
		backup: filepath.Join(dir, "missing.bak"),
	}})

	// The destination is left as it was (absent); the call itself does not panic
	// and returns nothing for the caller to mistake for success.
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat %s = %v, want it to stay absent", dest, err)
	}
}

// readFile returns a file's contents as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test-owned path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
