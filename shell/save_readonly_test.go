package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A file whose permissions deny writing is the filesystem's own way of saying
// "do not modify this". Every neighboring tool honors it: shell redirection,
// tee, cp, and sed all fail with EACCES, and an editor demands a bang. sqly
// staged its write beside the destination and renamed over it, which needs a
// writable directory and never consults the file's own mode — so a read-only
// file was replaced at exit 0, and its mode was faithfully copied onto the new
// content, leaving the protection in place over data the user had protected.
//
// None of these call t.Parallel: runScriptStreams swaps config.Stdout and
// config.Stderr, which are process-wide.

// requireModeEnforcement skips when the process can write regardless of the
// mode bits, which is the case for root and for Windows, where a read-only file
// is not expressed the same way.
func requireModeEnforcement(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not express a read-only file through the mode bits sqly reads")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes regardless of the mode bits, so the refusal cannot be observed")
	}
}

func TestSaveInPlace_RefusesAReadOnlySource(t *testing.T) {
	requireModeEnforcement(t)

	dir := t.TempDir()
	source := writeCSV(t, dir, "locked.csv", "name,age\nken,40\n")
	if err := os.Chmod(source, 0o444); err != nil { //nolint:gosec // a read-only file is the subject of the test
		t.Fatalf("make the source read-only: %v", err)
	}

	_, stderr, err := runScriptStreams(t, "UPDATE locked SET age = 41;\n.save --in-place\n", source)
	if err == nil {
		t.Fatal("an in-place save over a read-only source was accepted")
	}
	if !strings.Contains(stderr, "read-only") {
		t.Errorf("the refusal should say the file is read-only, got: %s", stderr)
	}
	if got := readFile(t, source); got != "name,age\nken,40\n" {
		t.Errorf("the read-only source was modified despite the refusal: %q", got)
	}
}

// A save covering several files is all-or-nothing, and one read-only file among
// writable ones has to leave every one of them alone.
func TestSaveInPlace_ReadOnlySourceLeavesTheOtherFilesAlone(t *testing.T) {
	requireModeEnforcement(t)

	dir := t.TempDir()
	writable := writeCSV(t, dir, "open.csv", "name,age\nken,40\n")
	locked := writeCSV(t, dir, "locked.csv", "name,age\nmai,30\n")
	if err := os.Chmod(locked, 0o444); err != nil { //nolint:gosec // a read-only file is the subject of the test
		t.Fatalf("make the source read-only: %v", err)
	}

	script := "UPDATE open SET age = 41;\nUPDATE locked SET age = 31;\n.save --in-place\n"
	_, stderr, err := runScriptStreams(t, script, writable, locked)
	if err == nil {
		t.Fatal("a save covering a read-only source was accepted")
	}
	if !strings.Contains(stderr, "read-only") {
		t.Errorf("the refusal should say which file is read-only, got: %s", stderr)
	}
	if got := readFile(t, writable); got != "name,age\nken,40\n" {
		t.Errorf("the writable file was rewritten even though the save was refused: %q", got)
	}
}

// An ACH or Fedwire source is written back as one file rebuilt from several
// tables, which gives it a planning path of its own. That path did not ask
// whether the file may be written, so the guard covered every format except the
// two whose contents are financial records.
func TestSaveInPlace_RefusesAReadOnlyFinancialSource(t *testing.T) {
	requireModeEnforcement(t)

	dir := t.TempDir()
	source := filepath.Join(dir, "ppd.ach")
	fixture, err := os.ReadFile(filepath.Join("..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatalf("read the ACH fixture: %v", err)
	}
	if err := os.WriteFile(source, fixture, 0o600); err != nil {
		t.Fatalf("write the ACH source: %v", err)
	}
	if err := os.Chmod(source, 0o444); err != nil { //nolint:gosec // a read-only file is the subject of the test
		t.Fatalf("make the source read-only: %v", err)
	}

	script := "UPDATE ppd_entries SET individual_name = 'RENAMED' WHERE entry_index = 0;\n.save --in-place\n"
	_, stderr, err := runScriptStreams(t, script, source)
	if err == nil {
		t.Fatal("an in-place save over a read-only ACH source was accepted")
	}
	if !strings.Contains(stderr, "read-only") {
		t.Errorf("the refusal should say the file is read-only, got: %s", stderr)
	}
	after, readErr := os.ReadFile(source)
	if readErr != nil {
		t.Fatalf("read the source back: %v", readErr)
	}
	if string(after) != string(fixture) {
		t.Error("the read-only ACH source was rewritten despite the refusal")
	}
}

func TestSaveDir_RefusesAReadOnlyDestinationDirectory(t *testing.T) {
	requireModeEnforcement(t)

	dir := t.TempDir()
	source := writeCSV(t, dir, "data.csv", "name,age\nken,40\n")
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o555); err != nil { //nolint:gosec // a directory that cannot be written is the subject of the test
		t.Fatalf("create the read-only directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(out, 0o755) }) //nolint:gosec // restore so TempDir can be removed

	_, stderr, err := runScriptStreams(t, "UPDATE data SET age = 41;\n.save "+out+"\n", source)
	if err == nil {
		t.Fatal("a save into a directory that cannot be written was accepted")
	}
	if stderr == "" {
		t.Error("the refusal said nothing about why the directory could not be written")
	}
}
