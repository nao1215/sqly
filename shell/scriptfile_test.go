package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `--sql-file` holds SQL and rejects a dot-command by name and line. That is the
// right rule for a flag that says `sql`, and it left a gap: the only way to run
// a script that mixes SQL and dot-commands was to pipe it in, so moving the very
// same text into a file made it stop working for no reason a user could see.
//
// `--script-file` is that missing entry point. These tests pin the boundary
// between the two flags, which is the part that has to stay stable: what each
// one accepts, what it refuses, and that neither quietly becomes the other.
//
// None of them call t.Parallel: runScriptStreams swaps config.Stdout and
// config.Stderr, which are process-wide.

func TestScriptFile_RunsDotCommandsAndSQL(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "t.csv", "a,b\n1,2\n")
	script := filepath.Join(dir, "s.sqly")
	writeScript(t, script, "UPDATE t SET b = 9;\n.save --in-place\n")

	_, stderr, err := runWithArgs(t, "--script-file", script, src)
	if err != nil {
		t.Fatalf("--script-file with a dot-command failed: %v (%s)", err, stderr)
	}
	if got := readFile(t, src); !strings.Contains(got, "9") {
		t.Errorf(".save --in-place did not persist: %q", got)
	}
}

// TestScriptFile_IsTheSameTextAPipeWouldRun is the point of the flag: a script
// that works piped in must work from a file unchanged. If the two ever diverge,
// the flag has stopped being a way to keep a script in version control.
func TestScriptFile_IsTheSameTextAPipeWouldRun(t *testing.T) {
	const script = ".mode csv\nSELECT b FROM t;\n"

	pipedDir := t.TempDir()
	pipedSrc := writeCSV(t, pipedDir, "t.csv", "a,b\n1,2\n")
	piped, _, err := runScriptStreams(t, script, pipedSrc)
	if err != nil {
		t.Fatalf("piped script: %v", err)
	}

	fileDir := t.TempDir()
	fileSrc := writeCSV(t, fileDir, "t.csv", "a,b\n1,2\n")
	path := filepath.Join(fileDir, "s.sqly")
	writeScript(t, path, script)
	fromFile, _, err := runWithArgs(t, "--script-file", path, fileSrc)
	if err != nil {
		t.Fatalf("--script-file: %v", err)
	}

	if piped != fromFile {
		t.Errorf("the same script gave different output:\n piped: %q\n  file: %q", piped, fromFile)
	}
}

// TestSQLFile_StillRejectsADotCommand keeps the boundary from eroding. Adding
// --script-file must not relax --sql-file: the flag says SQL, and a .sql file
// that silently ran .save would be a shell script wearing a SQL extension.
func TestSQLFile_StillRejectsADotCommand(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "t.csv", "a,b\n1,2\n")
	path := filepath.Join(dir, "q.sql")
	writeScript(t, path, "SELECT 1;\n.save --in-place\n")

	_, _, err := runWithArgs(t, "--sql-file", path, src)
	if err == nil {
		t.Fatal("--sql-file accepted a dot-command")
	}
	if !strings.Contains(err.Error(), "--script-file") {
		t.Errorf("the refusal should point at --script-file, got: %v", err)
	}
}

func TestScriptFile_ConflictsWithTheOtherQuerySources(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "t.csv", "a,b\n1,2\n")
	script := filepath.Join(dir, "s.sqly")
	writeScript(t, script, "SELECT 1;\n")
	sqlFile := filepath.Join(dir, "q.sql")
	writeScript(t, sqlFile, "SELECT 1;\n")

	tests := []struct {
		name string
		args []string
	}{
		{name: "with --sql", args: []string{"--script-file", script, "--sql", "SELECT 1"}},
		{name: "with --sql-file", args: []string{"--script-file", script, "--sql-file", sqlFile}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runWithArgs(t, append(tt.args, src)...)
			if err == nil {
				t.Fatal("two sources for the work to run were accepted")
			}
			if !strings.Contains(err.Error(), "cannot be used together") {
				t.Errorf("the error should say the flags collide, got: %v", err)
			}
		})
	}
}

// TestScriptFile_RejectsOutput scopes the flag. A script writes files with
// .dump, at the point in the script where the destination means something; one
// --output for a whole script would have to pick one of its results and nothing
// says which.
func TestScriptFile_RejectsOutput(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "t.csv", "a,b\n1,2\n")
	script := filepath.Join(dir, "s.sqly")
	writeScript(t, script, "SELECT 1;\n")

	_, _, err := runWithArgs(t, "--script-file", script, "--output", filepath.Join(dir, "out.csv"), src)
	if err == nil {
		t.Fatal("--output was accepted with --script-file")
	}
	if !strings.Contains(err.Error(), ".dump") {
		t.Errorf("the error should point at .dump, got: %v", err)
	}
}

func TestScriptFile_RejectsAnEmptyOrMissingFile(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "t.csv", "a,b\n1,2\n")
	empty := filepath.Join(dir, "empty.sqly")
	writeScript(t, empty, "\n   \n")

	t.Run("an empty script is rejected rather than exiting 0 having done nothing", func(t *testing.T) {
		_, stderr, err := runWithArgs(t, "--script-file", empty, src)
		if err == nil {
			t.Fatal("an empty --script-file was accepted")
		}
		if got := ExitCode(err); got != ExitUsage {
			t.Errorf("ExitCode = %d, want %d (%s)", got, ExitUsage, stderr)
		}
	})

	t.Run("a missing script is an input error", func(t *testing.T) {
		_, _, err := runWithArgs(t, "--script-file", filepath.Join(dir, "nope.sqly"), src)
		if err == nil {
			t.Fatal("a missing --script-file was accepted")
		}
		if got := ExitCode(err); got != ExitInput {
			t.Errorf("ExitCode = %d, want %d", got, ExitInput)
		}
	})
}

// TestScriptFile_AcceptsADotCommandOnlyScript is the case that separates
// readScriptFile from readSQLFile: a script whose whole content is dot-commands
// has no SQL statement in it and is perfectly valid.
func TestScriptFile_AcceptsADotCommandOnlyScript(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "t.csv", "a,b\n1,2\n")
	script := filepath.Join(dir, "s.sqly")
	writeScript(t, script, ".tables\n")

	stdout, stderr, err := runWithArgs(t, "--script-file", script, src)
	if err != nil {
		t.Fatalf("a dot-command-only script failed: %v (%s)", err, stderr)
	}
	if !strings.Contains(stdout+stderr, "t") {
		t.Errorf(".tables produced no output: %q / %q", stdout, stderr)
	}
}

// TestScriptFile_FreesStdinForADataset is the reason --sql-file exists at all,
// and it must hold here too: the script comes from the file, so a piped dataset
// still has stdin to itself.
func TestScriptFile_FreesStdinForADataset(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "s.sqly")
	writeScript(t, script, ".mode csv\nSELECT COUNT(*) AS n FROM stdin;\n")

	s, cleanup, err := newShell(t, []string{"sqly", "--stdin-format", "csv", "--script-file", script})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s.isTTY = func() bool { return false }
	s.stdin = strings.NewReader("a,b\n1,2\n3,4\n")

	stdout, stderr := captureStreams(t, func() error { return s.Run(t.Context()) })
	if !strings.Contains(stdout, "2") {
		t.Errorf("the piped dataset was not queried: %q / %q", stdout, stderr)
	}
}

func writeScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
