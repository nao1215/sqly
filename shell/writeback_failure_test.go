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

// failOnNth2 is failOnNth for the two-argument calls (Rename, Copy).
func failOnNth2[T, U any](n int, delegate func(T, U) error, err error) func(T, U) error {
	calls := 0
	return func(a T, b U) error {
		calls++
		if calls == n {
			return err
		}
		return delegate(a, b)
	}
}

// errInjected is the failure these tests inject. It is deliberately not an
// os.PathError: a real filesystem error would make it ambiguous whether the code
// path or the environment produced it.
var errInjected = errors.New("injected failure")

// TestWriteBack_SerializeFailureLeavesEverySourceUntouched is the central
// guarantee: nothing is moved onto a source file until every target has been
// serialized. Staging the second table is made to fail while the first is
// already written to its scratch file.
func TestWriteBack_SerializeFailureLeavesEverySourceUntouched(t *testing.T) {
	dir := t.TempDir()
	const firstContent = "id,name\n1,alice\n"
	const secondContent = "id,city\n1,tokyo\n"
	first := writeCSV(t, dir, "people.csv", firstContent)
	second := writeCSV(t, dir, "places.csv", secondContent)

	shell, cleanup := writeBackShell(t, first, second)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people", "places")

	// The second staging file cannot be created; the first already exists.
	ops := defaultFileOps()
	shell.files = ops
	calls := 0
	shell.files.CreateTemp = func(d, pattern string) (*os.File, error) {
		calls++
		if calls == 2 {
			return nil, errInjected
		}
		return ops.CreateTemp(d, pattern)
	}

	err := shell.writeBack(context.Background(), "")
	if err == nil {
		t.Fatal("writeBack succeeded although the second target could not be staged")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error should wrap the injected failure, got: %v", err)
	}

	if got := readFile(t, first); got != firstContent {
		t.Errorf("the first source was replaced although the save failed:\n got %q\nwant %q", got, firstContent)
	}
	if got := readFile(t, second); got != secondContent {
		t.Errorf("the second source changed:\n got %q\nwant %q", got, secondContent)
	}
	if extra := leftoverFiles(t, dir, "people.csv", "places.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_BackupFailureStopsBeforeAnyCommit checks the step between
// staging and committing: if the backup of an existing destination cannot be
// taken, the save must stop rather than replace a file it cannot put back.
func TestWriteBack_BackupFailureStopsBeforeAnyCommit(t *testing.T) {
	dir := t.TempDir()
	const content = "id,name\n1,alice\n"
	src := writeCSV(t, dir, "people.csv", content)

	shell, cleanup := writeBackShell(t, src)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people")

	ops := defaultFileOps()
	shell.files = ops
	// The first Copy is the backup; the commit's copy fallback comes later.
	shell.files.Copy = failOnNth2(1, ops.Copy, errInjected)

	if err := shell.writeBack(context.Background(), ""); err == nil {
		t.Fatal("writeBack succeeded although the backup could not be taken")
	}
	if got := readFile(t, src); got != content {
		t.Errorf("the source was replaced although the backup failed:\n got %q\nwant %q", got, content)
	}
	if extra := leftoverFiles(t, dir, "people.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_ChmodFailureStopsTheCommit checks that a save which cannot carry
// the destination's permissions onto the staged file stops instead of landing a
// file with the wrong mode.
func TestWriteBack_ChmodFailureStopsTheCommit(t *testing.T) {
	dir := t.TempDir()
	const content = "id,name\n1,alice\n"
	src := writeCSV(t, dir, "people.csv", content)

	shell, cleanup := writeBackShell(t, src)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people")

	ops := defaultFileOps()
	shell.files = ops
	shell.files.Chmod = func(string, os.FileMode) error { return errInjected }

	if err := shell.writeBack(context.Background(), ""); err == nil {
		t.Fatal("writeBack succeeded although the permissions could not be carried over")
	}
	if got := readFile(t, src); got != content {
		t.Errorf("the source was replaced although the commit failed:\n got %q\nwant %q", got, content)
	}
	if extra := leftoverFiles(t, dir, "people.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_RollbackFailureIsReportedWithTheCause is the case a user must
// never be left guessing about: the commit failed, and putting the earlier file
// back failed too, so a source now holds content from a save that reported an
// error. Both errors have to come out.
func TestWriteBack_RollbackFailureIsReportedWithTheCause(t *testing.T) {
	dir := t.TempDir()
	first := writeCSV(t, dir, "people.csv", "id,name\n1,alice\n")
	second := writeCSV(t, dir, "places.csv", "id,city\n1,tokyo\n")

	shell, cleanup := writeBackShell(t, first, second)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people", "places")

	ops := defaultFileOps()
	shell.files = ops
	// The second rename fails, and so does the copy the commit falls back to, so
	// both targets have to be rolled back — and the first one's restore fails too.
	//
	// Copy runs once per backup before any commit (2), then as the failed
	// commit's fallback (3), then as the rollback's restores, newest first: the
	// second target (4), then the first (5).
	rollbackErr := errors.New("restore refused")
	shell.files.Rename = failOnNth2(2, ops.Rename, errInjected)
	copyCalls := 0
	shell.files.Copy = func(src, dest string) error {
		copyCalls++
		switch copyCalls {
		case 3:
			return errInjected
		case 5:
			return rollbackErr
		default:
			return ops.Copy(src, dest)
		}
	}

	err := shell.writeBack(context.Background(), "")
	if err == nil {
		t.Fatal("writeBack succeeded although a commit and its rollback both failed")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("the commit failure must survive: %v", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Errorf("the rollback failure must be reported too, not swallowed: %v", err)
	}
	if !strings.Contains(err.Error(), first) {
		t.Errorf("the error should name the file left holding new content: %v", err)
	}
	if extra := leftoverFiles(t, dir, "people.csv", "places.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
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

	if _, err := runScript(t, "UPDATE people SET name = 'bob';\n.save --in-place\n", changed, untouched); err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := readFile(t, changed); !strings.Contains(got, "bob") {
		t.Errorf("the changed table was not written back: %q", got)
	}
	if got := readFile(t, untouched); got != untouchedContent {
		t.Errorf("an unchanged table was rewritten:\n got %q\nwant %q", got, untouchedContent)
	}
}

// TestRollbackCommitted_ReportsWhatItCouldNotRestore checks the unit directly:
// a restore that fails is returned, not dropped. The caller joins it with the
// error that caused the rollback, so the user learns both why the save stopped
// and which file was left holding the new content.
func TestRollbackCommitted_ReportsWhatItCouldNotRestore(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "gone.csv")

	// A backup path that does not exist makes the restore fail.
	err := (&Shell{}).rollbackCommitted([]stagedWrite{{
		target: writeTarget{dest: dest},
		backup: filepath.Join(dir, "missing.bak"),
	}})
	if err == nil {
		t.Fatal("rollbackCommitted returned nil although the restore failed")
	}
	if !strings.Contains(err.Error(), dest) {
		t.Errorf("error should name the file it could not restore: %v", err)
	}

	// The destination is left as it was: absent.
	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("stat %s = %v, want it to stay absent", dest, statErr)
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
