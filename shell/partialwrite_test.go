package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The failure these tests reproduce is the one a mock that returns an error
// immediately cannot: a copy that has already destroyed the destination and then
// fails.
//
// sqly moves a staged file onto its destination with a rename, which is atomic —
// it either replaces the file or leaves it alone. Where the platform refuses a
// rename (Windows will not rename over a destination another handle has open),
// the fallback copies the staged bytes over the destination, and that opens it
// with O_TRUNC. A disk that fills up, an I/O error, a failing close: any of them
// after the truncate leaves the destination empty or half-written. There is no
// ordering of the copy that avoids it, so the safety has to come from a backup
// and a restore, and that is what these tests exercise.
//
// A Copy stub that returns an error without touching the destination would pass
// against an implementation with no backup at all, which is exactly the bug that
// was here. Every stub below writes damage first.

// truncatingCopy returns a Copy that damages dest the way a real interrupted
// copy does — truncate, write a prefix, fail — and then returns failWith. Runs
// other than nth are performed for real, so backups and restores still work.
func truncatingCopy(actual func(src, dest string) error, nth int, prefix string, failWith error) func(src, dest string) error {
	calls := 0
	return func(src, dest string) error {
		calls++
		if calls != nth {
			return actual(src, dest)
		}
		// O_TRUNC is what the real fallback does, and it is what makes this
		// unrecoverable without a backup.
		f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // a test-owned temp path
		if err != nil {
			return err
		}
		_, _ = f.WriteString(prefix)
		_ = f.Close()
		return failWith
	}
}

// alwaysFailRename refuses every rename, which is how a test reaches the
// fallback copy on a platform where renames work. This is the Windows path:
// there the refusal comes from the OS, here it comes from the stub, and the code
// under test cannot tell the difference.
func alwaysFailRename(err error) func(oldpath, newpath string) error {
	return func(string, string) error { return err }
}

