package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

func TestCompareSchemas(t *testing.T) {
	t.Parallel()
	col := func(name, typ string) inspectColumn { return inspectColumn{Name: name, Type: typ} }

	t.Run("identical schemas are equal", func(t *testing.T) {
		t.Parallel()
		got := compareSchemas(
			[]inspectColumn{col("id", "INTEGER"), col("name", "TEXT")},
			[]inspectColumn{col("id", "INTEGER"), col("name", "TEXT")},
		)
		if !got.Equal {
			t.Errorf("expected equal schemas, got %+v", got)
		}
	})

	t.Run("added, removed, and type-changed columns are reported", func(t *testing.T) {
		t.Parallel()
		got := compareSchemas(
			[]inspectColumn{col("id", "INTEGER"), col("name", "TEXT"), col("old", "TEXT")},
			[]inspectColumn{col("id", "TEXT"), col("name", "TEXT"), col("new", "REAL")},
		)
		if got.Equal {
			t.Fatal("expected schemas to differ")
		}
		if len(got.LeftOnlyColumns) != 1 || got.LeftOnlyColumns[0] != "old" {
			t.Errorf("left_only = %v, want [old]", got.LeftOnlyColumns)
		}
		if len(got.RightOnlyColumns) != 1 || got.RightOnlyColumns[0] != "new" {
			t.Errorf("right_only = %v, want [new]", got.RightOnlyColumns)
		}
		if len(got.TypeChanges) != 1 || got.TypeChanges[0].Name != "id" ||
			got.TypeChanges[0].LeftType != "INTEGER" || got.TypeChanges[0].RightType != "TEXT" {
			t.Errorf("type_changes = %+v, want id INTEGER->TEXT", got.TypeChanges)
		}
	})
}

func TestCompareKeyedRows(t *testing.T) {
	t.Parallel()
	header := model.Header{"id", "name", "age"}
	left := model.NewTable("l", header, []model.Record{
		{"1", "Alice", "30"},
		{"2", "Bob", "25"},
		{"3", "Carol", "40"},
	})
	right := model.NewTable("r", header, []model.Record{
		{"1", "Alice", "31"}, // modified
		{"2", "Bob", "25"},   // unchanged
		{"4", "Dave", "50"},  // added
	})

	cell := func(r compareRow, col string) string {
		v, ok := r[col]
		if !ok || v == nil {
			return "<nil>"
		}
		return *v
	}

	t.Run("classifies added, removed, and modified rows", func(t *testing.T) {
		t.Parallel()
		rows, err := compareKeyedRows("l", "r", left, right, "id")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows.Added) != 1 || cell(rows.Added[0], "id") != "4" {
			t.Errorf("added = %v, want [id=4]", rows.Added)
		}
		if len(rows.Removed) != 1 || cell(rows.Removed[0], "id") != "3" {
			t.Errorf("removed = %v, want [id=3]", rows.Removed)
		}
		if len(rows.Modified) != 1 || rows.Modified[0].Key != "1" ||
			cell(rows.Modified[0].Left, "age") != "30" || cell(rows.Modified[0].Right, "age") != "31" {
			t.Errorf("modified = %+v, want key=1 age 30->31", rows.Modified)
		}
	})

	t.Run("distinguishes a SQL NULL from an empty string", func(t *testing.T) {
		t.Parallel()
		h := model.Header{"id", "v"}
		l := model.NewTable("l", h, []model.Record{{"1", ""}})
		l.SetNulls([][]bool{{false, true}}) // v is NULL
		r := model.NewTable("r", h, []model.Record{{"1", ""}})
		r.SetNulls([][]bool{{false, false}}) // v is empty string
		rows, err := compareKeyedRows("l", "r", l, r, "id")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows.Modified) != 1 {
			t.Fatalf("expected NULL vs \"\" to be a modification, got %+v", rows)
		}
		if rows.Modified[0].Left["v"] != nil {
			t.Errorf("left v should be NULL (nil), got %v", rows.Modified[0].Left["v"])
		}
		if rows.Modified[0].Right["v"] == nil || *rows.Modified[0].Right["v"] != "" {
			t.Errorf("right v should be empty string, got %v", rows.Modified[0].Right["v"])
		}
	})

	t.Run("missing key column is rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := compareKeyedRows("l", "r", left, right, "nope"); err == nil {
			t.Error("expected an error for a missing key column")
		}
	})

	t.Run("a duplicate key value is rejected as ambiguous", func(t *testing.T) {
		t.Parallel()
		dup := model.NewTable("d", header, []model.Record{
			{"1", "Alice", "30"},
			{"1", "Alice2", "31"},
		})
		if _, err := compareKeyedRows("l", "d", left, dup, "id"); err == nil {
			t.Error("expected an error for a duplicate key value")
		}
	})
}

