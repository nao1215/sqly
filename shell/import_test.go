package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

func TestImportDirectory_EmptyDir_ReturnsError(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	emptyDir := t.TempDir()

	// filesql returns an error for empty directories (no supported files found),
	// so importDirectory propagates the error and returns imported=false.
	imported, err := s.importDirectory(context.Background(), emptyDir, emptyDir, "")
	if err == nil {
		t.Fatal("expected error for empty directory, got nil")
	}
	if imported {
		t.Error("expected imported=false for empty directory, got true")
	}
}

func TestImportDirectory_OverwriteOnly_ReturnsNotImported(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	csvContent := "id,name\n1,Alice\n"
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First import creates the table
	imported, err := s.importDirectory(ctx, dir, dir, "")
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if !imported {
		t.Error("expected first import to succeed")
	}

	// Second import of the same directory overwrites the existing table;
	// no new tables are added, so it should return false.
	imported, err = s.importDirectory(ctx, dir, dir, "")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if imported {
		t.Error("expected imported=false for overwrite-only directory, got true")
	}
}

func TestImportCommand_EmptyDirDoesNotMaskFileError(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	emptyDir := t.TempDir()
	ctx := context.Background()

	// .import emptydir missing.csv — both should fail, returning "all import attempts failed"
	cmds := s.commands
	err = cmds.importCommand(ctx, s, []string{emptyDir, "missing.csv"})
	if err == nil {
		t.Error("expected error when all imports fail, got nil")
	}
}

