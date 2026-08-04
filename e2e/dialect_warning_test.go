//go:build smoke

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The dialect warning, against the real binary on every platform.
//
// The atago specs cover the same claims on Linux and macOS, where `sh -c` and
// `grep -c` are available; this suite is pure Go, so Windows gets the same
// coverage. What is asserted is that the warning appears exactly once on
// stderr, never on stdout, and never at all for SQLite — not the punctuation of
// the sentence, so the wording stays free to improve.

// dialectWarningMarkers are the claims the message must make, matched
// case-insensitively so a capitalization change does not fail a test.
var dialectWarningMarkers = []string{"translated to sqlite", "sqlite semantics"}

// countDialectWarnings returns how many lines carry the warning.
func countDialectWarnings(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		lower := strings.ToLower(line)
		all := true
		for _, marker := range dialectWarningMarkers {
			if !strings.Contains(lower, marker) {
				all = false
				break
			}
		}
		if all {
			count++
		}
	}
	return count
}

// writeDialectFixture writes a two-row CSV and returns its path.
func writeDialectFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.csv")
	if err := os.WriteFile(path, []byte("id,name\n1,Alice\n2,Bob\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestDialectWarningBinary_SQLiteIsSilent(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	for _, args := range [][]string{
		{"--output-format", "csv", "--sql", "SELECT * FROM users", csv},
		{"--dialect", "sqlite", "--output-format", "csv", "--sql", "SELECT * FROM users", csv},
	} {
		_, stderr, code := run(t, "", args...)
		if code != 0 {
			t.Fatalf("%v exit = %d (stderr: %s)", args, code, stderr)
		}
		if got := countDialectWarnings(stderr); got != 0 {
			t.Errorf("%v printed %d warning(s), want 0:\n%s", args, got, stderr)
		}
	}
}

func TestDialectWarningBinary_EachNonSQLiteDialectWarnsOnce(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	for _, tc := range []struct{ flag, name string }{
		{"mysql", "MySQL"},
		{"postgresql", "PostgreSQL"},
		{"googlesql", "GoogleSQL"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			stdout, stderr, code := run(t, "",
				"--dialect", tc.flag, "--output-format", "csv", "--sql", "SELECT * FROM users", csv)
			if code != 0 {
				t.Fatalf("exit = %d (stderr: %s)", code, stderr)
			}
			if got := countDialectWarnings(stderr); got != 1 {
				t.Errorf("stderr carried %d warning(s), want exactly 1:\n%s", got, stderr)
			}
			if !strings.Contains(stderr, tc.name) {
				t.Errorf("stderr does not spell the dialect as %q:\n%s", tc.name, stderr)
			}
			if countDialectWarnings(stdout) != 0 {
				t.Errorf("the warning reached stdout:\n%s", stdout)
			}
			if lines := strings.Count(strings.TrimSpace(stdout), "\n"); lines != 2 {
				t.Errorf("csv stdout has %d newline(s), want 2 (header + 2 rows):\n%s", lines, stdout)
			}
		})
	}
}