// TestOutputWrite_FallbackCopyFailurePreservesTheDestination is the `--output`
// half of the contract the documentation states: an existing destination is
// either the old file or the new one, never a truncated or half-written one.
//
// The rename is refused, so the commit falls back to a copy; the copy truncates
// the destination, writes a partial line, and fails. Without a backup the
// destination would be left holding that partial line.
func TestOutputWrite_FallbackCopyFailurePreservesTheDestination(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "report.csv")
	const original = "id,name\n1,alice\n2,bob\n"
	if err := os.WriteFile(dest, []byte(original), 0o644); err != nil { //nolint:gosec // the point is a world-readable file
		t.Fatal(err)
	}

	ops := defaultFileOps()
	s := &Shell{files: ops}
	s.files.Rename = alwaysFailRename(errors.New("rename refused, as on Windows"))
	copyFailure := errors.New("no space left on device")
	// The backup is taken only once the rename has been refused, so call 1 is the
	// backup and call 2 is the fallback: the one that damages.
	s.files.Copy = truncatingCopy(ops.Copy, 2, "id,na", copyFailure)

	err := s.writeFileAtomically(dest, func(staging string) error {
		return os.WriteFile(staging, []byte("id,name\n9,carol\n"), 0o600)
	})
	if err == nil {
		t.Fatal("writeFileAtomically succeeded although the commit copy failed")
	}
	if !errors.Is(err, copyFailure) {
		t.Errorf("the copy failure must survive: %v", err)
	}
	if got := readFile(t, dest); got != original {
		t.Errorf("the destination was left damaged:\n got %q\nwant %q", got, original)
	}
	if extra := leftoverFiles(t, dir, "report.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestOutputWrite_RestoreFailureIsReportedWithTheCause covers the outcome the
// user has to be told about: the copy damaged the destination and putting it
// back failed too, so the file is now neither version. Both errors must come
// out — reporting only the restore failure hides why the write stopped, and
// reporting only the write failure lets the user believe the file is intact.
func TestOutputWrite_RestoreFailureIsReportedWithTheCause(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "report.csv")
	if err := os.WriteFile(dest, []byte("id\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ops := defaultFileOps()
	s := &Shell{files: ops}
	s.files.Rename = alwaysFailRename(errors.New("rename refused"))
	copyFailure := errors.New("disk full")
	restoreFailure := errors.New("restore refused")
	calls := 0
	s.files.Copy = func(src, dest string) error {
		calls++
		switch calls {
		case 2: // the commit's fallback: damage, then fail
			return truncatingCopy(ops.Copy, 1, "id", copyFailure)(src, dest)
		case 3: // the restore
			return restoreFailure
		default:
			return ops.Copy(src, dest)
		}
	}

	err := s.writeFileAtomically(dest, func(staging string) error {
		return os.WriteFile(staging, []byte("id\n2\n"), 0o600)
	})
	if err == nil {
		t.Fatal("writeFileAtomically succeeded although the commit and its restore both failed")
	}
	if !errors.Is(err, copyFailure) {
		t.Errorf("the commit failure must survive: %v", err)
	}
	if !errors.Is(err, restoreFailure) {
		t.Errorf("the restore failure must be reported too, not swallowed: %v", err)
	}
	if !strings.Contains(err.Error(), dest) {
		t.Errorf("the error must name the file left holding partial content: %v", err)
	}
	var opErr *fileOpError
	if !errors.As(err, &opErr) {
		t.Errorf("the failure should be classifiable with errors.As: %v", err)
	}
}

// TestOutputWrite_FallbackCopySucceedsWhenRenameIsRefused is the Windows happy
// path. The rename is refused exactly as it is there, and the write still has to
// land, with the destination's permissions intact — a fallback that quietly
// stopped working would otherwise be invisible on Linux and macOS.
func TestOutputWrite_FallbackCopySucceedsWhenRenameIsRefused(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "report.csv")
	if err := os.WriteFile(dest, []byte("id\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &Shell{files: defaultFileOps()}
	s.files.Rename = alwaysFailRename(errors.New("rename refused, as on Windows"))

	const updated = "id\n2\n"
	if err := s.writeFileAtomically(dest, func(staging string) error {
		return os.WriteFile(staging, []byte(updated), 0o600)
	}); err != nil {
		t.Fatalf("writeFileAtomically through the fallback copy: %v", err)
	}
	if got := readFile(t, dest); got != updated {
		t.Errorf("the fallback copy did not land: got %q, want %q", got, updated)
	}
	if extra := leftoverFiles(t, dir, "report.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestOutputWrite_FailedCommitToANewPathLeavesNoFile is the other side of the
// backup: there was nothing at the destination, so there is nothing to restore,
// and the half-written file the fallback created must not be left behind
// pretending to be a result.
func TestOutputWrite_FailedCommitToANewPathLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "new.csv")
	placeholder := filepath.Join(dir, "placeholder")
	if err := os.WriteFile(placeholder, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	ops := defaultFileOps()
	s := &Shell{files: ops}
	// The destination does not exist, so a rename is what runs — and a rename
	// that fails creates nothing. Stat must therefore report the destination as
	// missing for the fallback to be skipped.
	s.files.Rename = alwaysFailRename(errors.New("rename refused"))

	err := s.writeFileAtomically(dest, func(staging string) error {
		return os.WriteFile(staging, []byte("id\n1\n"), 0o600)
	})
	if err == nil {
		t.Fatal("writeFileAtomically succeeded although the rename failed")
	}
	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a failed write to a new path left a file behind: %v", statErr)
	}
	if extra := leftoverFiles(t, dir, "placeholder"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_PartialCopyOnTheSecondTargetRestoresBoth is the `.save` half.
// Two sources are saved in place; the second commit falls back to a copy that
// truncates it, writes a prefix, and fails. Both files must end up as they were
// — including the second, which is the one actually damaged and the one the
// rollback used to skip.
func TestWriteBack_PartialCopyOnTheSecondTargetRestoresBoth(t *testing.T) {
	dir := t.TempDir()
	const (
		firstOriginal  = "id,name\n1,alice\n"
		secondOriginal = "id,city\n1,tokyo\n"
	)
	first := writeCSV(t, dir, "people.csv", firstOriginal)
	second := writeCSV(t, dir, "places.csv", secondOriginal)

	shell, cleanup := writeBackShell(t, first, second)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people", "places")

	ops := defaultFileOps()
	shell.files = ops
	// Only the second target's rename is refused, so the first lands atomically
	// and the second reaches the fallback copy.
	shell.files.Rename = failOnNth2(2, ops.Rename, errors.New("rename refused, as on Windows"))
	copyFailure := errors.New("no space left on device")
	// Copies: backup of people (1), backup of places (2), the second commit's
	// fallback (3) — which truncates places.csv and fails.
	shell.files.Copy = truncatingCopy(ops.Copy, 3, "id,ci", copyFailure)

	err := shell.writeBack(context.Background(), "")
	if err == nil {
		t.Fatal("writeBack succeeded although the second commit failed")
	}
	if !errors.Is(err, copyFailure) {
		t.Errorf("the copy failure must survive: %v", err)
	}
	if got := readFile(t, second); got != secondOriginal {
		t.Errorf("the damaged target was not restored:\n got %q\nwant %q", got, secondOriginal)
	}
	if got := readFile(t, first); got != firstOriginal {
		t.Errorf("the target that had already landed was not restored:\n got %q\nwant %q", got, firstOriginal)
	}
	if extra := leftoverFiles(t, dir, "people.csv", "places.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestWriteBack_FallbackCopyOnEveryTargetStillSaves is the Windows happy path
// for `.save`: no rename is available at all, every commit goes through the
// fallback copy, and the save still has to succeed for all targets.
func TestWriteBack_FallbackCopyOnEveryTargetStillSaves(t *testing.T) {
	dir := t.TempDir()
	first := writeCSV(t, dir, "people.csv", "id,name\n1,alice\n")
	second := writeCSV(t, dir, "places.csv", "id,city\n1,tokyo\n")

	shell, cleanup := writeBackShell(t, first, second)
	defer cleanup()
	silenceStderr(t)
	markChanged(t, shell, "people", "places")

	ops := defaultFileOps()
	shell.files = ops
	shell.files.Rename = alwaysFailRename(errors.New("rename refused, as on Windows"))

	if err := shell.writeBack(context.Background(), ""); err != nil {
		t.Fatalf("writeBack through the fallback copy: %v", err)
	}
	if got := readFile(t, first); !strings.Contains(got, "alice") {
		t.Errorf("the first target did not land: %q", got)
	}
	if got := readFile(t, second); !strings.Contains(got, "tokyo") {
		t.Errorf("the second target did not land: %q", got)
	}
	if extra := leftoverFiles(t, dir, "people.csv", "places.csv"); len(extra) > 0 {
		t.Errorf("staging or backup files left behind: %v", extra)
	}
}

// TestOutputWrite_SuccessfulRenameCopiesNothing pins the cost of the safety
// added above. A rename replaces the destination without reading it, so copying
// the old file aside first would double the work of every export for a case a
// successful rename never reaches. The backup is taken only once the rename has
// been refused.
func TestOutputWrite_SuccessfulRenameCopiesNothing(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "report.csv")
	if err := os.WriteFile(dest, []byte("id\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ops := defaultFileOps()
	s := &Shell{files: ops}
	copies := 0
	s.files.Copy = func(src, dst string) error {
		copies++
		return ops.Copy(src, dst)
	}

	const updated = "id\n2\n"
	if err := s.writeFileAtomically(dest, func(staging string) error {
		return os.WriteFile(staging, []byte(updated), 0o600)
	}); err != nil {
		t.Fatalf("writeFileAtomically: %v", err)
	}
	if copies != 0 {
		t.Errorf("a write that renamed cleanly copied %d time(s); the backup should be lazy", copies)
	}
	if got := readFile(t, dest); got != updated {
		t.Errorf("destination = %q, want %q", got, updated)
	}
}
