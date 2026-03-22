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

func TestFilterExcelSheetsInSet_NoCollisionWithSimilarPrefix(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Simulate pre-existing table from sales_q1.xlsx
	_, err = s.usecases.sqlite3.Exec(ctx,
		"CREATE TABLE sales_q1_Revenue (id INTEGER, amount REAL)")
	if err != nil {
		t.Fatalf("failed to create pre-existing table: %v", err)
	}
	_, err = s.usecases.sqlite3.Exec(ctx,
		"INSERT INTO sales_q1_Revenue VALUES (1, 100.0)")
	if err != nil {
		t.Fatalf("failed to insert into pre-existing table: %v", err)
	}

	// The new set represents tables just added by importing sales.xlsx
	newTables := map[string]struct{}{
		"sales_Summary": {},
		"sales_Details": {},
	}

	// Filter to keep only "Summary" sheet
	err = s.filterExcelSheetsInSet(ctx, newTables, "Summary")
	if err != nil {
		t.Fatalf("filterExcelSheetsInSet: %v", err)
	}

	// Verify sales_q1_Revenue was NOT dropped (it was not in newTables)
	table, err := s.usecases.sqlite3.List(ctx, "sales_q1_Revenue")
	if err != nil {
		t.Fatalf("sales_q1_Revenue should still exist but got error: %v", err)
	}
	if len(table.Records()) != 1 {
		t.Errorf("expected 1 record in sales_q1_Revenue, got %d", len(table.Records()))
	}

	// Verify sales_Details was dropped
	_, err = s.usecases.sqlite3.List(ctx, "sales_Details")
	if err == nil {
		t.Error("expected sales_Details to be dropped, but it still exists")
	}
}

func TestFilterExcelSheetsInSet_SheetNotFound(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Create tables as if imported from Excel
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

	newTables := map[string]struct{}{
		"report_Sheet1": {},
		"report_Sheet2": {},
	}

	err = s.filterExcelSheetsInSet(ctx, newTables, "NonExistent")
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
