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
	"compress/gzip"
	"encoding/json"
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
	for _, want := range []string{
		"Documentation: https://nao1215.github.io/sqly/",
		"GitHub Sponsors: https://github.com/sponsors/nao1215",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--help stdout does not contain %q: %q", want, out)
		}
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

	out, _, code := run(t, "", "--output-format", "csv", "--sql", "SELECT first_name FROM user ORDER BY first_name LIMIT 1", csv)
	if code != 0 {
		t.Fatalf("--output-format csv --sql exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "first_name") {
		t.Errorf("--output-format csv stdout = %q, want a csv header", out)
	}

	out, _, code = run(t, "", "--output-format", "json", "--sql", "SELECT first_name FROM user ORDER BY first_name LIMIT 1", csv)
	if code != 0 {
		t.Fatalf("--output-format json --sql exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "first_name") || !strings.Contains(out, "[") {
		t.Errorf("--output-format json stdout = %q, want a JSON array", out)
	}

	resultPath := filepath.Join(t.TempDir(), "typed.json")
	_, _, code = run(t, "", "--output-format", "json", "--output", resultPath, "--sql", "SELECT 42 AS integer_value, 1.5 AS real_value, '123' AS text_number, 'true' AS text_bool, '00123' AS padded, NULL AS null_value", csv)
	if code != 0 {
		t.Fatalf("JSON --output exit code = %d, want 0", code)
	}
	data, err := os.ReadFile(resultPath) //nolint:gosec // test reads a path it just wrote
	if err != nil {
		t.Fatalf("read JSON output file: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("decode JSON output file: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("decoded %d JSON rows, want 1", len(rows))
	}
	row := rows[0]
	if row["integer_value"] != float64(42) || row["real_value"] != 1.5 {
		t.Errorf("numeric JSON values = %#v, want native numbers", row)
	}
	for _, name := range []string{"text_number", "text_bool", "padded"} {
		if _, ok := row[name].(string); !ok {
			t.Errorf("%s = %#v (%T), want string", name, row[name], row[name])
		}
	}
	if row["text_number"] != "123" || row["text_bool"] != "true" || row["padded"] != "00123" || row["null_value"] != nil {
		t.Errorf("typed JSON boundary values changed: %#v", row)
	}
}

func TestSmoke_DumpJSONAndNDJSONPreserveSQLiteTypes(t *testing.T) {
	input := "CREATE TABLE typed_values (integer_value INTEGER, real_value REAL, text_value TEXT, null_value TEXT);\n" +
		"INSERT INTO typed_values VALUES (42, 1.5, '123', NULL);\n"
	for _, tc := range []struct {
		name string
		ext  string
		mode string
		nd   bool
	}{
		{name: "json", ext: ".json", mode: "json"},
		{name: "ndjson", ext: ".ndjson", mode: "ndjson", nd: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "typed"+tc.ext)
			stdin := input + ".mode " + tc.mode + "\n.dump typed_values " + path + "\n"
			_, stderr, code := run(t, stdin)
			if code != 0 {
				t.Fatalf(".dump %s exit code = %d (stderr=%q)", tc.name, code, stderr)
			}
			data, err := os.ReadFile(path) //nolint:gosec // test reads a path it just wrote
			if err != nil {
				t.Fatal(err)
			}
			var row map[string]any
			if tc.nd {
				if err := json.Unmarshal(data, &row); err != nil {
					t.Fatalf("decode NDJSON: %v; data=%q", err, data)
				}
			} else {
				var rows []map[string]any
				if err := json.Unmarshal(data, &rows); err != nil {
					t.Fatalf("decode JSON: %v; data=%q", err, data)
				}
				if len(rows) != 1 {
					t.Fatalf("decoded %d rows, want 1", len(rows))
				}
				row = rows[0]
			}
			if _, ok := row["integer_value"].(float64); !ok {
				t.Errorf("integer_value = %#v (%T), want JSON number", row["integer_value"], row["integer_value"])
			}
			if _, ok := row["real_value"].(float64); !ok {
				t.Errorf("real_value = %#v (%T), want JSON number", row["real_value"], row["real_value"])
			}
			if row["text_value"] != "123" || row["null_value"] != nil {
				t.Errorf("dump values = %#v, want TEXT string and JSON null", row)
			}
		})
	}
}

func writeSmokeGzip(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // test path is temporary
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(f)
	if _, err := writer.Write([]byte(content)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSmoke_StdinDataset(t *testing.T) {
	out, _, code := run(t, "id,name\n1,alice\n2,bob\n", "--stdin", "csv", "--output-format", "csv", "--sql", "SELECT COUNT(*) AS c FROM stdin")
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
	stdout, stderr, code := run(t, "", "--output-format", "csv", "--sql", "SELECT first_name FROM user LIMIT 1", "--output", outPath, filepath.Join("testdata", "user.csv"))
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

// TestSmoke_AtomicPadAndEmptyJSON exercises the user-facing binary for the
// import cases that must be handled in the same streaming load. The output
// file is also checked: a failed multi-file import must not expose a partial
// query result or a partially loaded SQLite session.
func TestSmoke_AtomicPadAndEmptyJSON(t *testing.T) {
	dir := t.TempDir()
	shortCSV := filepath.Join(dir, "short.csv")
	longCSV := filepath.Join(dir, "long.csv")
	shortTSV := filepath.Join(dir, "short.tsv")
	longTSV := filepath.Join(dir, "long.tsv")
	for path, content := range map[string]string{
		shortCSV: "id,name\n1,alice\n2\n",
		longCSV:  "id,name\n1,alice,unexpected\n",
		shortTSV: "id\tname\n1\talice\n2\n",
		longTSV:  "id\tname\n1\talice\textra\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{name: "csv", path: shortCSV, want: "alice"},
		{name: "tsv", path: shortTSV, want: "alice"},
	} {
		t.Run(tc.name+" short rows are padded", func(t *testing.T) {
			out, stderr, code := run(t, "", "--import-mode", "pad", "--output-format", "csv", "--sql", "SELECT name FROM short ORDER BY id", tc.path)
			if code != 0 || !strings.Contains(out, tc.want) {
				t.Fatalf("short %s import: code=%d stdout=%q stderr=%q", tc.name, code, out, stderr)
			}
		})
	}

	compressedShort := filepath.Join(dir, "compressed.csv.gz")
	compressedLong := filepath.Join(dir, "compressed-long.csv.gz")
	writeSmokeGzip(t, compressedShort, "id,name\n1,alice\n2\n")
	writeSmokeGzip(t, compressedLong, "id,name\n1,alice,unexpected\n")
	out, stderr, code := run(t, "", "--import-mode", "pad", "--output-format", "csv", "--sql", "SELECT name FROM compressed ORDER BY id", compressedShort)
	if code != 0 || !strings.Contains(out, "alice") {
		t.Fatalf("gzip short rows: code=%d stdout=%q stderr=%q", code, out, stderr)
	}
	_, stderr, code = run(t, "", "--import-mode", "pad", "--output-format", "csv", "--sql", "SELECT 1", compressedLong)
	if code == 0 || !strings.Contains(stderr, "refuses to truncate") {
		t.Fatalf("gzip long row: code=%d stderr=%q", code, stderr)
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "csv", path: longCSV},
		{name: "tsv", path: longTSV},
	} {
		t.Run(tc.name+" long rows rollback", func(t *testing.T) {
			result := filepath.Join(dir, tc.name+"-result.csv")
			_, stderr, code := run(t, "", "--import-mode", "pad", "--output", result, "--output-format", "csv", "--sql", "SELECT 1", tc.path)
			if code == 0 || !strings.Contains(stderr, "refuses to truncate") {
				t.Fatalf("long %s import: code=%d stderr=%q", tc.name, code, stderr)
			}
			if _, err := os.Stat(result); !os.IsNotExist(err) {
				t.Fatalf("long %s import created output %s, stat err=%v", tc.name, result, err)
			}
		})
	}

	// A valid file must not survive beside a long-row file in the same atomic
	// import, and an already-existing table must keep its original rows.
	validCSV := filepath.Join(dir, "valid.csv")
	if err := os.WriteFile(validCSV, []byte("id,name\n1,alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mixedResult := filepath.Join(dir, "mixed-result.csv")
	_, stderr, code = run(t, "", "--import-mode", "pad", "--output", mixedResult, "--output-format", "csv", "--sql", "SELECT COUNT(*) AS c FROM valid", validCSV, longCSV)
	if code == 0 || !strings.Contains(stderr, "refuses to truncate") {
		t.Fatalf("valid + long CSV import: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(mixedResult); !os.IsNotExist(err) {
		t.Fatalf("valid + long CSV import created output, stat err=%v", err)
	}
	emptyJSON := filepath.Join(dir, "empty.json")
	badJSON := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(emptyJSON, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badJSON, []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	out, stderr, code = run(t, "", "--output-format", "csv", "--sql", "SELECT COUNT(*) AS c FROM empty", emptyJSON)
	if code != 0 || !strings.Contains(out, "c") || !strings.Contains(out, "0") {
		t.Fatalf("empty JSON import: code=%d stdout=%q stderr=%q", code, out, stderr)
	}
	result := filepath.Join(dir, "empty-result.csv")
	_, stderr, code = run(t, "", "--output", result, "--output-format", "csv", "--sql", "SELECT 1", emptyJSON, badJSON)
	if code == 0 || !strings.Contains(stderr, "failed") {
		t.Fatalf("empty JSON rollback: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(result); !os.IsNotExist(err) {
		t.Fatalf("empty JSON rollback created output, stat err=%v", err)
	}

	datasetDir := filepath.Join(dir, "dataset")
	if err := os.Mkdir(datasetDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"valid.csv":    "id\n1\n",
		"long.csv":     "id\n1\n2\n",
		"valid.json":   `[{"id":1}]`,
		"invalid.json": "[",
	} {
		path := filepath.Join(datasetDir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The CSV long-row file is intentionally made longer than its header after
	// creation, so the directory contains both valid and invalid data sources.
	if err := os.WriteFile(filepath.Join(datasetDir, "long.csv"), []byte("id\n1,extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirResult := filepath.Join(dir, "directory-result.csv")
	_, stderr, code = run(t, "", "--import-mode", "pad", "--output", dirResult, "--output-format", "csv", "--sql", "SELECT 1", datasetDir)
	if code == 0 || !strings.Contains(stderr, "failed") {
		t.Fatalf("directory import: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(dirResult); !os.IsNotExist(err) {
		t.Fatalf("directory rollback created output, stat err=%v", err)
	}
}