func TestResolveCompareTables_PreservesImportOrder(t *testing.T) {
	t.Parallel()
	// With exactly two imported tables and no --compare-tables override, the
	// left/right pair must follow the import (CLI input) order that TablesName
	// returns, not an alphabetical re-sort. Here "users" is imported before "user"
	// even though it sorts after it.
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return([]*model.Table{
		model.NewTable("users", nil, nil),
		model.NewTable("user", nil, nil),
	}, nil)

	s := &Shell{usecases: Usecases{metadata: metadata}, argument: &config.Arg{}}
	left, right, err := s.resolveCompareTables(context.Background())
	if err != nil {
		t.Fatalf("resolveCompareTables: %v", err)
	}
	if left != "users" || right != "user" {
		t.Errorf("left,right = %q,%q, want users,user (CLI input order preserved)", left, right)
	}
}

func TestRunCompare_PreservesCLIInputOrder(t *testing.T) {
	// End-to-end: passing zebra.csv before ant.csv must report zebra as the left
	// side even though "ant" sorts first, so the left/right direction follows the
	// command the user typed.
	dir := t.TempDir()
	left := filepath.Join(dir, "zebra.csv")
	right := filepath.Join(dir, "ant.csv")
	if err := os.WriteFile(left, []byte("id,name\n1,Alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,name\n1,Alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := runCompareJSON(t, []string{"sqly", "--compare", left, right})
	if report.Left != "zebra" || report.Right != "ant" {
		t.Errorf("left,right = %q,%q, want zebra,ant (CLI input order preserved)", report.Left, report.Right)
	}
}

// runCompareJSON builds a shell from args, runs it, and decodes the JSON report.
func runCompareJSON(t *testing.T, args []string) compareReport {
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
	var report compareReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("compare output is not valid JSON: %v\n%s", err, out)
	}
	return report
}

func writeCompareFixtures(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	a := filepath.Join(dir, "cmp_left.csv")
	b := filepath.Join(dir, "cmp_right.csv")
	if err := os.WriteFile(a, []byte("id,name,age\n1,Alice,30\n2,Bob,25\n3,Carol,40\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("id,name,age\n1,Alice,31\n2,Bob,25\n4,Dave,50\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return a, b
}

func TestRunCompare_KeyedRowsJSON(t *testing.T) {
	a, b := writeCompareFixtures(t)
	report := runCompareJSON(t, []string{"sqly", "--compare", "--compare-key", "id", a, b})

	if !report.Schema.Equal {
		t.Errorf("expected equal schemas, got %+v", report.Schema)
	}
	if report.RowCount.Left != 3 || report.RowCount.Right != 3 || report.RowCount.Delta != 0 {
		t.Errorf("row_count = %+v, want 3/3/0", report.RowCount)
	}
	if report.Rows == nil {
		t.Fatal("expected keyed rows section")
	}
	if len(report.Rows.Added) != 1 || len(report.Rows.Removed) != 1 || len(report.Rows.Modified) != 1 {
		t.Errorf("rows = %+v, want 1 added/1 removed/1 modified", report.Rows)
	}
}

func TestRunCompare_SchemaOnlyAndRowCount(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "s_left.csv")
	b := filepath.Join(dir, "s_right.csv")
	// b drops a column and adds a row, with no key requested.
	if err := os.WriteFile(a, []byte("id,name,extra\n1,Alice,x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("id,name\n1,Alice\n2,Bob\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := runCompareJSON(t, []string{"sqly", "--compare", a, b})

	if report.Schema.Equal {
		t.Error("expected schemas to differ")
	}
	if len(report.Schema.LeftOnlyColumns) != 1 || report.Schema.LeftOnlyColumns[0] != "extra" {
		t.Errorf("left_only = %v, want [extra]", report.Schema.LeftOnlyColumns)
	}
	if report.RowCount.Delta != 1 {
		t.Errorf("delta = %d, want 1", report.RowCount.Delta)
	}
	if report.Rows != nil {
		t.Error("rows section must be absent without --compare-key")
	}
}

func TestRunCompare_Errors(t *testing.T) {
	a, b := writeCompareFixtures(t)

	t.Run("missing key column errors", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--compare", "--compare-key", "nope", a, b})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.Run(context.Background()); err == nil {
			t.Error("expected an error for a missing key column")
		}
	})

	t.Run("a single table is ambiguous", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--compare", a})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.Run(context.Background()); err == nil {
			t.Error("expected an error comparing a single table")
		}
	})

	t.Run("a named missing table errors", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--compare", "--compare-tables", "cmp_left,ghost", a, b})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.Run(context.Background()); err == nil {
			t.Error("expected an error for a missing named table")
		}
	})
}

