package shell

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The contract --inspect makes to a program that runs it.
//
// The one at the centre is that a default run does not print the data. --inspect
// is the command an agent, a wrapper, or a CI job reaches for when it has been
// handed a file nobody has read yet, and "tell me what this is" must not answer
// with what is inside it. Every other assertion here — the sample cap, the two
// version fields, the empty array that is never absent, the schema the document
// validates against — exists so that promise is checkable rather than believed.
//
// The fixture below carries sentinels rather than plausible data, so a test that
// asks "did any row leak?" can answer by looking for a string that appears
// nowhere else in the document: not in a column name, not in a source path, not
// in a version.

// inspectSecretCSV holds two rows whose values cannot be confused with metadata.
const inspectSecretCSV = "id,note\n1,SENTINEL_FIRST_ROW\n2,SENTINEL_SECOND_ROW\n" //nolint:gosec // sentinel fixture data, not a credential

const (
	sentinelFirstRow  = "SENTINEL_FIRST_ROW"
	sentinelSecondRow = "SENTINEL_SECOND_ROW"
)

// writeSecretCSV writes the sentinel fixture and returns its path.
func writeSecretCSV(t *testing.T) string {
	t.Helper()
	return writeCSV(t, t.TempDir(), "secrets.csv", inspectSecretCSV)
}

func TestInspect_DefaultSampleIsZero(t *testing.T) {
	if config.DefaultInspectSample != 0 {
		t.Fatalf("config.DefaultInspectSample = %d, want 0: --inspect must be schema-only unless asked otherwise",
			config.DefaultInspectSample)
	}

	arg, err := config.NewArg([]string{"sqly", "--inspect", "data.csv"})
	if err != nil {
		t.Fatalf("NewArg: %v", err)
	}
	if arg.InspectSample != 0 {
		t.Errorf("parsed --inspect-sample = %d, want 0", arg.InspectSample)
	}
}

// TestInspect_DefaultPrintsNoRowData is the central claim: run --inspect over a
// file holding secrets and none of them reaches stdout.
func TestInspect_DefaultPrintsNoRowData(t *testing.T) {
	csv := writeSecretCSV(t)

	out := runInspectRaw(t, []string{"sqly", "--inspect", csv})

	for _, sentinel := range []string{sentinelFirstRow, sentinelSecondRow} {
		if strings.Contains(out, sentinel) {
			t.Errorf("default --inspect stdout contains %q; row data must not be printed unless --inspect-sample asks for it:\n%s",
				sentinel, out)
		}
	}

	var report inspectReportForTest
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	if len(report.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(report.Tables))
	}
	// Schema and counts survive; only the rows are gone. A report that dropped
	// the row count too would "pass" the leak check while being useless.
	if report.Tables[0].RowCount != 2 {
		t.Errorf("row_count = %d, want 2: the count is metadata, not row data", report.Tables[0].RowCount)
	}
	if len(report.Tables[0].Columns) != 2 {
		t.Errorf("columns = %d, want 2", len(report.Tables[0].Columns))
	}
	if report.Tables[0].SampleRows == nil {
		t.Error("sample_rows decoded as nil; it must be an empty array, not absent and not null")
	}
	if len(report.Tables[0].SampleRows) != 0 {
		t.Errorf("sample_rows = %d, want 0", len(report.Tables[0].SampleRows))
	}
	if !strings.Contains(out, `"sample_rows": []`) {
		t.Errorf("stdout does not carry an explicit empty sample_rows array:\n%s", out)
	}
}

// TestInspect_ExplicitSampleOnePrintsExactlyOneRow is the other half: asked for
// row data, --inspect gives exactly as much as was asked for, from a file that
// holds more.
func TestInspect_ExplicitSampleOnePrintsExactlyOneRow(t *testing.T) {
	csv := writeSecretCSV(t)

	out := runInspectRaw(t, []string{"sqly", "--inspect", "--inspect-sample", "1", csv})

	if !strings.Contains(out, sentinelFirstRow) {
		t.Errorf("--inspect-sample 1 stdout does not contain the first row:\n%s", out)
	}
	if strings.Contains(out, sentinelSecondRow) {
		t.Errorf("--inspect-sample 1 stdout contains the second row, so the cap is not applied:\n%s", out)
	}

	var report inspectReportForTest
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	if got := len(report.Tables[0].SampleRows); got != 1 {
		t.Fatalf("sample_rows = %d, want 1", got)
	}
}

