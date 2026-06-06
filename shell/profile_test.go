package shell

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileColumnStats(t *testing.T) {
	t.Parallel()

	t.Run("counts nulls, blanks, distinct, and numeric values", func(t *testing.T) {
		t.Parallel()
		values := []string{"1", "2", "", "2", "x"}
		nulls := []bool{false, false, false, false, false}
		// Mark index 4 ("x") as a real NULL to separate null from blank.
		nulls[4] = true
		got := profileColumnStats("c", "TEXT", values, nulls)
		if got.NullCount != 1 {
			t.Errorf("null_count = %d, want 1", got.NullCount)
		}
		if got.BlankCount != 1 {
			t.Errorf("blank_count = %d, want 1", got.BlankCount)
		}
		// Non-null, non-blank values are "1","2","2" -> distinct {1,2} = 2.
		if got.DistinctCount != 2 {
			t.Errorf("distinct_count = %d, want 2", got.DistinctCount)
		}
		if got.NumericCount != 3 {
			t.Errorf("numeric_count = %d, want 3", got.NumericCount)
		}
	})

	t.Run("warns on a mix of numeric and non-numeric values", func(t *testing.T) {
		t.Parallel()
		got := profileColumnStats("c", "TEXT", []string{"1", "2", "abc"}, []bool{false, false, false})
		if !hasWarningContaining(got.Warnings, "mixed numeric") {
			t.Errorf("expected a mixed-type warning, got %v", got.Warnings)
		}
	})

	t.Run("warns on null-like placeholder text and surrounding whitespace", func(t *testing.T) {
		t.Parallel()
		got := profileColumnStats("c", "TEXT", []string{"N/A", " hi "}, []bool{false, false})
		if !hasWarningContaining(got.Warnings, "null placeholders") {
			t.Errorf("expected a null-placeholder warning, got %v", got.Warnings)
		}
		if !hasWarningContaining(got.Warnings, "whitespace") {
			t.Errorf("expected a whitespace warning, got %v", got.Warnings)
		}
	})

	t.Run("clean numeric column has no warnings", func(t *testing.T) {
		t.Parallel()
		got := profileColumnStats("c", "INTEGER", []string{"1", "2", "3"}, []bool{false, false, false})
		if len(got.Warnings) != 0 {
			t.Errorf("expected no warnings, got %v", got.Warnings)
		}
	})
}

func hasWarningContaining(warnings []string, sub string) bool {
	for _, w := range warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func TestIsNumericValue(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"1": true, "-2.5": true, "1e3": true, "0": true,
		"abc": false, "": false, "NaN": false, "Inf": false, "1e400": false, "1,000": false,
	}
	for in, want := range cases {
		if got := isNumericValue(in); got != want {
			t.Errorf("isNumericValue(%q) = %v, want %v", in, got, want)
		}
	}
}

// runProfileJSON builds a shell from args, runs it, and decodes the JSON report.
func runProfileJSON(t *testing.T, args []string) profileReport {
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
	var report profileReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("profile output is not valid JSON: %v\n%s", err, out)
	}
	return report
}

func TestRunProfile_MessyDataJSON(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "messy.csv")
	if err := os.WriteFile(csv, []byte("id,score,note\n1,10, hi \n2,abc,\n3,30,N/A\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := runProfileJSON(t, []string{"sqly", "--profile", csv})

	if len(report.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(report.Tables))
	}
	tbl := report.Tables[0]
	if tbl.RowCount != 3 || tbl.ColumnCount != 3 {
		t.Errorf("row/col = %d/%d, want 3/3", tbl.RowCount, tbl.ColumnCount)
	}
	byName := map[string]profileColumn{}
	for _, c := range tbl.Columns {
		byName[c.Name] = c
	}
	if !hasWarningContaining(byName["score"].Warnings, "mixed numeric") {
		t.Errorf("score should warn about mixed types, got %v", byName["score"].Warnings)
	}
	if !hasWarningContaining(byName["note"].Warnings, "null placeholders") {
		t.Errorf("note should warn about null placeholders, got %v", byName["note"].Warnings)
	}
}

func TestRunProfile_MultiTable(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "users.csv")
	b := filepath.Join(dir, "orders.csv")
	if err := os.WriteFile(a, []byte("id,name\n1,Alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("oid,amount\n1,9.99\n2,5.00\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := runProfileJSON(t, []string{"sqly", "--profile", a, b})
	if len(report.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(report.Tables))
	}
	// Sorted by name: orders, users.
	if report.Tables[0].Name != "orders" || report.Tables[1].Name != "users" {
		t.Errorf("tables = %s,%s, want orders,users", report.Tables[0].Name, report.Tables[1].Name)
	}
}

func TestRunProfile_TextFormat(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "nums.csv")
	if err := os.WriteFile(csv, []byte("id\n1\n2\n3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shell, cleanup, err := newShell(t, []string{"sqly", "--profile", "--profile-format", "text", csv})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	out := captureStdout(t, func() {
		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
	if !strings.Contains(out, "table nums: 3 rows, 1 columns") {
		t.Errorf("text output missing table header line:\n%s", out)
	}
}
