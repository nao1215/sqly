package shell

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
)

// The dialect warning: said once, on stderr, and only where a dialect applies.
//
// The tests below deliberately do not assert the whole sentence. What has to
// hold is that it appears exactly once, that it goes to stderr and never to
// stdout, and that it names translation and SQLite semantics — not the
// punctuation. A test pinned to the exact string turns every wording improvement
// into a failure, which teaches people to change the test rather than read it.

// dialectWarningMarkers are the claims the message must make, matched
// case-insensitively.
var dialectWarningMarkers = []string{"translated to sqlite", "sqlite semantics"}

// countDialectWarnings returns how many lines of stderr carry the warning.
func countDialectWarnings(stderr string) int {
	count := 0
	for _, line := range strings.Split(stderr, "\n") {
		lower := strings.ToLower(line)
		hasAll := true
		for _, marker := range dialectWarningMarkers {
			if !strings.Contains(lower, marker) {
				hasAll = false
				break
			}
		}
		if hasAll {
			count++
		}
	}
	return count
}

// runDialect builds a shell, runs it, and returns stdout and stderr separately.
func runDialect(t *testing.T, stdin string, args ...string) (stdout, stderr string) {
	t.Helper()

	shell, cleanup, err := newShell(t, args)
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return false }
	if stdin != "" {
		shell.stdin = strings.NewReader(stdin)
	}

	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			if err := shell.Run(context.Background()); err != nil {
				t.Fatalf("Run: %v", err)
			}
		})
	})
	return stdout, stderr
}

func TestDialectWarning_SQLiteSaysNothing(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	for _, args := range [][]string{
		{"sqly", "--output-format", "csv", "--sql", "SELECT * FROM x", csv},
		{"sqly", "--dialect", "sqlite", "--output-format", "csv", "--sql", "SELECT * FROM x", csv},
	} {
		_, stderr := runDialect(t, "", args...)
		if got := countDialectWarnings(stderr); got != 0 {
			t.Errorf("%v printed %d dialect warning(s), want 0: SQLite is the engine, not a translation\nstderr: %s",
				args, got, stderr)
		}
	}
}

func TestDialectWarning_EachNonSQLiteDialectWarnsOnce(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	for _, tc := range []struct{ flag, name string }{
		{"mysql", "MySQL"},
		{"postgresql", "PostgreSQL"},
		{"googlesql", "GoogleSQL"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			stdout, stderr := runDialect(t, "",
				"sqly", "--dialect", tc.flag, "--output-format", "csv", "--sql", "SELECT * FROM x", csv)

			if got := countDialectWarnings(stderr); got != 1 {
				t.Errorf("stderr carried %d warning(s), want exactly 1:\n%s", got, stderr)
			}
			if !strings.Contains(stderr, tc.name) {
				t.Errorf("stderr does not name the dialect as %q:\n%s", tc.name, stderr)
			}
			if countDialectWarnings(stdout) != 0 {
				t.Errorf("the warning reached stdout, which carries results:\n%s", stdout)
			}
		})
	}
}

func TestDialectWarning_SeveralStatementsStillWarnOnce(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")
	sqlPath := filepath.Join(dir, "many.sql")
	statements := "SELECT * FROM x;\nSELECT * FROM x;\nSELECT * FROM x;\nSELECT * FROM x;\n"
	if err := os.WriteFile(sqlPath, []byte(statements), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr := runDialect(t, "",
		"sqly", "--dialect", "postgresql", "--output-format", "table", "--sql-file", sqlPath, csv)

	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("a four-statement --sql-file printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

func TestDialectWarning_ScriptFileWarnsOnce(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")
	scriptPath := filepath.Join(dir, "run.sqly")
	script := ".tables\nSELECT * FROM x;\nSELECT * FROM x;\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr := runDialect(t, "",
		"sqly", "--dialect", "mysql", "--output-format", "table", "--script-file", scriptPath, csv)

	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("a --script-file printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

func TestDialectWarning_StdinScriptWarnsOnce(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	_, stderr := runDialect(t, "SELECT * FROM x;\nSELECT * FROM x;\n",
		"sqly", "--dialect", "googlesql", "--output-format", "table", csv)

	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("a piped script printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

// TestDialectWarning_DotDialectWarnsOnceAndDoesNotRearm covers the interactive
// path and the "switch back and forth" case in one session, which is where a
// per-call warning would show up as noise.
func TestDialectWarning_DotDialectWarnsOnceAndDoesNotRearm(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx := context.Background()

	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			for _, line := range []string{
				".dialect postgresql",
				".dialect sqlite",
				".dialect mysql",
				".dialect googlesql",
				".dialect postgresql",
			} {
				if err := shell.exec(ctx, line); err != nil {
					t.Fatalf("%s: %v", line, err)
				}
			}
		})
	})

	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("five .dialect switches printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

// TestDialectWarning_SQLiteFirstThenPostgresWarnsAtTheSwitch checks the warning
// fires when the dialect actually becomes non-SQLite, not at session start.
func TestDialectWarning_SQLiteFirstThenPostgresWarnsAtTheSwitch(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx := context.Background()

	quiet := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			if err := shell.exec(ctx, ".dialect sqlite"); err != nil {
				t.Fatal(err)
			}
		})
	})
	if got := countDialectWarnings(quiet); got != 0 {
		t.Errorf(".dialect sqlite printed %d warning(s), want 0:\n%s", got, quiet)
	}

	loud := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			if err := shell.exec(ctx, ".dialect postgresql"); err != nil {
				t.Fatal(err)
			}
		})
	})
	if got := countDialectWarnings(loud); got != 1 {
		t.Errorf(".dialect postgresql printed %d warning(s), want 1:\n%s", got, loud)
	}
}