// writeKeyedCompareBenchCSVs writes two large keyed CSVs that differ by a few
// added, removed, and modified rows, returning their paths.
func writeKeyedCompareBenchCSVs(tb testing.TB, rows int) (string, string) {
	tb.Helper()
	dir := tb.TempDir()
	left := filepath.Join(dir, "kleft.csv")
	right := filepath.Join(dir, "kright.csv")

	var lb, rb strings.Builder
	lb.WriteString("id,name,score\n")
	rb.WriteString("id,name,score\n")
	for i := range rows {
		fmt.Fprintf(&lb, "%d,name%d,%d\n", i, i, i%100)
		// Shift the score on every 10th row (modified) and drop the last row on
		// the right while adding one fresh id, so the diff has work to do.
		score := i % 100
		if i%10 == 0 {
			score++
		}
		if i != rows-1 {
			fmt.Fprintf(&rb, "%d,name%d,%d\n", i, i, score)
		}
	}
	fmt.Fprintf(&rb, "%d,fresh,1\n", rows+1)

	if err := os.WriteFile(left, []byte(lb.String()), 0o600); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(right, []byte(rb.String()), 0o600); err != nil {
		tb.Fatal(err)
	}
	return left, right
}

func TestBuildCompareReport_KeyedDiff(t *testing.T) {
	left, right := writeKeyedCompareBenchCSVs(t, 50)
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{left, right}); err != nil {
		t.Fatalf("import: %v", err)
	}

	report, err := shell.buildCompareReport(context.Background(), "kleft", "kright", "id")
	if err != nil {
		t.Fatalf("buildCompareReport: %v", err)
	}
	if report.Rows == nil {
		t.Fatal("expected keyed rows section")
	}
	// Right drops the last row (1 removed), adds one fresh id (1 added), and
	// shifts the score on ids 0,10,20,30,40 (5 modified).
	if len(report.Rows.Removed) != 1 {
		t.Errorf("removed = %d, want 1", len(report.Rows.Removed))
	}
	if len(report.Rows.Added) != 1 {
		t.Errorf("added = %d, want 1", len(report.Rows.Added))
	}
	if len(report.Rows.Modified) != 5 {
		t.Errorf("modified = %d, want 5", len(report.Rows.Modified))
	}
}

func BenchmarkBuildCompareReportKeyed(b *testing.B) {
	left, right := writeKeyedCompareBenchCSVs(b, 5000)
	shell, cleanup, err := newShell(b, []string{"sqly"})
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{left, right}); err != nil {
		b.Fatalf("import: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := shell.buildCompareReport(context.Background(), "kleft", "kright", "id"); err != nil {
			b.Fatal(err)
		}
	}
}
