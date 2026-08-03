//go:build smoke

package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// removedFlag is one CLI option that v1.0.0-rc1 deleted. The table below is the
// complete list, and it exists so a future change that reintroduces one of these
// names — by restoring a flag, or by adding a new flag that happens to reuse a
// freed shorthand — fails here instead of silently resurrecting a surface the
// release notes say is gone.
type removedFlag struct {
	// category groups the flags by the workflow they belonged to, so a failure
	// names the feature rather than just a string.
	category string
	// flag is the exact argument a user would type.
	flag string
	// value, when non-empty, is passed as a separate argument, so a flag that
	// took a value is exercised the way it used to be written.
	value string
}

// removedFlags enumerates every option deleted for v1.0.0-rc1.
func removedFlags() []removedFlag {
	return []removedFlag{
		// Profile workflow.
		{category: "profile", flag: "--profile"},
		{category: "profile", flag: "--profile-format", value: "json"},
		{category: "profile", flag: "--profile-format", value: "text"},

		// Compare workflow.
		{category: "compare", flag: "--compare"},
		{category: "compare", flag: "--compare-key", value: "id"},
		{category: "compare", flag: "--compare-tables", value: "a,b"},
		{category: "compare", flag: "--compare-format", value: "json"},
		{category: "compare", flag: "--compare-format", value: "text"},

		// Typed JSON variants, replaced by type-preserving json/ndjson.
		{category: "typed-json", flag: "--json-typed"},
		{category: "typed-json", flag: "--ndjson-typed"},

		// Cache invalidation, now automatic from the content hash.
		{category: "cache-clear", flag: "--cache-clear"},

		// Long-form output aliases, replaced by --output-format.
		{category: "output-alias-long", flag: "--csv"},
		{category: "output-alias-long", flag: "--tsv"},
		{category: "output-alias-long", flag: "--ltsv"},
		{category: "output-alias-long", flag: "--json"},
		{category: "output-alias-long", flag: "--ndjson"},
		{category: "output-alias-long", flag: "--excel"},
		{category: "output-alias-long", flag: "--markdown"},
		{category: "output-alias-long", flag: "--parquet"},
		{category: "output-alias-long", flag: "--vertical"},

		// Short-form output aliases. These matter more than the long forms: a
		// freed single letter is easy to reassign by accident, and the old
		// meaning would then apply silently to a user's muscle memory.
		{category: "output-alias-short", flag: "-c"},
		{category: "output-alias-short", flag: "-t"},
		{category: "output-alias-short", flag: "-l"},
		{category: "output-alias-short", flag: "-j"},
		{category: "output-alias-short", flag: "-n"},
		{category: "output-alias-short", flag: "-e"},
		{category: "output-alias-short", flag: "-m"},
		{category: "output-alias-short", flag: "-p"},
	}
}

// TestRemovedSurface_FlagsRejected runs the real binary once per deleted flag
// and asserts the four things that make a removal safe: a non-zero exit, an
// explanation on stderr that names the flag, nothing on stdout that a pipeline
// could mistake for data, and no panic.
func TestRemovedSurface_FlagsRejected(t *testing.T) {
	csv := filepath.Join("testdata", "user.csv")

	for _, rf := range removedFlags() {
		name := rf.category + "/" + rf.flag
		if rf.value != "" {
			name += "=" + rf.value
		}
		t.Run(name, func(t *testing.T) {
			args := []string{rf.flag}
			if rf.value != "" {
				args = append(args, rf.value)
			}
			args = append(args, "--sql", "SELECT 1", csv)

			stdout, stderr, code := run(t, "", args...)
			assertRejected(t, rf.flag, stdout, stderr, code)
		})
	}
}

