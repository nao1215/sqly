//go:build smoke

package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// writeFixture writes a fixture file inside dir and returns its path.
func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// dirEntries lists the names directly under dir, sorted, so a test can compare
// the directory before and after a run and see anything sqly left behind.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// TestSmoke_MultiFileImportIsAtomic drives the ordered multi-file import through
// the real binary, one subtest per reason the import can fail. Each case asserts
// the same four things, because a rollback that only half works shows up in a
// different one each time: a non-zero exit, a diagnostic on stderr and nothing
// on stdout, no output file written, and a session in which the earlier inputs
// left no table behind.
//
// The good input is always first and the broken one last, which is the case a
// per-file import gets wrong: the first file is already applied by the time the
// second one fails.
func TestSmoke_MultiFileImportIsAtomic(t *testing.T) {
	const goodCSV = "id,name\n1,alice\n2,bob\n"

	tests := []struct {
		name string
		// file is the broken input: its name and its bytes.
		fileName string
		content  string
		// wantStderr is a fragment the diagnostic must contain, so the test
		// fails when the run fails for an unintended reason.
		wantStderr string
	}{
		{
			name:       "csv row wider than its header",
			fileName:   "broken.csv",
			content:    "id,name\n1,alice,unexpected\n",
			wantStderr: "broken",
		},
		{
			name:       "json that is not an array of objects",
			fileName:   "broken.json",
			content:    "{ this is not json",
			wantStderr: "broken",
		},
		{
			name:       "ndjson with a malformed line",
			fileName:   "broken.jsonl",
			content:    "{\"a\":1}\n{\"a\":\n",
			wantStderr: "broken",
		},
		{
			name:       "tsv row wider than its header",
			fileName:   "broken.tsv",
			content:    "id\tname\n1\talice\textra\n",
			wantStderr: "broken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			good := writeFixture(t, dir, "good.csv", goodCSV)
			broken := writeFixture(t, dir, tt.fileName, tt.content)
			outPath := filepath.Join(dir, "result.csv")

			stdout, stderr, code := run(t, "",
				"--output-format", "csv",
				"--sql", "SELECT * FROM good",
				"--output", outPath,
				good, broken)

			if code == 0 {
				t.Fatalf("exit code = 0, want non-zero (stdout=%q stderr=%q)", stdout, stderr)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("stdout = %q, want it empty on a failed import", stdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want it to mention %q so the failure is attributable", stderr, tt.wantStderr)
			}
			assertNoPanic(t, stdout, stderr)
			if _, err := os.Stat(outPath); err == nil {
				t.Error("--output file was written even though the import failed")
			}

			// The failure must be recoverable: importing the good input alone
			// afterwards has to work and produce the full result.
			stdout, stderr, code = run(t, "", "--output-format", "csv", "--sql", "SELECT COUNT(*) AS c FROM good", good)
			if code != 0 {
				t.Fatalf("re-import after the failed run exited %d (stderr=%q)", code, stderr)
			}
			if !strings.Contains(stdout, "2") {
				t.Errorf("re-import result = %q, want the 2 rows of the good input", stdout)
			}
		})
	}
}

// TestSmoke_UnreadableFileLeavesNothing covers the input that fails before it is
// parsed at all. Permission bits behave differently on Windows, so the case runs
// where it is meaningful.
func TestSmoke_UnreadableFileLeavesNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not deny read the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, which ignores the permission bits this case relies on")
	}

	dir := t.TempDir()
	good := writeFixture(t, dir, "good.csv", "id,name\n1,alice\n")
	unreadable := writeFixture(t, dir, "denied.csv", "id,name\n2,bob\n")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })

	stdout, stderr, code := run(t, "", "--output-format", "csv", "--sql", "SELECT * FROM good", good, unreadable)
	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero (stdout=%q)", stdout)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want it empty", stdout)
	}
	if !strings.Contains(stderr, "denied") {
		t.Errorf("stderr = %q, want it to name the unreadable input", stderr)
	}
	assertNoPanic(t, stdout, stderr)
}

// TestSmoke_SQLSyntaxErrorLeavesNoPartialOutput checks the other end of the run:
// the import succeeded, but the query did not. The exit code must be non-zero
// and no output file may exist, so a script cannot mistake a failed query for an
// empty result.
func TestSmoke_SQLSyntaxErrorLeavesNoPartialOutput(t *testing.T) {
	dir := t.TempDir()
	good := writeFixture(t, dir, "good.csv", "id,name\n1,alice\n")
	outPath := filepath.Join(dir, "result.csv")

	stdout, stderr, code := run(t, "",
		"--output-format", "csv", "--sql", "SELECT FROM WHERE", "--output", outPath, good)
	if code == 0 {
		t.Fatalf("exit code = 0 for a syntax error, want non-zero (stdout=%q)", stdout)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want it empty", stdout)
	}
	if stderr == "" {
		t.Error("a syntax error produced no diagnostic on stderr")
	}
	assertNoPanic(t, stdout, stderr)
	if _, err := os.Stat(outPath); err == nil {
		t.Error("--output file exists after a failed query")
	}
}

