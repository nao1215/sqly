package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// columnStats folds the given values and NULL mask through the streaming
// columnProfiler, so the unit tests exercise the same accumulator the profiling
// path uses.
func columnStats(name, typ string, values []string, nulls []bool) profileColumn {
	p := newColumnProfiler(name, typ)
	for i, v := range values {
		p.add(v, i < len(nulls) && nulls[i])
	}
	return p.result()
}

func TestProfileColumnStats(t *testing.T) {
	t.Parallel()

	t.Run("counts nulls, blanks, distinct, and numeric values", func(t *testing.T) {
		t.Parallel()
		values := []string{"1", "2", "", "2", "x"}
		nulls := []bool{false, false, false, false, false}
		// Mark index 4 ("x") as a real NULL to separate null from blank.
		nulls[4] = true
		got := columnStats("counts", "TEXT", values, nulls)
		if got.NullCount != 1 {
			t.Errorf("null_count = %d, want 1", got.NullCount)
		}
		if got.BlankCount != 1 {
			t.Errorf("blank_count = %d, want 1", got.BlankCount)
		}
		// Values are "1","2","","2" (index 4 is NULL). The blank string counts as a
		// real distinct value, so distinct {1,2,""} = 3 and the metric no longer
		// understates cardinality for columns that mix blanks with real values.
		if got.DistinctCount != 3 {
			t.Errorf("distinct_count = %d, want 3", got.DistinctCount)
		}
		if got.NumericCount != 3 {
			t.Errorf("numeric_count = %d, want 3", got.NumericCount)
		}
	})

	t.Run("counts a blank string as a distinct value alongside real values", func(t *testing.T) {
		t.Parallel()
		// Mixing blanks with real values: ["", "A", ""] has one real value plus the
		// blank, so distinct {"", "A"} = 2 and blank_count = 2.
		got := columnStats("v", "TEXT", []string{"", "A", ""}, []bool{false, false, false})
		if got.BlankCount != 2 {
			t.Errorf("blank_count = %d, want 2", got.BlankCount)
		}
		if got.DistinctCount != 2 {
			t.Errorf("distinct_count = %d, want 2", got.DistinctCount)
		}
	})

	t.Run("warns on a mix of numeric and non-numeric values", func(t *testing.T) {
		t.Parallel()
		got := columnStats("c", "TEXT", []string{"1", "2", "abc"}, []bool{false, false, false})
		if !hasWarningContaining(got.Warnings, "mixed numeric") {
			t.Errorf("expected a mixed-type warning, got %v", got.Warnings)
		}
	})

	t.Run("warns on null-like placeholder text and surrounding whitespace", func(t *testing.T) {
		t.Parallel()
		got := columnStats("c", "TEXT", []string{"N/A", " hi "}, []bool{false, false})
		if !hasWarningContaining(got.Warnings, "null placeholders") {
			t.Errorf("expected a null-placeholder warning, got %v", got.Warnings)
		}
		if !hasWarningContaining(got.Warnings, "whitespace") {
			t.Errorf("expected a whitespace warning, got %v", got.Warnings)
		}
	})

	t.Run("flags a padded null-like placeholder and its whitespace together", func(t *testing.T) {
		t.Parallel()
		// " NULL " and " N/A " are null-like once trimmed, and they also carry
		// surrounding whitespace, so both warnings must fire.
		got := columnStats("c", "TEXT", []string{" NULL ", " N/A "}, []bool{false, false})
		if !hasWarningContaining(got.Warnings, "null placeholders") {
			t.Errorf("expected a null-placeholder warning, got %v", got.Warnings)
		}
		if !hasWarningContaining(got.Warnings, "whitespace") {
			t.Errorf("expected a whitespace warning, got %v", got.Warnings)
		}
	})

	t.Run("padded ordinary value warns only about whitespace", func(t *testing.T) {
		t.Parallel()
		// " hello " is padded but not null-like, so only the whitespace warning
		// fires; trimming must not turn ordinary values into null placeholders.
		got := columnStats("c", "TEXT", []string{" hello "}, []bool{false})
		if hasWarningContaining(got.Warnings, "null placeholders") {
			t.Errorf("did not expect a null-placeholder warning, got %v", got.Warnings)
		}
		if !hasWarningContaining(got.Warnings, "whitespace") {
			t.Errorf("expected a whitespace warning, got %v", got.Warnings)
		}
	})

	t.Run("clean numeric column has no warnings", func(t *testing.T) {
		t.Parallel()
		got := columnStats("c", "INTEGER", []string{"1", "2", "3"}, []bool{false, false, false})
		if len(got.Warnings) != 0 {
			t.Errorf("expected no warnings, got %v", got.Warnings)
		}
	})

	t.Run("counts comma-formatted numerals as numeric like table-mode does", func(t *testing.T) {
		t.Parallel()
		// "1,000" and "2,500" are numeric to table-mode alignment, so profiling
		// must agree and count them as numeric instead of reporting numeric=0.
		got := columnStats("amount", "TEXT", []string{"1,000", "2,500"}, []bool{false, false})
		if got.NumericCount != 2 {
			t.Errorf("numeric_count = %d, want 2", got.NumericCount)
		}
		if hasWarningContaining(got.Warnings, "mixed numeric") {
			t.Errorf("comma numerals should not be a mixed-type column, got %v", got.Warnings)
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

// writeProfileBenchCSV writes a wide, tall CSV and returns its path.
func writeProfileBenchCSV(tb testing.TB, rows, cols int) string {
	tb.Helper()
	dir := tb.TempDir()
	path := filepath.Join(dir, "big.csv")

	var sb strings.Builder
	for c := range cols {
		if c > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "c%d", c)
	}
	sb.WriteByte('\n')
	for r := range rows {
		for c := range cols {
			if c > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(&sb, "v%d", (r+c)%97)
		}
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		tb.Fatal(err)
	}
	return path
}

func TestProfileTable_StreamingMatchesFullScan(t *testing.T) {
	csv := writeProfileBenchCSV(t, 200, 4)
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{csv}); err != nil {
		t.Fatalf("import: %v", err)
	}

	entry, err := shell.profileTable(context.Background(), "big")
	if err != nil {
		t.Fatalf("profileTable: %v", err)
	}

	if entry.RowCount != 200 || entry.ColumnCount != 4 {
		t.Fatalf("row/col = %d/%d, want 200/4", entry.RowCount, entry.ColumnCount)
	}
	// Each column draws from 97 distinct generated values, so a 200-row column
	// sees all 97. This confirms the single-pass aggregation matches a full scan.
	for _, c := range entry.Columns {
		if c.DistinctCount != 97 {
			t.Errorf("column %s distinct = %d, want 97", c.Name, c.DistinctCount)
		}
		if c.NumericCount != 0 {
			t.Errorf("column %s numeric = %d, want 0 (values are v-prefixed)", c.Name, c.NumericCount)
		}
	}
}

func BenchmarkProfileTable(b *testing.B) {
	csv := writeProfileBenchCSV(b, 5000, 8)
	shell, cleanup, err := newShell(b, []string{"sqly"})
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{csv}); err != nil {
		b.Fatalf("import: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := shell.profileTable(context.Background(), "big"); err != nil {
			b.Fatal(err)
		}
	}
}