func TestFilterExcelSheets_NoCollisionWithSimilarPrefix(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Simulate pre-existing tables from sales_q1.xlsx (prefix: sales_q1_)
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE sales_q1_Revenue (id INTEGER, amount REAL)")
	if err != nil {
		t.Fatalf("failed to create pre-existing table: %v", err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"INSERT INTO sales_q1_Revenue VALUES (1, 100.0)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Simulate tables from sales.xlsx (prefix: sales_)
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE sales_Summary (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE sales_Details (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	// Filter sales.xlsx to keep only "Summary".
	// Pass candidates scoped to sales.xlsx tables only, simulating what
	// importFile/importDirectory would provide from the diff.
	candidates := map[string]struct{}{
		"sales_Summary": {},
		"sales_Details": {},
	}
	err = s.filterExcelSheets(ctx, "sales.xlsx", "Summary", candidates)
	if err != nil {
		t.Fatalf("filterExcelSheets: %v", err)
	}

	// sales_q1_Revenue must NOT be dropped (different prefix)
	table, err := s.usecases.sqlite3.List(ctx, "sales_q1_Revenue")
	if err != nil {
		t.Fatalf("sales_q1_Revenue should still exist: %v", err)
	}
	if len(table.Records()) != 1 {
		t.Errorf("expected 1 record in sales_q1_Revenue, got %d", len(table.Records()))
	}

	// sales_Summary must be kept
	_, err = s.usecases.sqlite3.List(ctx, "sales_Summary")
	if err != nil {
		t.Fatalf("sales_Summary should still exist: %v", err)
	}

	// sales_Details must be dropped
	_, err = s.usecases.sqlite3.List(ctx, "sales_Details")
	if err == nil {
		t.Error("expected sales_Details to be dropped, but it still exists")
	}
}

func TestFilterExcelSheets_UnderscoreInFilename(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// sales_q1.xlsx produces tables with prefix "sales_q1_"
	// So sales_q1_Summary has sheet part "Summary" (after stripping "sales_q1_")
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE sales_q1_Summary (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE sales_q1_Details (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	err = s.filterExcelSheets(ctx, "sales_q1.xlsx", "Summary", nil)
	if err != nil {
		t.Fatalf("filterExcelSheets: %v", err)
	}

	// Summary should be kept
	_, err = s.usecases.sqlite3.List(ctx, "sales_q1_Summary")
	if err != nil {
		t.Fatalf("sales_q1_Summary should still exist: %v", err)
	}

	// Details should be dropped
	_, err = s.usecases.sqlite3.List(ctx, "sales_q1_Details")
	if err == nil {
		t.Error("expected sales_q1_Details to be dropped")
	}
}

func TestFilterExcelSheets_SheetNotFound(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE report_Sheet1 (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE report_Sheet2 (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	err = s.filterExcelSheets(ctx, "report.xlsx", "NonExistent", nil)
	if err == nil {
		t.Error("expected error for non-existent sheet, got nil")
	}

	// Both tables should be dropped
	_, err = s.usecases.sqlite3.List(ctx, "report_Sheet1")
	if err == nil {
		t.Error("expected report_Sheet1 to be dropped")
	}
	_, err = s.usecases.sqlite3.List(ctx, "report_Sheet2")
	if err == nil {
		t.Error("expected report_Sheet2 to be dropped")
	}
}

func TestFilterExcelSheets_ReimportWithSheet(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Simulate first import of report.xlsx (all sheets loaded)
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE report_Summary (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE report_Details (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	// Re-import with --sheet=Summary: tables already exist (overwrite case).
	// filterExcelSheets uses prefix matching on all current tables, not diff,
	// so it should still find and filter correctly.
	err = s.filterExcelSheets(ctx, "report.xlsx", "Summary", nil)
	if err != nil {
		t.Fatalf("filterExcelSheets on re-import: %v", err)
	}

	// Summary should be kept
	_, err = s.usecases.sqlite3.List(ctx, "report_Summary")
	if err != nil {
		t.Fatalf("report_Summary should still exist: %v", err)
	}

	// Details should be dropped
	_, err = s.usecases.sqlite3.List(ctx, "report_Details")
	if err == nil {
		t.Error("expected report_Details to be dropped on re-import with --sheet")
	}
}

func TestImportDirectory_SheetDoesNotDropNonExcelTables(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Pre-load a CSV table that should survive --sheet filtering
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE users (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"INSERT INTO users VALUES (1, 'Alice')")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate Excel tables that would be imported from a directory
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE workbook_Sheet1 (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE workbook_Sheet2 (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	// filterExcelSheets only touches tables with the exact Excel prefix;
	// non-Excel tables like "users" must not be affected.
	err = s.filterExcelSheets(ctx, "workbook.xlsx", "Sheet1", nil)
	if err != nil {
		t.Fatalf("filterExcelSheets: %v", err)
	}

	// users table must still exist
	table, err := s.usecases.sqlite3.List(ctx, "users")
	if err != nil {
		t.Fatalf("users table should still exist: %v", err)
	}
	if len(table.Records()) != 1 {
		t.Errorf("expected 1 record in users, got %d", len(table.Records()))
	}

	// workbook_Sheet1 kept, workbook_Sheet2 dropped
	_, err = s.usecases.sqlite3.List(ctx, "workbook_Sheet1")
	if err != nil {
		t.Fatalf("workbook_Sheet1 should still exist: %v", err)
	}
	_, err = s.usecases.sqlite3.List(ctx, "workbook_Sheet2")
	if err == nil {
		t.Error("expected workbook_Sheet2 to be dropped")
	}
}

func TestImportFile_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tmpFile := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}

	err = s.importFile(context.Background(), tmpFile, tmpFile, "")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestImportFile_CSVSuccess(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tmpFile := filepath.Join(t.TempDir(), "people.csv")
	if err := os.WriteFile(tmpFile, []byte("id,name\n1,Alice\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := s.importFile(ctx, tmpFile, tmpFile, ""); err != nil {
		t.Fatalf("importFile: %v", err)
	}

	tables, err := s.usecases.sqlite3.GetTableNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tbl := range tables {
		if tbl.Name() == "people" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'people' table after CSV import")
	}
}

func TestImportFile_NonexistentFile(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	err = s.importFile(context.Background(), "/nonexistent/file.csv", "/nonexistent/file.csv", "")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestImportFile_ExcelWithSheet(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Use the project's test Excel file
	excelPath := filepath.Join("..", "testdata", "sample.xlsx")
	if _, err := os.Stat(excelPath); os.IsNotExist(err) {
		t.Skip("testdata/sample.xlsx not found")
	}

	ctx := context.Background()
	err = s.importFile(ctx, excelPath, excelPath, "test_sheet")
	if err != nil {
		t.Fatalf("importFile with --sheet: %v", err)
	}

	// Verify at least one table exists after import
	tables, err := s.usecases.sqlite3.GetTableNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Error("expected at least one table after Excel import with --sheet")
	}
}

func TestImportDirectory_WithCSVFiles(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.csv"), []byte("id,val\n1,x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.tsv"), []byte("id\tval\n2\ty\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	imported, err := s.importDirectory(ctx, dir, dir, "")
	if err != nil {
		t.Fatalf("importDirectory: %v", err)
	}
	if !imported {
		t.Error("expected imported=true")
	}

	tables, err := s.usecases.sqlite3.GetTableNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) < 2 {
		t.Errorf("expected at least 2 tables, got %d", len(tables))
	}
}

func TestImportCommand_PartialSuccess(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "ok.csv")
	if err := os.WriteFile(csvPath, []byte("id\n1\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	// One valid file + one missing file → partial success (no error returned)
	err = s.commands.importCommand(ctx, s, []string{csvPath, "missing.csv"})
	if err != nil {
		t.Errorf("expected nil error for partial success, got: %v", err)
	}
}

func TestImportCommand_SheetArgExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{"no sheet", []string{"file.csv"}, ""},
		{"sheet flag", []string{"file.xlsx", "--sheet=Summary"}, "Summary"},
		{"sheet flag first", []string{"--sheet=Data", "file.xlsx"}, "Data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractSheetNameFromArgs(tt.argv)
			if got != tt.want {
				t.Errorf("extractSheetNameFromArgs(%v) = %q, want %q", tt.argv, got, tt.want)
			}
		})
	}
}

func TestValidatePath_Import(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"normal path", "testdata/sample.csv", false},
		{"relative path", "./foo/bar.csv", false},
		{"path traversal", "../../../etc/passwd", true},
		{"url encoded traversal", "..%2f..%2fetc/passwd", true},
		{"system dir /etc", "/etc/hosts", true},
		{"system dir /proc", "/proc/cpuinfo", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestDiffTableNames(t *testing.T) {
	t.Parallel()

	// Minimal test covering the helper function
	existing := map[string]struct{}{"a": {}, "b": {}}

	tables := []*model.Table{
		model.NewTable("a", nil, nil),
		model.NewTable("b", nil, nil),
		model.NewTable("c", nil, nil),
	}

	got := diffTableNames(tables, existing)
	if len(got) != 1 || got[0] != "c" {
		t.Errorf("diffTableNames = %v, want [c]", got)
	}
}

func TestTableNameSet(t *testing.T) {
	t.Parallel()

	tables := []*model.Table{
		model.NewTable("x", nil, nil),
		model.NewTable("y", nil, nil),
	}

	set := tableNameSet(tables)
	if len(set) != 2 {
		t.Errorf("expected 2 entries, got %d", len(set))
	}
	if _, ok := set["x"]; !ok {
		t.Error("expected 'x' in set")
	}
	if _, ok := set["y"]; !ok {
		t.Error("expected 'y' in set")
	}
}