// TestInspect_ZeroSampleMatchesTheDefault fixes the two spellings of the same
// request to one document, byte for byte. The same binary wrote both, so
// sqly_version is identical too and the whole output can be compared.
func TestInspect_ZeroSampleMatchesTheDefault(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")

	implicit := runInspectRaw(t, []string{"sqly", "--inspect", csv})
	explicit := runInspectRaw(t, []string{"sqly", "--inspect", "--inspect-sample", "0", csv})

	if implicit != explicit {
		t.Errorf("--inspect and --inspect-sample 0 produced different documents:\n--- default ---\n%s\n--- explicit 0 ---\n%s",
			implicit, explicit)
	}
}

// TestInspect_OutputIsDeterministic states what determinism means here: the same
// binary, the same input, and the same options produce the same bytes. It is
// deliberately not a claim across versions, because sqly_version moves.
func TestInspect_OutputIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	a := writeCSV(t, dir, "a.csv", "x\n1\n2\n")
	b := writeCSV(t, dir, "b.csv", "y\n3\n")

	first := runInspectRaw(t, []string{"sqly", "--inspect", "--inspect-sample", "2", b, a})
	second := runInspectRaw(t, []string{"sqly", "--inspect", "--inspect-sample", "2", b, a})

	if first != second {
		t.Errorf("two identical runs produced different documents:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestInspect_TopLevelFieldOrderIsFixed checks the document's field order, which
// is what makes the byte-for-byte comparisons above meaningful rather than
// accidental.
func TestInspect_TopLevelFieldOrderIsFixed(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	out := runInspectRaw(t, []string{"sqly", "--inspect", csv})

	want := []string{`"schema_version"`, `"sqly_version"`, `"tables"`}
	at := -1
	for _, key := range want {
		next := strings.Index(out, key)
		if next < 0 {
			t.Fatalf("stdout has no %s field:\n%s", key, out)
		}
		if next <= at {
			t.Fatalf("field %s is out of order in:\n%s", key, out)
		}
		at = next
	}
}

func TestInspect_SchemaVersionIsTheNumberOne(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	out := runInspectRaw(t, []string{"sqly", "--inspect", csv})

	// Decoded into `any`, a JSON number becomes float64 and a JSON string stays a
	// string, so this distinguishes 1 from "1" — which a struct field typed int
	// would not.
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	value, ok := raw["schema_version"].(float64)
	if !ok {
		t.Fatalf("schema_version is %T (%v), want a JSON number", raw["schema_version"], raw["schema_version"])
	}
	if int(value) != InspectSchemaVersion {
		t.Errorf("schema_version = %v, want %d", value, InspectSchemaVersion)
	}
}

func TestInspect_SqlyVersionIsNonEmptyAndMatchesTheBinary(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	// Version is the ldflags-set global a release binary carries. Setting it here
	// is the same path a release takes, so the report and --version cannot report
	// different things.
	restore := config.Version
	config.Version = "v9.9.9-test"
	defer func() { config.Version = restore }()

	var report inspectReportForTest
	out := runInspectRaw(t, []string{"sqly", "--inspect", csv})
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	if report.SqlyVersion == "" {
		t.Fatal("sqly_version is empty")
	}
	if report.SqlyVersion != config.GetVersion() {
		t.Errorf("sqly_version = %q, want %q (the version --version prints)", report.SqlyVersion, config.GetVersion())
	}
	if report.SqlyVersion != "v9.9.9-test" {
		t.Errorf("sqly_version = %q, want the ldflags version", report.SqlyVersion)
	}
}

// TestInspect_SqlyVersionIsNotAFixedTestValue guards the mutation of hard-coding
// the field: a development build must report whatever the existing version
// accessor says, not a literal chosen to make tests pass.
func TestInspect_SqlyVersionIsNotAFixedTestValue(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	restore := config.Version
	defer func() { config.Version = restore }()

	seen := make(map[string]string, 2)
	for _, version := range []string{"v1.2.3", "v4.5.6"} {
		config.Version = version
		var report inspectReportForTest
		out := runInspectRaw(t, []string{"sqly", "--inspect", csv})
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
		}
		seen[version] = report.SqlyVersion
	}
	if seen["v1.2.3"] == seen["v4.5.6"] {
		t.Errorf("sqly_version did not follow the binary version: both runs reported %q", seen["v1.2.3"])
	}
}

// TestInspect_ValidatesAgainstThePublishedSchema is the drift guard between the
// implementation and the file the website serves. The schema is loaded from the
// repository copy, which is the same bytes Hugo publishes.
func TestInspect_ValidatesAgainstThePublishedSchema(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")

	schema := compileInspectSchema(t)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"schema only", []string{"sqly", "--inspect", csv}},
		{"with a sample", []string{"sqly", "--inspect", "--inspect-sample", "2", csv}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := runInspectRaw(t, tc.args)
			assertValidatesAgainstInspectSchema(t, schema, out)
		})
	}
}