// TestRemovedSurface_FlagPositionDoesNotMatter checks that a deleted flag is
// rejected wherever it appears. sqly parses interspersed, so a flag written
// after the file path must not be swallowed as a positional argument and
// reported as a missing file — an error that would send the user looking for a
// path problem that does not exist.
func TestRemovedSurface_FlagPositionDoesNotMatter(t *testing.T) {
	csv := filepath.Join("testdata", "user.csv")

	positions := map[string][]string{
		"before everything": {"--csv", "--sql", "SELECT 1", csv},
		"between flags":     {"--sql", "SELECT 1", "--csv", csv},
		"after the path":    {"--sql", "SELECT 1", csv, "--csv"},
		"last argument":     {csv, "--sql", "SELECT 1", "--csv"},
	}
	for name, args := range positions {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code := run(t, "", args...)
			assertRejected(t, "--csv", stdout, stderr, code)
			if strings.Contains(strings.ToLower(stderr), "no such file") ||
				strings.Contains(strings.ToLower(stderr), "does not exist") {
				t.Errorf("a removed flag was misread as a path: stderr = %q", stderr)
			}
		})
	}
}

// TestRemovedSurface_ShellModesRejected covers the shell half of the removal.
// The typed JSON output modes were reachable as `.mode json-typed` as well as
// through a flag, so deleting only the flag would leave the feature alive for
// anyone who used the shell.
func TestRemovedSurface_ShellModesRejected(t *testing.T) {
	csv := filepath.Join("testdata", "user.csv")

	for _, mode := range []string{"json-typed", "ndjson-typed"} {
		t.Run(".mode "+mode, func(t *testing.T) {
			stdout, stderr, code := run(t, ".mode "+mode+"\nSELECT 1 AS n;\n", csv)
			if code == 0 {
				t.Fatalf(".mode %s exit code = 0, want non-zero (stdout=%q stderr=%q)", mode, stdout, stderr)
			}
			if !strings.Contains(stderr, "invalid output mode") {
				t.Errorf("stderr = %q, want it to reject the mode by name", stderr)
			}
			assertNoPanic(t, stdout, stderr)
			// The query after the rejected mode must not have produced output:
			// a partially applied session is worse than a rejected one.
			if strings.Contains(stdout, "1") {
				t.Errorf("stdout = %q, want no query output after a rejected mode", stdout)
			}
		})
	}
}

// TestRemovedSurface_RenamedImportModeValue pins the fill -> pad rename. The old
// value must fail rather than fall back to a default, because `fill` silently
// treated as `stop` would change which rows a script imports without saying so.
func TestRemovedSurface_RenamedImportModeValue(t *testing.T) {
	csv := filepath.Join("testdata", "user.csv")

	t.Run("--import-mode fill", func(t *testing.T) {
		stdout, stderr, code := run(t, "", "--import-mode", "fill", "--sql", "SELECT 1", csv)
		if code == 0 {
			t.Fatalf("--import-mode fill exit code = 0, want non-zero (stdout=%q)", stdout)
		}
		if !strings.Contains(stderr, "fill") {
			t.Errorf("stderr = %q, want it to name the rejected value", stderr)
		}
		assertNoPanic(t, stdout, stderr)
	})

	t.Run(".import-mode fill", func(t *testing.T) {
		stdout, stderr, code := run(t, ".import-mode fill\n", csv)
		if code == 0 {
			t.Fatalf(".import-mode fill exit code = 0, want non-zero (stdout=%q)", stdout)
		}
		assertNoPanic(t, stdout, stderr)
	})

	t.Run("pad is the replacement and works", func(t *testing.T) {
		_, stderr, code := run(t, "", "--import-mode", "pad", "--sql", "SELECT 1 AS n", "--output-format", "csv", csv)
		if code != 0 {
			t.Fatalf("--import-mode pad exit code = %d, want 0 (stderr=%q)", code, stderr)
		}
	})
}