// TestDialectWarning_MachineReadableStdoutStaysParseable is the reason the
// warning is on stderr at all.
func TestDialectWarning_MachineReadableStdoutStaysParseable(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n2\n")

	t.Run("json", func(t *testing.T) {
		stdout, stderr := runDialect(t, "",
			"sqly", "--dialect", "postgresql", "--output-format", "json", "--sql", "SELECT * FROM x", csv)
		var rows []map[string]any
		if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
			t.Fatalf("stdout is not a JSON array: %v\n%s", err, stdout)
		}
		if len(rows) != 2 {
			t.Errorf("rows = %d, want 2", len(rows))
		}
		if countDialectWarnings(stderr) != 1 {
			t.Errorf("stderr should still carry the warning:\n%s", stderr)
		}
	})

	t.Run("jsonl", func(t *testing.T) {
		stdout, _ := runDialect(t, "",
			"sqly", "--dialect", "mysql", "--output-format", "jsonl", "--sql", "SELECT * FROM x", csv)
		for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatalf("NDJSON line %q does not parse: %v", line, err)
			}
		}
	})

	t.Run("csv", func(t *testing.T) {
		stdout, _ := runDialect(t, "",
			"sqly", "--dialect", "googlesql", "--output-format", "csv", "--sql", "SELECT * FROM x", csv)
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 3 {
			t.Errorf("csv stdout = %d lines, want 3 (header + 2 rows):\n%s", len(lines), stdout)
		}
		if countDialectWarnings(stdout) != 0 {
			t.Errorf("the warning reached csv stdout:\n%s", stdout)
		}
	})
}

// TestDialectWarning_InspectIsSilent keeps the warning to the runs where a
// dialect means something. --inspect never translates user SQL, so it says
// nothing about the dialect it did not use. An explicit --dialect is rejected
// before this point (see validateInspectFlags), which leaves the default.
func TestDialectWarning_InspectIsSilent(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	stdout, stderr := runDialect(t, "", "sqly", "--inspect", csv)

	if got := countDialectWarnings(stderr); got != 0 {
		t.Errorf("--inspect printed %d dialect warning(s), want 0:\n%s", got, stderr)
	}
	var report inspectReportForTest
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("the inspect document did not survive: %v\n%s", err, stdout)
	}
}

// TestDialectWarning_HelpAndVersionAreSilent covers the two runs that never
// execute SQL at all.
func TestDialectWarning_HelpAndVersionAreSilent(t *testing.T) {
	for _, flag := range []string{"--help", "--version"} {
		_, stderr := runDialect(t, "", "sqly", "--dialect", "postgresql", flag)
		if got := countDialectWarnings(stderr); got != 0 {
			t.Errorf("%s printed %d dialect warning(s), want 0:\n%s", flag, got, stderr)
		}
	}
}

// TestDialectWarning_ARejectedCommandLineIsSilent keeps the warning out of a run
// that never got as far as deciding what to execute.
func TestDialectWarning_ARejectedCommandLineIsSilent(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	shell, cleanup, err := newShell(t, []string{"sqly", "--dialect", "postgresql", "--inspect", "--sql", "SELECT 1", csv})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	var runErr error
	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() { runErr = shell.Run(context.Background()) })
	})
	if runErr == nil {
		t.Fatal("--inspect with --sql was accepted, want a usage error")
	}
	if got := countDialectWarnings(stderr); got != 0 {
		t.Errorf("a rejected command line printed %d dialect warning(s), want 0:\n%s", got, stderr)
	}
}

// TestDialectWarning_TranslationFailuresDoNotRepeatIt checks the warning stays at
// one even when every statement of a script fails to translate — the case where
// a per-statement message would flood stderr hardest.
func TestDialectWarning_TranslationFailuresDoNotRepeatIt(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	shell, cleanup, err := newShell(t, []string{"sqly", "--dialect", "postgresql", "--output-format", "table", csv})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return false }
	// DISTINCT ON is PostgreSQL syntax the translator rejects outright, so each
	// statement fails at translation rather than at SQLite.
	shell.stdin = strings.NewReader("SELECT DISTINCT ON (a) a FROM x;\nSELECT DISTINCT ON (a) a FROM x;\n")

	stderr := captureStderr(t, func() {
		_ = captureStdout(t, func() { _ = shell.Run(context.Background()) })
	})
	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("a script of failing statements printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

// TestDialectWarning_UsesNoPackageState is the structural claim: the warning is
// remembered per shell, so two sessions in one process each say it once.
func TestDialectWarning_UsesNoPackageState(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	for i := range 2 {
		_, stderr := runDialect(t, "",
			"sqly", "--dialect", "mysql", "--output-format", "csv", "--sql", "SELECT * FROM x", csv)
		if got := countDialectWarnings(stderr); got != 1 {
			t.Errorf("session %d printed %d warning(s), want 1; a process-wide flag would silence the second:\n%s",
				i+1, got, stderr)
		}
	}
}

// TestDialectWarning_NamesEveryNonSQLiteDialect is the drift guard for the
// warning's wording: a dialect added to filesql must be announced by the name
// its own project uses, not by its lowercase wire value.
func TestDialectWarning_NamesEveryNonSQLiteDialect(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	for _, d := range dialect.Dialects() {
		if d == dialect.SQLite {
			continue
		}
		t.Run(string(d), func(t *testing.T) {
			_, stderr := runDialect(t, "",
				"sqly", "--dialect", string(d), "--output-format", "csv", "--sql", "SELECT * FROM x", csv)
			if !strings.Contains(stderr, d.DisplayName()) {
				t.Errorf("warning for %q = %q, want it to name %q", d, stderr, d.DisplayName())
			}
		})
	}
}
