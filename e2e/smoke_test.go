//go:build smoke

// Package e2e holds binary-level smoke tests that build the real sqly binary and
// drive it the way a user does (flags, piped stdin, exit codes). Unlike the
// ShellSpec suite, this harness is pure Go, so it runs identically on Linux,
// macOS, and Windows and gives Windows binary-level coverage that shell-based
// tests cannot. It is gated behind the "smoke" build tag so it does not run in
// the normal `go test ./...` unit pass.
package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sqlyBin is the path to the binary built once for the whole smoke run.
var sqlyBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "sqly-smoke-")
	if err != nil {
		panic("create temp dir: " + err.Error())
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "sqly")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = repoRoot()
	if out, err := build.CombinedOutput(); err != nil {
		panic("build sqly: " + err.Error() + "\n" + string(out))
	}
	sqlyBin = bin

	os.Exit(m.Run())
}

// repoRoot returns the repository root (the parent of this e2e directory), so the
// build picks up the module main package regardless of the working directory.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(file))
}

// run executes the built sqly binary with stdin and arguments, returning stdout,
// stderr, and the process exit code. It isolates HOME and the history DB into a
// per-test temp directory so the smoke run never touches real config state.
func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(sqlyBin, args...)
	cmd.Dir = repoRoot()
	cmd.Stdin = strings.NewReader(stdin)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"USERPROFILE="+home,
		"SQLY_HISTORY_DB_PATH="+filepath.Join(home, "history.db"),
	)
	err := cmd.Run()
	code = 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run sqly %v: %v", args, err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

func TestSmoke_VersionAndHelpFlags(t *testing.T) {
	out, _, code := run(t, "", "--version")
	if code != 0 {
		t.Fatalf("--version exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "sqly") {
		t.Errorf("--version stdout = %q, want it to mention sqly", out)
	}

	out, _, code = run(t, "", "--help")
	if code != 0 {
		t.Fatalf("--help exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Usage") {
		t.Errorf("--help stdout = %q, want usage text", out)
	}
}

func TestSmoke_BatchHelperCommands(t *testing.T) {
	out, _, code := run(t, ".pwd\n.mode csv\nSELECT 1 AS one;\n", filepath.Join("testdata", "user.csv"))
	if code != 0 {
		t.Fatalf("batch helper run exit code = %d, want 0 (stdout=%q)", code, out)
	}
	if !strings.Contains(out, "one") || !strings.Contains(out, "1") {
		t.Errorf("batch stdout = %q, want the csv query result", out)
	}
}

func TestSmoke_MissingHelperArgFailsBatch(t *testing.T) {
	_, stderr, code := run(t, ".schema\nSELECT 1;\n", filepath.Join("testdata", "user.csv"))
	if code == 0 {
		t.Fatalf(".schema with no argument should fail the batch run, got exit 0 (stderr=%q)", stderr)
	}
	if !strings.Contains(stderr, ".schema requires") {
		t.Errorf("stderr = %q, want it to mention the missing argument", stderr)
	}
}

func TestSmoke_DirectSQLOutputFormats(t *testing.T) {
	csv := filepath.Join("testdata", "user.csv")

	out, _, code := run(t, "", "--csv", "--sql", "SELECT first_name FROM user ORDER BY first_name LIMIT 1", csv)
	if code != 0 {
		t.Fatalf("--csv --sql exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "first_name") {
		t.Errorf("--csv stdout = %q, want a csv header", out)
	}

	out, _, code = run(t, "", "--json", "--sql", "SELECT first_name FROM user ORDER BY first_name LIMIT 1", csv)
	if code != 0 {
		t.Fatalf("--json --sql exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "first_name") || !strings.Contains(out, "[") {
		t.Errorf("--json stdout = %q, want a JSON array", out)
	}
}

func TestSmoke_StdinDataset(t *testing.T) {
	out, _, code := run(t, "id,name\n1,alice\n2,bob\n", "--stdin", "csv", "--csv", "--sql", "SELECT COUNT(*) AS c FROM stdin")
	if code != 0 {
		t.Fatalf("--stdin csv exit code = %d, want 0 (stdout=%q)", code, out)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("--stdin csv stdout = %q, want the piped row count", out)
	}
}

func TestSmoke_OutputToFileAndStderrSeparation(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "result.csv")
	stdout, stderr, code := run(t, "", "--csv", "--sql", "SELECT first_name FROM user LIMIT 1", "--output", outPath, filepath.Join("testdata", "user.csv"))
	if code != 0 {
		t.Fatalf("--output exit code = %d, want 0 (stderr=%q)", code, stderr)
	}
	// The data goes to the file; stdout stays empty and progress goes to stderr.
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("--output stdout = %q, want it empty (data went to the file)", stdout)
	}
	data, err := os.ReadFile(outPath) //nolint:gosec // test reads a path it just wrote
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(data), "first_name") {
		t.Errorf("output file = %q, want the csv result", string(data))
	}
}

func TestSmoke_PositionalSubcommandHint(t *testing.T) {
	_, stderr, code := run(t, "", "help")
	if code == 0 {
		t.Fatal("`sqly help` should fail with a hint, got exit 0")
	}
	if !strings.Contains(stderr, "--help") || !strings.Contains(stderr, "no subcommands") {
		t.Errorf("stderr = %q, want a flag-driven hint", stderr)
	}
}

func TestSmoke_CdAndImportWithSpacePath(t *testing.T) {
	// A directory whose name contains a space exercises path handling that differs
	// across platforms (especially Windows).
	base := t.TempDir()
	spaceDir := filepath.Join(base, "my data")
	if err := os.Mkdir(spaceDir, 0o750); err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(spaceDir, "rows.csv")
	if err := os.WriteFile(csvPath, []byte("id,name\n1,alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// .import with a quoted space-containing path, then query the imported table.
	script := ".import \"" + csvPath + "\"\n.mode csv\nSELECT COUNT(*) AS c FROM rows;\n"
	out, stderr, code := run(t, script, filepath.Join("testdata", "user.csv"))
	if code != 0 {
		t.Fatalf("space-path import exit code = %d, want 0 (stderr=%q)", code, stderr)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("stdout = %q, want the imported row count", out)
	}
}
