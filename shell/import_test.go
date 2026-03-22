package shell

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