// inspectSchemaPath is the one canonical copy of the contract. There is no
// second copy in the repository to keep in step, which is the point.
func inspectSchemaPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Join(filepath.Dir(filepath.Dir(file)), "website", "static", "schema", "inspect-v1.schema.json")
}

// compileInspectSchema loads and compiles the published JSON Schema.
//
// Why a real validator rather than a hand-rolled check: a partial re-implementation
// of JSON Schema is a validator that agrees with the file for the cases someone
// thought of, which is exactly the drift this test exists to catch.
// santhosh-tekuri/jsonschema is used in tests only — no code sqly ships imports
// it — and it implements Draft 2020-12, the draft the schema declares.
func compileInspectSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	path := inspectSchemaPath(t)
	f, err := os.Open(path) //nolint:gosec // a repository fixture path
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	compiler := jsonschema.NewCompiler()
	const url = "https://nao1215.github.io/sqly/schema/inspect-v1.schema.json"
	if err := compiler.AddResource(url, doc); err != nil {
		t.Fatalf("add %s: %v", path, err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatalf("compile %s: %v", path, err)
	}
	return schema
}

// assertValidatesAgainstInspectSchema fails when the document does not satisfy
// the schema, quoting both so a failure says which field and which run.
func assertValidatesAgainstInspectSchema(t *testing.T, schema *jsonschema.Schema, document string) {
	t.Helper()

	value, err := jsonschema.UnmarshalJSON(strings.NewReader(document))
	if err != nil {
		t.Fatalf("inspect output is not valid JSON: %v\n%s", err, document)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatalf("inspect output does not satisfy the published schema: %v\n%s", err, document)
	}
}

