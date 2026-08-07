//go:build smoke

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The --inspect contract, against the real binary, on Linux, macOS, and Windows.
//
// The unit tests in shell/ drive the same code in-process. What this adds is the
// process boundary: the actual stdout a caller pipes into jq, the actual exit
// code, and — for the version field — a binary built the way a release builds
// one, with the ldflags a tag carries.
//
// The central claim is the one about leakage: a default run over a file full of
// secrets must not put any of them on stdout.

const (
	inspectSentinelFirst  = "SENTINEL_FIRST_ROW"
	inspectSentinelSecond = "SENTINEL_SECOND_ROW"
	inspectSecretCSV      = "id,note\n1," + inspectSentinelFirst + "\n2," + inspectSentinelSecond + "\n"
)

// writeInspectFixture writes the sentinel CSV into a fresh directory and returns
// the directory and the file path.
func writeInspectFixture(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "secrets.csv")
	if err := os.WriteFile(path, []byte(inspectSecretCSV), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir, path
}

func TestInspectBinary_DefaultLeaksNoRowData(t *testing.T) {
	t.Parallel()
	_, csv := writeInspectFixture(t)

	stdout, stderr, code := run(t, "", "--inspect", csv)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, sentinel := range []string{inspectSentinelFirst, inspectSentinelSecond} {
		if strings.Contains(stdout, sentinel) {
			t.Errorf("default --inspect stdout contains %q:\n%s", sentinel, stdout)
		}
	}
	// stderr must not carry row data either, and must carry no part of the
	// report. It is not asserted empty here: this harness hands the child an
	// empty pipe on stdin, which produces the documented "standard input was not
	// read" notice. The empty-stderr case is covered by the atago specs, which
	// run with no pipe attached.
	for _, sentinel := range []string{inspectSentinelFirst, inspectSentinelSecond, `"tables"`} {
		if strings.Contains(stderr, sentinel) {
			t.Errorf("stderr contains %q; the report and its data belong on stdout only:\n%s", sentinel, stderr)
		}
	}
	if !strings.Contains(stdout, `"sample_rows": []`) {
		t.Errorf("stdout does not carry an explicit empty sample_rows array:\n%s", stdout)
	}
}

func TestInspectBinary_ExplicitSampleYieldsExactlyThatMany(t *testing.T) {
	t.Parallel()
	_, csv := writeInspectFixture(t)

	stdout, stderr, code := run(t, "", "--inspect", "--inspect-sample", "1", csv)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, inspectSentinelFirst) {
		t.Errorf("--inspect-sample 1 stdout lacks the first row:\n%s", stdout)
	}
	if strings.Contains(stdout, inspectSentinelSecond) {
		t.Errorf("--inspect-sample 1 stdout contains the second row, so the cap did not hold:\n%s", stdout)
	}
}

func TestInspectBinary_ZeroSampleMatchesTheDefaultByteForByte(t *testing.T) {
	t.Parallel()
	_, csv := writeInspectFixture(t)

	implicit, _, code := run(t, "", "--inspect", csv)
	if code != 0 {
		t.Fatalf("default --inspect exit = %d", code)
	}
	explicit, _, code := run(t, "", "--inspect", "--inspect-sample", "0", csv)
	if code != 0 {
		t.Fatalf("--inspect-sample 0 exit = %d", code)
	}
	if implicit != explicit {
		t.Errorf("--inspect and --inspect-sample 0 differ:\n--- default ---\n%s\n--- explicit ---\n%s", implicit, explicit)
	}
}

func TestInspectBinary_RejectsBadSampleCounts(t *testing.T) {
	t.Parallel()
	_, csv := writeInspectFixture(t)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"without --inspect", []string{"--inspect-sample", "1", csv}},
		{"negative", []string{"--inspect", "--inspect-sample", "-1", csv}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(t, "", tc.args...)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (stderr: %s)", code, stderr)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "inspect-sample") {
				t.Errorf("stderr does not name the flag:\n%s", stderr)
			}
		})
	}
}

func TestInspectBinary_TopLevelFieldsAndTypes(t *testing.T) {
	t.Parallel()
	_, csv := writeInspectFixture(t)

	stdout, stderr, code := run(t, "", "--inspect", csv)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout)
	}
	version, ok := raw["schema_version"].(float64)
	if !ok {
		t.Fatalf("schema_version is %T (%v), want a JSON number", raw["schema_version"], raw["schema_version"])
	}
	if version != 1 {
		t.Errorf("schema_version = %v, want 1", version)
	}
	sqlyVersion, ok := raw["sqly_version"].(string)
	if !ok || sqlyVersion == "" {
		t.Errorf("sqly_version = %v, want a non-empty string", raw["sqly_version"])
	}
	tables, ok := raw["tables"].([]any)
	if !ok {
		t.Fatalf("tables is %T, want an array", raw["tables"])
	}
	if len(tables) == 0 {
		t.Fatalf("tables is empty, want the one table the fixture produces:\n%s", stdout)
	}
	table, ok := tables[0].(map[string]any)
	if !ok {
		t.Fatalf("tables[0] is %T, want an object", tables[0])
	}
	sample, ok := table["sample_rows"].([]any)
	if !ok {
		t.Fatalf("sample_rows is %T, want an array (never null, never absent)", table["sample_rows"])
	}
	if len(sample) != 0 {
		t.Errorf("sample_rows = %v, want empty by default", sample)
	}
	// A run with no workbook must not carry the additive field at all.
	if _, present := raw["excel_sheets"]; present {
		t.Errorf("excel_sheets is present for a run with no workbook:\n%s", stdout)
	}
}