// TestSmoke_SameTableNameConflict pins what two inputs that sanitize to the same
// table name do: the run is refused outright, naming the second file, rather
// than silently letting one input overwrite the other's rows. A half-resolved
// collision — a table holding some rows from each file — is the state this
// forbids, and the query that would have read it must produce nothing.
func TestSmoke_SameTableNameConflict(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	first := writeFixture(t, dir, "dup.csv", "id,name\n1,alice\n2,bob\n")
	second := writeFixture(t, sub, "dup.csv", "id,name\n9,zoe\n")
	outPath := filepath.Join(dir, "result.csv")

	stdout, stderr, code := run(t, "",
		"--output-format", "csv", "--sql", "SELECT id, name FROM dup ORDER BY id",
		"--output", outPath, first, second)

	if code == 0 {
		t.Fatalf("same-name import exited 0, want non-zero (stdout=%q)", stdout)
	}
	assertNoPanic(t, stdout, stderr)
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want it empty on a refused import", stdout)
	}
	if !strings.Contains(stderr, "collision") || !strings.Contains(stderr, second) {
		t.Errorf("stderr = %q, want it to name the colliding input %q", stderr, second)
	}
	// Neither file's rows may reach the output: no zoe from the second, and no
	// alice/bob from the first.
	if _, err := os.Stat(outPath); err == nil {
		t.Error("--output file was written even though the import was refused")
	}
	for _, leaked := range []string{"zoe", "alice", "bob"} {
		if strings.Contains(stdout, leaked) {
			t.Errorf("stdout leaked row data %q from a refused import: %q", leaked, stdout)
		}
	}
}

// TestSmoke_ShellSessionSurvivesAFailedImport checks the interactive/batch path
// rather than startup. A `.import` that fails must leave the session exactly as
// it was: the already-imported table still queryable, and no table from the
// failed import.
func TestSmoke_ShellSessionSurvivesAFailedImport(t *testing.T) {
	dir := t.TempDir()
	good := writeFixture(t, dir, "good.csv", "id,name\n1,alice\n")
	extra := writeFixture(t, dir, "extra.csv", "id,city\n1,tokyo\n")
	broken := writeFixture(t, dir, "broken.csv", "id,name\n1,alice,unexpected\n")

	script := ".import " + extra + "\n" +
		".import " + broken + "\n" +
		".tables\n" +
		".mode csv\n" +
		"SELECT COUNT(*) AS c FROM good;\n" +
		"SELECT COUNT(*) AS c FROM extra;\n"

	stdout, stderr, code := run(t, script, good)
	if code == 0 {
		t.Fatalf("a batch run whose .import failed exited 0 (stdout=%q)", stdout)
	}
	assertNoPanic(t, stdout, stderr)
	if strings.Contains(stdout, "broken") {
		t.Errorf(".tables listed the failed import's table:\n%s", stdout)
	}
	if !strings.Contains(stderr, "broken") {
		t.Errorf("stderr = %q, want it to name the input that failed", stderr)
	}
}

// TestSmoke_SaveDoesNotPersistUncommittedData checks the write-back path against
// a failed import: .save must write the tables the session actually holds and
// nothing from the import that was rolled back.
func TestSmoke_SaveDoesNotPersistUncommittedData(t *testing.T) {
	dir := t.TempDir()
	good := writeFixture(t, dir, "good.csv", "id,name\n1,alice\n")
	broken := writeFixture(t, dir, "broken.csv", "id,name\n1,alice,unexpected\n")
	saveTablesDir := filepath.Join(dir, "out")
	if err := os.Mkdir(saveTablesDir, 0o750); err != nil {
		t.Fatal(err)
	}

	script := ".import " + broken + "\n" +
		".save " + saveTablesDir + "\n"
	stdout, stderr, code := run(t, script, good)
	if code == 0 {
		t.Fatalf("exit code = 0 after a failed .import, want non-zero (stdout=%q stderr=%q)", stdout, stderr)
	}
	assertNoPanic(t, stdout, stderr)

	for _, name := range dirEntries(t, saveTablesDir) {
		if strings.HasPrefix(name, "broken") {
			t.Errorf(".save wrote %q from an import that was rolled back", name)
		}
	}
}