// TestInspectSchema_RejectsDocumentsThatBreakTheContract proves the schema is
// load-bearing. A schema that accepts everything would let every test above
// pass while promising nothing, so each mutation here is one the contract names
// as breaking.
func TestInspectSchema_RejectsDocumentsThatBreakTheContract(t *testing.T) {
	schema := compileInspectSchema(t)

	valid := `{"schema_version":1,"sqly_version":"v1.0.0","tables":[` +
		`{"name":"t","source":"/tmp/t.csv","row_count":2,` +
		`"columns":[{"name":"a","type":"TEXT","nullable":true,"primary_key":false}],` +
		`"sample_rows":[]}]}`

	// The baseline has to pass, or the rejections below prove nothing.
	assertValidatesAgainstInspectSchema(t, schema, valid)

	for _, tc := range []struct {
		name     string
		document string
	}{
		{"schema_version missing", `{"sqly_version":"v1","tables":[]}`},
		{"schema_version as a string", `{"schema_version":"1","sqly_version":"v1","tables":[]}`},
		{"schema_version of another version", `{"schema_version":2,"sqly_version":"v1","tables":[]}`},
		{"sqly_version missing", `{"schema_version":1,"tables":[]}`},
		{"sqly_version empty", `{"schema_version":1,"sqly_version":"","tables":[]}`},
		{"tables missing", `{"schema_version":1,"sqly_version":"v1"}`},
		{"tables not an array", `{"schema_version":1,"sqly_version":"v1","tables":{}}`},
		{
			"sample_rows absent",
			`{"schema_version":1,"sqly_version":"v1","tables":[{"name":"t","row_count":0,"columns":[]}]}`,
		},
		{
			"sample_rows null",
			`{"schema_version":1,"sqly_version":"v1","tables":[{"name":"t","row_count":0,"columns":[],"sample_rows":null}]}`,
		},
		{
			"row_count as a string",
			`{"schema_version":1,"sqly_version":"v1","tables":[{"name":"t","row_count":"2","columns":[],"sample_rows":[]}]}`,
		},
		{
			"column missing nullable",
			`{"schema_version":1,"sqly_version":"v1","tables":[{"name":"t","row_count":0,` +
				`"columns":[{"name":"a","type":"TEXT","primary_key":false}],"sample_rows":[]}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.document))
			if err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}
			if err := schema.Validate(value); err == nil {
				t.Errorf("schema accepted %s, which the compatibility policy calls a breaking change", tc.name)
			}
		})
	}
}

// TestInspectSchema_AllowsAdditiveFields is the other side of the same coin. The
// policy promises that a new optional field does not move schema_version, so the
// schema must not close its objects.
func TestInspectSchema_AllowsAdditiveFields(t *testing.T) {
	schema := compileInspectSchema(t)

	withExtras := `{"schema_version":1,"sqly_version":"v1.0.0","future_top_level":{"x":1},"tables":[` +
		`{"name":"t","row_count":0,"columns":[` +
		`{"name":"a","type":"TEXT","nullable":true,"primary_key":false,"future_column_field":"x"}],` +
		`"sample_rows":[],"future_table_field":[1,2]}]}`

	assertValidatesAgainstInspectSchema(t, schema, withExtras)
}

// TestInspectSchema_AcceptsEverySampleValueTypeTheRendererProduces keeps the
// schema from being narrower than the implementation. The sample reuses sqly's
// query JSON renderer, which emits strings, numbers, and null, plus the three
// non-finite floats as strings; a schema that said "string" would reject a real
// report of an INTEGER column.
func TestInspectSchema_AcceptsEverySampleValueTypeTheRendererProduces(t *testing.T) {
	schema := compileInspectSchema(t)

	document := `{"schema_version":1,"sqly_version":"v1.0.0","tables":[{"name":"t","row_count":1,` +
		`"columns":[{"name":"a","type":"","nullable":true,"primary_key":false}],` +
		`"sample_rows":[{"text":"123","int":7,"real":1.5,"null":null,"bool":true,"nan":"NaN","inf":"-Infinity"}]}]}`

	assertValidatesAgainstInspectSchema(t, schema, document)
}

// TestInspect_ExcelSheetsContractSurvives keeps the workbook field additive: a
// run with no workbook omits it entirely, which is what lets a consumer of
// `tables` ignore it.
func TestInspect_ExcelSheetsContractSurvives(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")

	out := runInspectRaw(t, []string{"sqly", "--inspect", csv})

	if strings.Contains(out, "excel_sheets") {
		t.Errorf("a run with no workbook reported excel_sheets:\n%s", out)
	}
}

// TestInspect_FailureLeavesStdoutEmpty holds the half of the stdout contract
// that only shows up when something goes wrong: a failed --inspect must not
// leave a partial document behind for a consumer to parse.
func TestInspect_FailureLeavesStdoutEmpty(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly", "--inspect"})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	var runErr error
	out := captureStdout(t, func() {
		runErr = shell.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("--inspect with no input returned nil, want an error")
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("stdout = %q, want empty on a failed --inspect", out)
	}
}