// TestInspectBinary_ReportedVersionMatchesTheVersionFlag ties the two together
// through the same process, so a binary cannot print one version and report
// another.
func TestInspectBinary_ReportedVersionMatchesTheVersionFlag(t *testing.T) {
	t.Parallel()
	_, csv := writeInspectFixture(t)

	printed, _, code := run(t, "", "--version")
	if code != 0 {
		t.Fatalf("--version exit = %d", code)
	}
	want := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(printed), "sqly "))

	stdout, _, code := run(t, "", "--inspect", csv)
	if code != 0 {
		t.Fatalf("--inspect exit = %d", code)
	}
	var report struct {
		SQLyVersion string `json:"sqly_version"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout)
	}
	if report.SQLyVersion != want {
		t.Errorf("sqly_version = %q, --version prints %q", report.SQLyVersion, want)
	}
}

// TestInspectBinary_ReleaseLdflagsVersionIsReported builds sqly the way a
// release builds it — with the version stamped into config.Version — and checks
// the report carries that exact tag. A development build reports "(devel)" or
// the module version instead, which is the case the other tests cover; what has
// to hold here is that a released binary's report names the release.
func TestInspectBinary_ReleaseLdflagsVersionIsReported(t *testing.T) {
	t.Parallel()

	const wantVersion = "v9.9.9-smoke"
	dir := t.TempDir()
	bin := filepath.Join(dir, "sqly-release")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/nao1215/sqly/config.Version="+wantVersion,
		"-o", bin, ".")
	build.Dir = repoRoot()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build a release-style binary: %v\n%s", err, out)
	}

	_, csv := writeInspectFixture(t)
	home := t.TempDir()
	cmd := exec.Command(bin, "--inspect", csv)
	cmd.Dir = repoRoot()
	cmd.Env = append(os.Environ(),
		"HOME="+home, "USERPROFILE="+home,
		"SQLY_HISTORY_PATH="+filepath.Join(home, "history"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run the release-style binary: %v", err)
	}

	var report struct {
		SchemaVersion int    `json:"schema_version"`
		SQLyVersion   string `json:"sqly_version"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if report.SQLyVersion != wantVersion {
		t.Errorf("sqly_version = %q, want the ldflags version %q", report.SQLyVersion, wantVersion)
	}
	if report.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", report.SchemaVersion)
	}
}

// TestInspectBinary_ValidatesAgainstThePublishedSchema is the drift guard at the
// process boundary: whatever the shipped binary writes must satisfy the schema
// the website publishes.
func TestInspectBinary_ValidatesAgainstThePublishedSchema(t *testing.T) {
	t.Parallel()
	dir, csv := writeInspectFixture(t)

	// A second table with a mix of value types, so the sample exercises more of
	// the schema than one column of text would.
	mixed := filepath.Join(dir, "mixed.csv")
	if err := os.WriteFile(mixed, []byte("n,f,t\n1,1.5,abc\n2,2.5,00123\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	schemaPath := filepath.Join(repoRoot(), "website", "static", "schema", "inspect-v1.schema.json")
	f, err := os.Open(schemaPath) //nolint:gosec // a repository fixture path
	if err != nil {
		t.Fatalf("open %s: %v", schemaPath, err)
	}
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("parse %s: %v", schemaPath, err)
	}
	compiler := jsonschema.NewCompiler()
	const url = "https://nao1215.github.io/sqly/schema/inspect-v1.schema.json"
	if err := compiler.AddResource(url, doc); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"schema only", []string{"--inspect", csv, mixed}},
		{"with a sample", []string{"--inspect", "--inspect-sample", "2", csv, mixed}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(t, "", tc.args...)
			if code != 0 {
				t.Fatalf("exit = %d (stderr: %s)", code, stderr)
			}
			value, err := jsonschema.UnmarshalJSON(strings.NewReader(stdout))
			if err != nil {
				t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
			}
			if err := schema.Validate(value); err != nil {
				t.Fatalf("the binary's output does not satisfy the published schema: %v\n%s", err, stdout)
			}
		})
	}
}

// TestInspectBinary_FailureLeavesStdoutEmpty holds the other half of the stdout
// contract.
func TestInspectBinary_FailureLeavesStdoutEmpty(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := run(t, "", "--inspect", filepath.Join(t.TempDir(), "missing.csv"))
	if code == 0 {
		t.Fatalf("exit = 0 for a missing input, want non-zero (stderr: %s)", stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty on a failed --inspect", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr is empty; a failure must say what went wrong")
	}
}