func TestDialectWarningBinary_SeveralStatementsWarnOnce(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)
	sqlPath := filepath.Join(t.TempDir(), "many.sql")
	statements := strings.Repeat("SELECT COUNT(*) FROM users;\n", 4)
	if err := os.WriteFile(sqlPath, []byte(statements), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "", "--dialect", "postgresql", "--sql-file", sqlPath, csv)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("a four-statement --sql-file printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

func TestDialectWarningBinary_ScriptFileWarnsOnce(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)
	scriptPath := filepath.Join(t.TempDir(), "run.sqly")
	if err := os.WriteFile(scriptPath, []byte(".tables\nSELECT COUNT(*) FROM users;\nSELECT COUNT(*) FROM users;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, "", "--dialect", "mysql", "--script-file", scriptPath, csv)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("a --script-file printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

func TestDialectWarningBinary_StdinScriptWarnsOnce(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	_, stderr, code := run(t, "SELECT COUNT(*) FROM users;\nSELECT COUNT(*) FROM users;\n",
		"--dialect", "googlesql", csv)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("a piped script printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

// TestDialectWarningBinary_RepeatedSwitchesWarnOnce is the shell case: the
// dot-command path, driven through a piped script, switching four times.
func TestDialectWarningBinary_RepeatedSwitchesWarnOnce(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	script := ".dialect postgresql\n.dialect sqlite\n.dialect mysql\n.dialect googlesql\nSELECT COUNT(*) FROM users;\n"
	_, stderr, code := run(t, script, csv)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if got := countDialectWarnings(stderr); got != 1 {
		t.Errorf("four .dialect switches printed %d warning(s), want 1:\n%s", got, stderr)
	}
}

func TestDialectWarningBinary_MachineReadableStdoutStaysParseable(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	t.Run("json", func(t *testing.T) {
		stdout, stderr, code := run(t, "",
			"--dialect", "postgresql", "--output-format", "json", "--sql", "SELECT * FROM users", csv)
		if code != 0 {
			t.Fatalf("exit = %d (stderr: %s)", code, stderr)
		}
		var rows []map[string]any
		if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
			t.Fatalf("stdout is not a JSON array: %v\n%s", err, stdout)
		}
		if len(rows) != 2 {
			t.Errorf("rows = %d, want 2", len(rows))
		}
		if countDialectWarnings(stderr) != 1 {
			t.Errorf("stderr should carry the warning:\n%s", stderr)
		}
	})

	t.Run("jsonl", func(t *testing.T) {
		stdout, stderr, code := run(t, "",
			"--dialect", "mysql", "--output-format", "jsonl", "--sql", "SELECT * FROM users", csv)
		if code != 0 {
			t.Fatalf("exit = %d (stderr: %s)", code, stderr)
		}
		for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatalf("NDJSON line %q does not parse: %v", line, err)
			}
		}
	})
}

func TestDialectWarningBinary_InspectAndMetaFlagsAreSilent(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"--inspect", []string{"--dialect", "postgresql", "--inspect", csv}},
		{"--help", []string{"--dialect", "postgresql", "--help"}},
		{"--version", []string{"--dialect", "mysql", "--version"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(t, "", tc.args...)
			if code != 0 {
				t.Fatalf("exit = %d (stderr: %s)", code, stderr)
			}
			if got := countDialectWarnings(stderr); got != 0 {
				t.Errorf("%s printed %d warning(s), want 0:\n%s", tc.name, got, stderr)
			}
			if countDialectWarnings(stdout) != 0 {
				t.Errorf("%s printed the warning on stdout:\n%s", tc.name, stdout)
			}
		})
	}
}

func TestDialectWarningBinary_ARejectedCommandLineIsSilent(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	_, stderr, code := run(t, "", "--dialect", "postgresql", "--inspect", "--sql", "SELECT 1", csv)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	if got := countDialectWarnings(stderr); got != 0 {
		t.Errorf("a rejected command line printed %d warning(s), want 0:\n%s", got, stderr)
	}
}

// TestDialectWarningBinary_SQLiteSemanticsDecideTheAnswer is the claim behind
// the warning, checked rather than asserted: PostgreSQL's `||` on a number is a
// type error there and a string concatenation in SQLite, and SQLite is what
// answers.
func TestDialectWarningBinary_SQLiteSemanticsDecideTheAnswer(t *testing.T) {
	t.Parallel()
	csv := writeDialectFixture(t)

	stdout, stderr, code := run(t, "",
		"--dialect", "postgresql", "--output-format", "csv", "--sql", "SELECT 'x' || 1 AS c", csv)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "x1") {
		t.Errorf("stdout = %q, want SQLite's concatenation result", stdout)
	}
	if countDialectWarnings(stderr) != 1 {
		t.Errorf("the run that shows the divergence did not carry the warning that explains it:\n%s", stderr)
	}
}
