package shell

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inspectReportForTest mirrors the JSON contract produced by --inspect so tests
// can decode and assert it without depending on output formatting.
type inspectReportForTest struct {
	Tables []struct {
		Name     string `json:"name"`
		Source   string `json:"source"`
		RowCount int64  `json:"row_count"`
		Columns  []struct {
			Name       string `json:"name"`
			Type       string `json:"type"`
			Nullable   bool   `json:"nullable"`
			PrimaryKey bool   `json:"primary_key"`
		} `json:"columns"`
		SampleRows []map[string]string `json:"sample_rows"`
	} `json:"tables"`
}

// runInspectJSON builds a shell from args, runs it, and decodes the inspect JSON.
func runInspectJSON(t *testing.T, args []string) inspectReportForTest {
	t.Helper()
	shell, cleanup, err := newShell(t, args)
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()

	out := captureStdout(t, func() {
		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	var report inspectReportForTest
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("inspect output is not valid JSON: %v\n%s", err, out)
	}
	return report
}

func writeCSV(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestInspect_RejectsConflictingFlags(t *testing.T) {
	// Regression for: --inspect must reject other effectful flags instead
	// of silently discarding them.
	dir := t.TempDir()
	csv := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\n")

	cases := []struct {
		name string
		args []string
	}{
		{"with --sql", []string{"sqly", "--inspect", "--sql", "SELECT 1", csv}},
		{"with --output", []string{"sqly", "--inspect", "--output", filepath.Join(dir, "out.csv"), csv}},
		{"with --save-dir", []string{"sqly", "--inspect", "--save-dir", dir, csv}},
		{"with --save --force", []string{"sqly", "--inspect", "--save", "--force", csv}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shell, cleanup, err := newShell(t, tc.args)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			shell.isTTY = func() bool { return true }

			if err := shell.Run(context.Background()); err == nil {
				t.Fatalf("Run returned nil for --inspect %s, want a conflict error", tc.name)
			}
		})
	}
}

func TestInspect_SingleFile(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")

	report := runInspectJSON(t, []string{"sqly", "--inspect", csv})

	if len(report.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d: %+v", len(report.Tables), report.Tables)
	}
	tbl := report.Tables[0]
	if tbl.Name != "people" {
		t.Errorf("table name = %q, want people", tbl.Name)
	}
	if tbl.Source != csv {
		t.Errorf("source = %q, want %q", tbl.Source, csv)
	}
	if tbl.RowCount != 2 {
		t.Errorf("row_count = %d, want 2", tbl.RowCount)
	}
	if len(tbl.Columns) != 2 || tbl.Columns[0].Name != "name" || tbl.Columns[1].Name != "age" {
		t.Errorf("columns = %+v, want [name age]", tbl.Columns)
	}
	if len(tbl.SampleRows) != 2 {
		t.Fatalf("sample_rows = %d, want 2", len(tbl.SampleRows))
	}
	if tbl.SampleRows[0]["name"] != "Alice" {
		t.Errorf("first sample row name = %q, want Alice", tbl.SampleRows[0]["name"])
	}
}

func TestInspect_TypedJSONSampleRows(t *testing.T) {
	// --inspect --json-typed renders the report's sample rows with the typed
	// contract: a numeric column decodes to a native number, not a string.
	dir := t.TempDir()
	csv := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")

	shell, cleanup, err := newShell(t, []string{"sqly", "--inspect", "--json-typed", csv})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()

	out := captureStdout(t, func() {
		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	// Decode sample rows as any so the value types are observable.
	var report struct {
		Tables []struct {
			SampleRows []map[string]any `json:"sample_rows"`
		} `json:"tables"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("inspect output is not valid JSON: %v\n%s", err, out)
	}
	if len(report.Tables) != 1 || len(report.Tables[0].SampleRows) == 0 {
		t.Fatalf("expected sample rows, got %s", out)
	}
	row := report.Tables[0].SampleRows[0]
	if _, ok := row["age"].(float64); !ok {
		t.Errorf("expected numeric age in typed sample, got %#v (%s)", row["age"], out)
	}
	if name, ok := row["name"].(string); !ok || name == "" {
		t.Errorf("expected string name in typed sample, got %#v", row["name"])
	}
}

func TestInspect_RejectsPlainJSONButAllowsTyped(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\n")

	// Plain --json adds nothing to --inspect (already JSON) and is rejected.
	shellPlain, cleanupPlain, err := newShell(t, []string{"sqly", "--inspect", "--json", csv})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanupPlain()
	if err := shellPlain.Run(context.Background()); err == nil {
		t.Error("expected --inspect --json to be rejected, got nil")
	}

	// --json-typed is the meaningful opt-in and must be accepted.
	shellTyped, cleanupTyped, err := newShell(t, []string{"sqly", "--inspect", "--json-typed", csv})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanupTyped()
	if err := shellTyped.Run(context.Background()); err != nil {
		t.Errorf("expected --inspect --json-typed to be accepted, got %v", err)
	}
}

func TestInspect_SampleRowsAreLimited(t *testing.T) {
	dir := t.TempDir()
	// 10 rows; the sample must be capped below the row count.
	content := "id\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	csv := writeCSV(t, dir, "nums.csv", content)

	report := runInspectJSON(t, []string{"sqly", "--inspect", csv})

	if len(report.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(report.Tables))
	}
	tbl := report.Tables[0]
	if tbl.RowCount != 10 {
		t.Errorf("row_count = %d, want 10", tbl.RowCount)
	}
	if len(tbl.SampleRows) >= 10 {
		t.Errorf("sample_rows = %d, want a capped subset (< 10)", len(tbl.SampleRows))
	}
}

func TestInspect_SampleRowCountIsConfigurable(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "nums.csv", "id\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n")

	report := runInspectJSON(t, []string{"sqly", "--inspect", "--inspect-sample", "2", csv})

	if len(report.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(report.Tables))
	}
	if got := len(report.Tables[0].SampleRows); got != 2 {
		t.Errorf("sample_rows = %d, want 2", got)
	}
}

func TestInspect_SchemaOnlyWithZeroSample(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")

	report := runInspectJSON(t, []string{"sqly", "--inspect", "--inspect-sample", "0", csv})

	if len(report.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(report.Tables))
	}
	tbl := report.Tables[0]
	// Schema and counts are still present; only the sample is suppressed.
	if len(tbl.Columns) != 2 {
		t.Errorf("columns = %d, want 2 (schema must remain)", len(tbl.Columns))
	}
	if tbl.RowCount != 2 {
		t.Errorf("row_count = %d, want 2", tbl.RowCount)
	}
	if len(tbl.SampleRows) != 0 {
		t.Errorf("sample_rows = %d, want 0 (schema only)", len(tbl.SampleRows))
	}
}

func TestInspect_NegativeSampleErrors(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "x.csv", "a\n1\n")
	shell, cleanup, err := newShell(t, []string{"sqly", "--inspect", "--inspect-sample", "-1", csv})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.Run(context.Background()); err == nil {
		t.Fatal("expected an error for a negative --inspect-sample, got nil")
	}
}

func TestInspect_MultipleFileArgs(t *testing.T) {
	dir := t.TempDir()
	a := writeCSV(t, dir, "a.csv", "x\n1\n")
	b := writeCSV(t, dir, "b.csv", "y\n2\n")

	report := runInspectJSON(t, []string{"sqly", "--inspect", a, b})

	if len(report.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %+v", len(report.Tables), report.Tables)
	}
	bySource := map[string]string{}
	for _, tbl := range report.Tables {
		bySource[tbl.Name] = tbl.Source
	}
	if bySource["a"] != a {
		t.Errorf("table a source = %q, want %q", bySource["a"], a)
	}
	if bySource["b"] != b {
		t.Errorf("table b source = %q, want %q", bySource["b"], b)
	}
}

func TestInspect_Directory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "data")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	writeCSV(t, sub, "one.csv", "a\n1\n")
	writeCSV(t, sub, "two.csv", "b\n2\n")

	report := runInspectJSON(t, []string{"sqly", "--inspect", sub})

	if len(report.Tables) != 2 {
		t.Fatalf("expected 2 tables from directory, got %d", len(report.Tables))
	}
	// Each table reports its real source file, not the directory path.
	want := map[string]string{
		"one": filepath.Join(sub, "one.csv"),
		"two": filepath.Join(sub, "two.csv"),
	}
	for _, tbl := range report.Tables {
		if tbl.Source != want[tbl.Name] {
			t.Errorf("table %q source = %q, want file %q", tbl.Name, tbl.Source, want[tbl.Name])
		}
	}
}

func TestReportOnly_DirectoryImportQuietOnStderr(t *testing.T) {
	// Regression for #662: a successful directory import must not print its
	// progress banner to stderr in report-only modes (--inspect, --compare,
	// --profile). The structured report is the only intended output of a clean run.
	dir := t.TempDir()
	sub := filepath.Join(dir, "data")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	writeCSV(t, sub, "one.csv", "a\n1\n")
	writeCSV(t, sub, "two.csv", "b\n2\n")

	runCapturingStderr := func(t *testing.T, args []string) string {
		t.Helper()
		shell, cleanup, err := newShell(t, args)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		return captureStderr(t, func() {
			_ = captureStdout(t, func() {
				if err := shell.Run(context.Background()); err != nil {
					t.Fatalf("Run: %v", err)
				}
			})
		})
	}

	t.Run("--inspect stays quiet on a successful directory import", func(t *testing.T) {
		stderr := runCapturingStderr(t, []string{"sqly", "--inspect", sub})
		if strings.Contains(stderr, "Successfully imported") {
			t.Errorf("--inspect directory import should be quiet on stderr, got %q", stderr)
		}
	})

	t.Run("--profile stays quiet on a successful directory import", func(t *testing.T) {
		stderr := runCapturingStderr(t, []string{"sqly", "--profile", sub})
		if strings.Contains(stderr, "Successfully imported") {
			t.Errorf("--profile directory import should be quiet on stderr, got %q", stderr)
		}
	})

	t.Run("--compare stays quiet on a successful directory import", func(t *testing.T) {
		// The directory imports exactly two tables, so --compare needs no
		// --compare-tables to pick a left/right pair.
		stderr := runCapturingStderr(t, []string{"sqly", "--compare", sub})
		if strings.Contains(stderr, "Successfully imported") {
			t.Errorf("--compare directory import should be quiet on stderr, got %q", stderr)
		}
	})

	t.Run("a normal directory import still prints the banner on stderr", func(t *testing.T) {
		stderr := runCapturingStderr(t, []string{"sqly", "--csv", "--sql", "SELECT COUNT(*) FROM one", sub})
		if !strings.Contains(stderr, "Successfully imported") {
			t.Errorf("a non-report directory import should still print the banner, got %q", stderr)
		}
	})

	t.Run("--inspect still surfaces import warnings on stderr", func(t *testing.T) {
		wsub := filepath.Join(t.TempDir(), "data")
		if err := os.Mkdir(wsub, 0o750); err != nil {
			t.Fatal(err)
		}
		// A file named after a SQLite keyword produces a keyword table-name warning,
		// which must still print even though the success banner is suppressed.
		writeCSV(t, wsub, "select.csv", "a\n1\n")
		stderr := runCapturingStderr(t, []string{"sqly", "--inspect", wsub})
		if strings.Contains(stderr, "Successfully imported") {
			t.Errorf("--inspect directory import should be quiet on stderr, got %q", stderr)
		}
		if !strings.Contains(stderr, "warning") {
			t.Errorf("--inspect should still surface import warnings on stderr, got %q", stderr)
		}
	})
}

func TestWriteBack_RejectsDirectoryImport(t *testing.T) {
	// Regression for/: a directory-imported table reports its per-file
	// source in --inspect, but write-back must still reject it.
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	writeCSV(t, dir, "one.csv", "a\n1\n")
	if err := shell.commands.importCommand(context.Background(), shell, []string{dir}); err != nil {
		t.Fatalf("directory import failed: %v", err)
	}

	// Change the table so write-back actually considers it; an unchanged table is
	// skipped before the directory-import rejection. The change still must be
	// rejected because a directory import is not a single editable source.
	if err := shell.exec(context.Background(), "INSERT INTO one VALUES (2)"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if err := shell.writeBack(context.Background(), t.TempDir()); err == nil {
		t.Fatal("write-back of a directory-imported table returned nil, want rejection")
	}
}

func TestInspect_TablesSortedByName(t *testing.T) {
	dir := t.TempDir()
	z := writeCSV(t, dir, "zebra.csv", "a\n1\n")
	m := writeCSV(t, dir, "mango.csv", "b\n2\n")
	a := writeCSV(t, dir, "apple.csv", "c\n3\n")

	report := runInspectJSON(t, []string{"sqly", "--inspect", z, m, a})

	got := make([]string, 0, len(report.Tables))
	for _, tbl := range report.Tables {
		got = append(got, tbl.Name)
	}
	want := []string{"apple", "mango", "zebra"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("tables not sorted by name: got %v, want %v", got, want)
		}
	}
}

func TestInspect_NoInputErrors(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly", "--inspect"})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()

	if err := shell.Run(context.Background()); err == nil {
		t.Fatal("expected an error when --inspect is given no input, got nil")
	}
}