// TestSmoke_SavePreflightIsAUsageError pins the class of the .save preflight
// refusal. The check runs before the first statement, so nothing has run when it
// fires, and the exit-code table reserves 1 for "a statement ran and failed".
// It reported 1 anyway, because a bare error falls through to that code.
func TestSmoke_SavePreflightIsAUsageError(t *testing.T) {
	dir := t.TempDir()
	source := writeFixture(t, dir, "src.csv", "id\n1\n")
	out := filepath.Join(dir, "out")

	script := "CREATE TABLE brand_new (a TEXT);\n" +
		"INSERT INTO brand_new VALUES ('v');\n" +
		".save " + out + "\n"
	stdout, stderr, code := run(t, script, source)

	if code != 2 {
		t.Errorf("exit code = %d, want 2 (the script was not accepted)\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "cannot persist") {
		t.Errorf("stderr = %q, want the preflight refusal", stderr)
	}
	// Nothing ran, so neither statement's feedback line may appear.
	for _, feedback := range []string{"statement executed successfully", "affected is"} {
		if strings.Contains(stdout, feedback) {
			t.Errorf("stdout = %q, want no statement feedback: the script was refused before it ran", stdout)
		}
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("a refused script created %s", out)
	}
}

// TestSmoke_SaveAfterTheLastSaveIsAllowed is the other side of the preflight
// window. A statement write-back cannot persist is only a problem when a .save
// could reach it, so one after the final .save leaves the run alone.
func TestSmoke_SaveAfterTheLastSaveIsAllowed(t *testing.T) {
	dir := t.TempDir()
	source := writeFixture(t, dir, "src.csv", "id\n1\n")
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o750); err != nil {
		t.Fatal(err)
	}

	script := "UPDATE src SET id = 2;\n" +
		".save " + out + "\n" +
		"CREATE TABLE scratch (a TEXT);\n"
	_, stderr, code := run(t, script, source)

	if code != 0 {
		t.Errorf("exit code = %d, want 0: the scratch table is after the last .save\nstderr: %s", code, stderr)
	}
}

// TestSmoke_NoStrayFilesLeftBehind runs sqly with its working directory inside a
// directory the test owns and requires that directory to be unchanged
// afterwards. A temporary staging file or a stray SQLite database left in the
// user's working directory is the kind of leak nothing else in the suite would
// notice.
func TestSmoke_NoStrayFilesLeftBehind(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "data.csv", "id,name\n1,alice\n")
	before := dirEntries(t, dir)

	t.Run("successful run", func(t *testing.T) {
		_, stderr, code := runIn(t, dir, "", "--output-format", "csv", "--sql", "SELECT * FROM data", "data.csv")
		if code != 0 {
			t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
		}
		if got := dirEntries(t, dir); strings.Join(got, ",") != strings.Join(before, ",") {
			t.Errorf("working directory changed: before=%v after=%v", before, got)
		}
	})

	t.Run("failed run", func(t *testing.T) {
		writeFixture(t, dir, "bad.csv", "id,name\n1,alice,unexpected\n")
		snapshot := dirEntries(t, dir)
		_, _, code := runIn(t, dir, "", "--output-format", "csv", "--sql", "SELECT 1", "data.csv", "bad.csv")
		if code == 0 {
			t.Fatal("exit code = 0 for a failed import, want non-zero")
		}
		if got := dirEntries(t, dir); strings.Join(got, ",") != strings.Join(snapshot, ",") {
			t.Errorf("failed run left files behind: before=%v after=%v", snapshot, got)
		}
	})
}

// TestSmoke_ConcurrentRunsDoNotCollide runs several imports at once, each in its
// own directory, and requires every one to succeed with its own data. Sessions
// are per-process, so a shared temporary path or a fixed database file would
// show up here as a failure or as one run seeing another's rows.
func TestSmoke_ConcurrentRunsDoNotCollide(t *testing.T) {
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := writeFixture(t, dir, name+".csv", "id,who\n1,"+name+"\n")

			stdout, stderr, code := run(t, "",
				"--output-format", "csv", "--sql", "SELECT who FROM "+name, path)
			if code != 0 {
				t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
			}
			if !strings.Contains(stdout, name) {
				t.Errorf("stdout = %q, want the row this run imported", stdout)
			}
			for _, other := range []string{"alpha", "beta", "gamma", "delta"} {
				if other != name && strings.Contains(stdout, other) {
					t.Errorf("stdout = %q, leaked data from the %q run", stdout, other)
				}
			}
		})
	}
}