// TestRemovedSurface_NotSuggestedByHelp checks that nothing still advertises the
// removed surface. Help text and the mode listing are where a user looks after a
// rejection, so a stale mention there sends them straight back to the flag that
// just failed.
func TestRemovedSurface_NotSuggestedByHelp(t *testing.T) {
	helpOut, _, code := run(t, "", "--help")
	if code != 0 {
		t.Fatalf("--help exit code = %d, want 0", code)
	}

	gone := []string{
		"--profile", "--compare", "--json-typed", "--ndjson-typed", "--cache-clear",
		"json-typed", "ndjson-typed", "compare-key", "compare-tables",
		"compare-format", "profile-format",
	}
	for _, token := range gone {
		if strings.Contains(helpOut, token) {
			t.Errorf("--help still advertises %q:\n%s", token, helpOut)
		}
	}
	// The long output aliases are gone too, but "--csv" as a substring would
	// also match nothing else, so check them with a word boundary of a space.
	for _, token := range []string{" --csv", " --tsv", " --ltsv", " --json ", " --ndjson", " --excel", " --markdown", " --parquet", " --vertical"} {
		if strings.Contains(helpOut, token) {
			t.Errorf("--help still advertises %q:\n%s", token, helpOut)
		}
	}
	if !strings.Contains(helpOut, "--output-format") {
		t.Errorf("--help does not document the replacement --output-format:\n%s", helpOut)
	}

	// The shell's own help must agree with the CLI help.
	csv := filepath.Join("testdata", "user.csv")
	shellHelp, _, code := run(t, ".help\n", csv)
	if code != 0 {
		t.Fatalf(".help exit code = %d, want 0", code)
	}

	// `.mode` with no argument prints the mode listing and fails the batch, so
	// both streams are inspected and the non-zero exit is expected.
	modeOut, modeErr, _ := run(t, ".mode\n", csv)
	combined := shellHelp + modeOut + modeErr
	for _, token := range []string{"json-typed", "ndjson-typed", ".compare", ".profile"} {
		if strings.Contains(combined, token) {
			t.Errorf(".help/.mode still advertises %q:\n%s", token, combined)
		}
	}
	// The listing must still be there — an empty one would pass the checks above
	// for the wrong reason.
	if !strings.Contains(combined, "ndjson") {
		t.Errorf(".mode listing did not print the surviving modes:\n%s", combined)
	}
}

// TestRemovedSurface_NoSubcommandsResurrected checks the neighbouring shape
// change: sqly is flag-driven, so a word that looks like a subcommand must be
// rejected with a pointer to the flag, not treated as a file path.
func TestRemovedSurface_NoSubcommandsResurrected(t *testing.T) {
	for _, word := range []string{"profile", "compare", "help", "version"} {
		t.Run(word, func(t *testing.T) {
			stdout, stderr, code := run(t, "", word)
			if code == 0 {
				t.Fatalf("positional %q exit code = 0, want non-zero (stdout=%q)", word, stdout)
			}
			assertNoPanic(t, stdout, stderr)
			if stdout != "" {
				t.Errorf("stdout = %q, want the diagnostic on stderr only", stdout)
			}
		})
	}
}

// assertRejected is the shared contract for a removed flag: a non-zero exit, a
// stderr message naming the flag, an empty stdout, and no panic.
func assertRejected(t *testing.T, flag, stdout, stderr string, code int) {
	t.Helper()

	if code == 0 {
		t.Fatalf("%s exit code = 0, want non-zero (stdout=%q stderr=%q)", flag, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("%s wrote to stdout: %q; a rejected run must keep stdout clean for pipelines", flag, stdout)
	}
	if stderr == "" {
		t.Fatalf("%s exited %d with no explanation on stderr", flag, code)
	}
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "unknown flag") && !strings.Contains(lower, "unknown shorthand") {
		t.Errorf("%s stderr = %q, want it to say the flag is unknown", flag, stderr)
	}
	trimmed := strings.TrimLeft(flag, "-")
	if !strings.Contains(stderr, trimmed) {
		t.Errorf("%s stderr = %q, want it to name the offending flag", flag, stderr)
	}
	assertNoPanic(t, stdout, stderr)
}

// assertNoPanic fails when either stream carries a Go panic or stack trace. A
// rejected flag is a user error, and a user error must not read like a crash.
func assertNoPanic(t *testing.T, stdout, stderr string) {
	t.Helper()
	for name, stream := range map[string]string{"stdout": stdout, "stderr": stderr} {
		for _, marker := range []string{"panic:", "goroutine ", "runtime error:", ".go:"} {
			if strings.Contains(stream, marker) {
				t.Errorf("%s contains %q, which looks like a crash rather than a diagnostic:\n%s", name, marker, stream)
			}
		}
	}
}
