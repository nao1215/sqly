package shell

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	for _, tbl := range report.Tables {
		if tbl.Source != sub {
			t.Errorf("table %q source = %q, want directory %q", tbl.Name, tbl.Source, sub)
		}
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
