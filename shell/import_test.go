package shell

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// These tests intentionally avoid t.Parallel at the top level.
// importCommand/importDirectory/importFile can write to the package-global
// config.Stdout, and running them concurrently with shell_test helpers that
// temporarily swap config.Stdout to an os.Pipe can deadlock on Windows due to
// the smaller pipe buffer size.

func TestImportDirectory_EmptyDir_ReturnsError(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	emptyDir := t.TempDir()

	// filesql returns an error for empty directories (no supported files found),
	// so importDirectory propagates the error and returns imported=false.
	imported, _, err := s.importDirectory(context.Background(), emptyDir, emptyDir, "", false)
	if err == nil {
		t.Fatal("expected error for empty directory, got nil")
	}
	if imported {
		t.Error("expected imported=false for empty directory, got true")
	}
}

func TestImportDirectory_ReimportSameDir_ReportsOverwrite(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	csvContent := "id,name\n1,Alice\n"
	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First import creates the table.
	imported, _, err := s.importDirectory(ctx, dir, dir, "", false)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if !imported {
		t.Error("expected first import to succeed")
	}

	// Re-importing the same directory overwrites the existing table. The
	// directory still contains a supported file, so the import is reported as
	// successful (it overwrote data) rather than as "No supported files". Ref
	imported, _, err = s.importDirectory(ctx, dir, dir, "", false)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if !imported {
		t.Error("expected re-import of a directory with a supported file to report imported=true")
	}
}

// copyTestFile copies a file from shell/testdata into dst for directory-import
// tests that need real Excel/ACH/Fedwire inputs.
func copyTestFile(t *testing.T, name, dst string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name)) //nolint:gosec // test reads a fixed testdata fixture
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil { //nolint:gosec // test writes to a temp path
		t.Fatal(err)
	}
}

func newHTTPImportServer(t *testing.T) *httptest.Server {
	t.Helper()
	shiftJISCSV := mustEncodeString(t, japanese.ShiftJIS.NewEncoder(), "id,name\n1,太郎\n2,花子\n")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user.csv":
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("user_name,identifier,first_name,last_name\nbooker12,1,Rachel,Booker\njenkins46,2,Mary,Jenkins\n"))
		case "/download":
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", `attachment; filename="remote-user.csv"`)
			_, _ = w.Write([]byte("user_name,identifier\nbooker12,1\n"))
		case "/shiftjis.csv":
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write(shiftJISCSV)
		default:
			http.NotFound(w, r)
		}
	}))
}

func mustEncodeString(t *testing.T, transformer transform.Transformer, content string) []byte {
	t.Helper()

	encoded, _, err := transform.String(transformer, content)
	if err != nil {
		t.Fatalf("transform.String: %v", err)
	}
	return []byte(encoded)
}

func TestImportCommand_DownloadsHTTPURL(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	server := newHTTPImportServer(t)
	defer server.Close()
	s.httpClient = server.Client()

	backupStderr := config.Stderr
	defer func() { config.Stderr = backupStderr }()
	var stderr bytes.Buffer
	config.Stderr = &stderr

	if err := s.commands.importCommand(context.Background(), s, []string{server.URL + "/user.csv"}); err != nil {
		t.Fatalf("importCommand: %v", err)
	}

	user, err := s.usecases.metadata.List(context.Background(), "user")
	if err != nil {
		t.Fatalf("List(user): %v", err)
	}
	if got := len(user.Records()); got != 2 {
		t.Fatalf("row count = %d, want 2", got)
	}
	if got := s.tableSources["user"]; got != server.URL+"/user.csv" {
		t.Fatalf("table source = %q, want %q", got, server.URL+"/user.csv")
	}
	if out := stderr.String(); out != "" {
		t.Fatalf("stderr = %q, want no download output", out)
	}
}

func TestImportCommand_UsesContentDispositionFilenameForHTTPURL(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	server := newHTTPImportServer(t)
	defer server.Close()
	s.httpClient = server.Client()

	if err := s.commands.importCommand(context.Background(), s, []string{server.URL + "/download"}); err != nil {
		t.Fatalf("importCommand: %v", err)
	}

	if _, err := s.usecases.metadata.List(context.Background(), "remote_user"); err != nil {
		t.Fatalf("expected remote_user table from Content-Disposition filename: %v", err)
	}
}

func TestImportCommand_DecodesShiftJISHTTPURL(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly", "--encoding", "shift-jis"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	server := newHTTPImportServer(t)
	defer server.Close()
	s.httpClient = server.Client()

	if err := s.commands.importCommand(context.Background(), s, []string{server.URL + "/shiftjis.csv"}); err != nil {
		t.Fatalf("importCommand: %v", err)
	}

	people, err := s.usecases.metadata.List(context.Background(), "shiftjis")
	if err != nil {
		t.Fatalf("List(shiftjis): %v", err)
	}
	if got := people.Records()[0][1]; got != "太郎" {
		t.Fatalf("first row name = %q, want %q", got, "太郎")
	}
}

func TestImportDirectory_RecordsPerFileSource(t *testing.T) {
	// Directory --inspect must report each table's real source file, even when
	// the basename is sanitized or the file produces multiple tables such
	// as Excel/ACH/Fedwire. The directory path must never be used as a
	// table source.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2023-data.csv"), []byte("id,name\n1,a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyTestFile(t, "sample.xlsx", filepath.Join(dir, "sample.xlsx"))
	copyTestFile(t, "ppd-debit.ach", filepath.Join(dir, "ppd-debit.ach"))
	copyTestFile(t, "customer-transfer.fed", filepath.Join(dir, "customer-transfer.fed"))

	ctx := context.Background()
	if _, _, err := s.importDirectory(ctx, dir, dir, "", false); err != nil {
		t.Fatalf("importDirectory: %v", err)
	}

	absDir, _ := filepath.Abs(dir)
	want := map[string]string{
		"sheet_2023_data":           "2023-data.csv",
		"sample_test_sheet":         "sample.xlsx",
		"ppd_debit_file_header":     "ppd-debit.ach",
		"ppd_debit_batches":         "ppd-debit.ach",
		"ppd_debit_entries":         "ppd-debit.ach",
		"customer_transfer_message": "customer-transfer.fed",
	}
	for table, file := range want {
		src, ok := s.tableSources[table]
		if !ok {
			t.Errorf("table %q has no recorded source", table)
			continue
		}
		if src == absDir {
			t.Errorf("table %q source is the directory path, want the file %q", table, file)
			continue
		}
		if !strings.HasSuffix(src, file) {
			t.Errorf("table %q source = %q, want it to end with %q", table, src, file)
		}
	}
}

func TestImportDirectory_RejectsDuplicateBasenameCollision(t *testing.T) {
	// Two files that map to the same table name from different subdirectories must
	// be rejected instead of one silently overwriting the other.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "user.csv"), []byte("id,name\n1,alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b", "user.csv"), []byte("id,name\n2,beta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err = s.importDirectory(context.Background(), dir, dir, "", false)
	if err == nil {
		t.Fatal("expected a collision error for duplicate basenames, got nil")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error = %q, want it to mention a collision", err)
	}
}

func TestImportDirectory_RejectsSanitizedCollision(t *testing.T) {
	// Two files whose names sanitize to the same table name must be rejected. Ref
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a-b.csv"), []byte("id,name\n1,alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_b.csv"), []byte("id,name\n2,beta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err = s.importDirectory(context.Background(), dir, dir, "", false)
	if err == nil {
		t.Fatal("expected a collision error for sanitized-name collision, got nil")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error = %q, want it to mention a collision", err)
	}
}

func TestImportDirectory_ReimportOverFileImport_UpdatesSourceAndBlocksSave(t *testing.T) {
	// A directory import that overwrites a table previously loaded from a file
	// argument must update the table's source to the directory file and mark it as
	// a directory import, so later .save --force cannot write the directory rows
	// back into the original file.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Original file argument: a user.csv loaded directly.
	work := t.TempDir()
	orig := filepath.Join(work, "user.csv")
	origData := []byte("user_name,identifier,first_name,last_name\norig1,1,ORIG,One\n")
	if err := os.WriteFile(orig, origData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.importFile(ctx, orig, orig, ""); err != nil {
		t.Fatalf("importFile: %v", err)
	}
	if s.dirImported["user"] {
		t.Fatal("user should not be a directory import yet")
	}

	// Directory whose user.csv overwrites the existing table.
	dir := filepath.Join(work, "dir")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	dirFile := filepath.Join(dir, "user.csv")
	if err := os.WriteFile(dirFile, []byte("user_name,identifier,first_name,last_name\nalt1,1,ALT,One\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	imported, _, err := s.importDirectory(ctx, dir, dir, "", false)
	if err != nil {
		t.Fatalf("importDirectory re-import: %v", err)
	}
	if !imported {
		t.Error("expected the directory re-import to report imported=true")
	}
	if !s.dirImported["user"] {
		t.Error("user must be marked as a directory import after re-import")
	}
	absDirFile, _ := filepath.Abs(dirFile)
	if got := s.tableSources["user"]; got != absDirFile {
		t.Errorf("user source = %q, want the directory file %q", got, absDirFile)
	}

	// Change the table so write-back considers it (an unchanged table is skipped),
	// then .save --force must refuse to write back a directory import, leaving the
	// original untouched.
	if err := s.exec(ctx, "INSERT INTO user VALUES ('alt2',2,'ALT','Two')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.writeBack(ctx, ""); err == nil {
		t.Error("expected .save --force to be rejected for a directory-imported table")
	}
	after, err := os.ReadFile(orig) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(origData) {
		t.Errorf("original file was overwritten:\n got %q\nwant %q", after, origData)
	}
}

func TestImportCommand_EmptyDirDoesNotMaskFileError(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	emptyDir := t.TempDir()
	ctx := context.Background()

	// .import emptydir missing.csv — both should fail, returning an all-failed error.
	cmds := s.commands
	err = cmds.importCommand(ctx, s, []string{emptyDir, "missing.csv"})
	if err == nil {
		t.Error("expected error when all imports fail, got nil")
	}
}

func TestSummarizeImportErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []string
		want     string
	}{
		{"no messages yields an empty summary", nil, ""},
		{"a single message is returned unchanged", []string{"path does not exist: a.csv"}, "path does not exist: a.csv"},
		{"multiple messages report the first and a remaining count", []string{"path does not exist: a.csv", "path does not exist: b.csv", "permission denied accessing path: c.csv"}, "path does not exist: a.csv (+2 more)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := summarizeImportErrors(tt.messages); got != tt.want {
				t.Errorf("summarizeImportErrors(%v) = %q, want %q", tt.messages, got, tt.want)
			}
		})
	}
}

func TestImportCommand_TopLevelErrorCarriesDetail(t *testing.T) {
	ctx := context.Background()

	t.Run("all-failed error names the count and the first failing path", func(t *testing.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		var importErr error
		captureStderr(t, func() {
			importErr = s.commands.importCommand(ctx, s, []string{"missing_a.csv", "missing_b.csv"})
		})
		if importErr == nil {
			t.Fatal("expected an error when all imports fail")
		}
		got := importErr.Error()
		if !strings.Contains(got, "all 2 import(s) failed") {
			t.Errorf("error %q should report the failed count", got)
		}
		if !strings.Contains(got, "missing_a.csv") {
			t.Errorf("error %q should name the first failing path", got)
		}
		if !strings.Contains(got, "+1 more") {
			t.Errorf("error %q should summarize the remaining failures", got)
		}
	})

	t.Run("partial-failed error keeps the sentinel and names the failing path", func(t *testing.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		var importErr error
		captureStderr(t, func() {
			importErr = s.commands.importCommand(ctx, s, []string{"testdata/user.csv", "missing_b.csv"})
		})
		if importErr == nil {
			t.Fatal("expected an error when one input fails")
		}
		if !errors.Is(importErr, errPartialImport) {
			t.Errorf("partial import error must remain detectable via errPartialImport: %v", importErr)
		}
		if !strings.Contains(importErr.Error(), "missing_b.csv") {
			t.Errorf("error %q should name the failing path", importErr.Error())
		}
	})
}

func TestShell_importDirectory_importsAndReportsTables(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "orders.csv"), []byte("id,total\n1,10\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var imported bool
	// Import progress goes to stderr, so capture stderr here.
	out := captureStderr(t, func() {
		imported, _, err = s.importDirectory(context.Background(), dir, "fixtures", "", false)
	})
	if err != nil {
		t.Fatalf("importDirectory returned error: %v", err)
	}
	if !imported {
		t.Fatal("importDirectory reported imported=false, want true")
	}
	if !strings.Contains(out, "Successfully imported 1 table(s) from directory fixtures") {
		t.Fatalf("output %q does not report a successful import", out)
	}
	if !strings.Contains(out, "orders") {
		t.Fatalf("output %q does not mention imported table name", out)
	}
	// The table maps to its real file, and a directory import is marked so
	// write-back rejects it.
	if !s.dirImported["orders"] {
		t.Error("orders should be marked as a directory import")
	}
	if src := s.tableSources["orders"]; !strings.HasSuffix(src, "orders.csv") {
		t.Errorf("orders source = %q, want it to end with orders.csv", src)
	}
}

func TestShell_importFile_excelSheetFiltering_dependsOnImportAndQueryUsecases(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)
	query := mock.NewMockQueryUsecase(ctrl)
	filePath := "report.xlsx"

	gomock.InOrder(
		importer.EXPECT().IsSupportedFile(filePath).Return(true),
		// Source tracking records the table-to-file mapping with a before/after diff.
		importer.EXPECT().GetTableNames(gomock.Any()).Return(nil, nil),
		importer.EXPECT().LoadFiles(gomock.Any(), filePath).Return(nil),
		importer.EXPECT().IsExcelFile(filePath).Return(true),
		importer.EXPECT().GetTableNameFromFilePath(filePath).Return("report"),
		importer.EXPECT().GetTableNames(gomock.Any()).Return([]*model.Table{
			model.NewTable("report_Summary", nil, nil),
			model.NewTable("report_Details", nil, nil),
		}, nil),
		importer.EXPECT().SanitizeForSQL("Summary").Return("Summary"),
		importer.EXPECT().QuoteIdentifier("report_Details").Return(`"report_Details"`),
		query.EXPECT().Exec(gomock.Any(), `DROP TABLE IF EXISTS "report_Details"`).Return(int64(0), nil),
		importer.EXPECT().GetTableNames(gomock.Any()).Return([]*model.Table{
			model.NewTable("report_Summary", nil, nil),
		}, nil),
	)

	s := newBoundaryTestShell(t, Usecases{
		importer: importer,
		query:    query,
	})

	if err := s.importFile(context.Background(), filePath, filePath, "Summary"); err != nil {
		t.Fatalf("importFile returned error: %v", err)
	}
}

func TestFilterExcelSheets_NoCollisionWithSimilarPrefix(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Simulate pre-existing tables from sales_q1.xlsx (prefix: sales_q1_)
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE sales_q1_Revenue (id INTEGER, amount REAL)")
	if err != nil {
		t.Fatalf("failed to create pre-existing table: %v", err)
	}
	_, err = s.usecases.query.Exec(ctx,
		"INSERT INTO sales_q1_Revenue VALUES (1, 100.0)")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Simulate tables from sales.xlsx (prefix: sales_)
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE sales_Summary (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.query.Exec(ctx,
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
	table, err := s.usecases.metadata.List(ctx, "sales_q1_Revenue")
	if err != nil {
		t.Fatalf("sales_q1_Revenue should still exist: %v", err)
	}
	if len(table.Records()) != 1 {
		t.Errorf("expected 1 record in sales_q1_Revenue, got %d", len(table.Records()))
	}

	// sales_Summary must be kept
	_, err = s.usecases.metadata.List(ctx, "sales_Summary")
	if err != nil {
		t.Fatalf("sales_Summary should still exist: %v", err)
	}

	// sales_Details must be dropped
	_, err = s.usecases.metadata.List(ctx, "sales_Details")
	if err == nil {
		t.Error("expected sales_Details to be dropped, but it still exists")
	}
}

func TestFilterExcelSheets_UnderscoreInFilename(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// sales_q1.xlsx produces tables with prefix "sales_q1_"
	// So sales_q1_Summary has sheet part "Summary" (after stripping "sales_q1_")
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE sales_q1_Summary (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE sales_q1_Details (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	err = s.filterExcelSheets(ctx, "sales_q1.xlsx", "Summary", nil)
	if err != nil {
		t.Fatalf("filterExcelSheets: %v", err)
	}

	// Summary should be kept
	_, err = s.usecases.metadata.List(ctx, "sales_q1_Summary")
	if err != nil {
		t.Fatalf("sales_q1_Summary should still exist: %v", err)
	}

	// Details should be dropped
	_, err = s.usecases.metadata.List(ctx, "sales_q1_Details")
	if err == nil {
		t.Error("expected sales_q1_Details to be dropped")
	}
}

func TestFilterExcelSheets_SheetNotFound(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE report_Sheet1 (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE report_Sheet2 (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	err = s.filterExcelSheets(ctx, "report.xlsx", "NonExistent", nil)
	if err == nil {
		t.Error("expected error for non-existent sheet, got nil")
	}

	// Both tables should be dropped
	_, err = s.usecases.metadata.List(ctx, "report_Sheet1")
	if err == nil {
		t.Error("expected report_Sheet1 to be dropped")
	}
	_, err = s.usecases.metadata.List(ctx, "report_Sheet2")
	if err == nil {
		t.Error("expected report_Sheet2 to be dropped")
	}
}

func TestFilterExcelSheets_ReimportWithSheet(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Simulate first import of report.xlsx (all sheets loaded)
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE report_Summary (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.query.Exec(ctx,
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
	_, err = s.usecases.metadata.List(ctx, "report_Summary")
	if err != nil {
		t.Fatalf("report_Summary should still exist: %v", err)
	}

	// Details should be dropped
	_, err = s.usecases.metadata.List(ctx, "report_Details")
	if err == nil {
		t.Error("expected report_Details to be dropped on re-import with --sheet")
	}
}

func TestImportDirectory_SheetDoesNotDropNonExcelTables(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()

	// Pre-load a CSV table that should survive --sheet filtering
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE users (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.query.Exec(ctx,
		"INSERT INTO users VALUES (1, 'Alice')")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate Excel tables that would be imported from a directory
	_, err = s.usecases.query.Exec(ctx,
		"CREATE TABLE workbook_Sheet1 (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.usecases.query.Exec(ctx,
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
	table, err := s.usecases.metadata.List(ctx, "users")
	if err != nil {
		t.Fatalf("users table should still exist: %v", err)
	}
	if len(table.Records()) != 1 {
		t.Errorf("expected 1 record in users, got %d", len(table.Records()))
	}

	// workbook_Sheet1 kept, workbook_Sheet2 dropped
	_, err = s.usecases.metadata.List(ctx, "workbook_Sheet1")
	if err != nil {
		t.Fatalf("workbook_Sheet1 should still exist: %v", err)
	}
	_, err = s.usecases.metadata.List(ctx, "workbook_Sheet2")
	if err == nil {
		t.Error("expected workbook_Sheet2 to be dropped")
	}
}

func TestImportFile_UnsupportedFormat(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tmpFile := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0o600); err != nil {
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
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tmpFile := filepath.Join(t.TempDir(), "people.csv")
	if err := os.WriteFile(tmpFile, []byte("id,name\n1,Alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := s.importFile(ctx, tmpFile, tmpFile, ""); err != nil {
		t.Fatalf("importFile: %v", err)
	}

	tables, err := s.usecases.importer.GetTableNames(ctx)
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
	tables, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Error("expected at least one table after Excel import with --sheet")
	}
}

func TestSheetMissErrors_AreDiagnostic(t *testing.T) {
	ctx := context.Background()
	sample := filepath.Join("..", "testdata", "sample.xlsx")
	accents := filepath.Join("..", "testdata", "sheet_with_accents.xlsx")

	t.Run("a non-Excel input with --sheet distinguishes validation failure and suggests recovery", func(t *testing.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		var importErr error
		captureStderr(t, func() {
			importErr = s.commands.importCommand(ctx, s, []string{filepath.Join("..", "testdata", "user.csv"), "--sheet", "Summary"})
		})
		if importErr == nil {
			t.Fatal("expected --sheet on a non-Excel input to fail")
		}
		got := importErr.Error()
		if !strings.Contains(got, "Excel") || !strings.Contains(got, "remove --sheet") {
			t.Errorf("error %q should explain --sheet needs an Excel input and how to recover", got)
		}
	})

	t.Run("a single-workbook miss names the workbook and suggests recovery", func(t *testing.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		err = s.importFile(ctx, sample, sample, "no_such_sheet")
		if err == nil {
			t.Fatal("expected a missing sheet to fail")
		}
		got := err.Error()
		if !strings.Contains(got, "sample.xlsx") {
			t.Errorf("error %q should name the checked workbook", got)
		}
		if !strings.Contains(got, "without --sheet") {
			t.Errorf("error %q should suggest re-importing without --sheet", got)
		}
	})

	t.Run("a multi-workbook miss names every checked workbook and suggests recovery", func(t *testing.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		var importErr error
		captureStderr(t, func() {
			importErr = s.commands.importCommand(ctx, s, []string{sample, accents, "--sheet", "no_such_sheet"})
		})
		if importErr == nil {
			t.Fatal("expected a missing sheet across workbooks to fail")
		}
		got := importErr.Error()
		if !strings.Contains(got, "sample.xlsx") || !strings.Contains(got, "sheet_with_accents.xlsx") {
			t.Errorf("error %q should name every checked workbook", got)
		}
		if !strings.Contains(got, "without --sheet") {
			t.Errorf("error %q should suggest re-importing without --sheet", got)
		}
	})
}

func TestImportDirectory_WithCSVFiles(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.csv"), []byte("id,val\n1,x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.tsv"), []byte("id\tval\n2\ty\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	imported, _, err := s.importDirectory(ctx, dir, dir, "", false)
	if err != nil {
		t.Fatalf("importDirectory: %v", err)
	}
	if !imported {
		t.Error("expected imported=true")
	}

	tables, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) < 2 {
		t.Errorf("expected at least 2 tables, got %d", len(tables))
	}
}

func TestImportCommand_TableNameCollision(t *testing.T) {
	// Regression for: two inputs that sanitize to the same table name must
	// fail instead of one silently overwriting the other.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	first := filepath.Join(dir, "a-b.csv")
	second := filepath.Join(dir, "a_b.csv")
	if err := os.WriteFile(first, []byte("id,name\n1,A\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("id,name\n2,B\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = s.commands.importCommand(context.Background(), s, []string{first, second})
	if err == nil {
		t.Fatal("importing two inputs with colliding sanitized names returned nil, want error")
	}
}

func TestImportCommand_ReimportSameFileIsNotACollision(t *testing.T) {
	// Re-importing the same source path is a harmless last-wins overwrite, not a
	// collision; it must not be rejected by the collision check.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	path := filepath.Join(t.TempDir(), "data.csv")
	if err := os.WriteFile(path, []byte("id,name\n1,A\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := s.commands.importCommand(context.Background(), s, []string{path}); err != nil {
		t.Fatalf("first import failed: %v", err)
	}
	if err := s.commands.importCommand(context.Background(), s, []string{path}); err != nil {
		t.Fatalf("re-import of the same file was rejected: %v", err)
	}
}

func TestImportCommand_PartialSuccess(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "ok.csv")
	if err := os.WriteFile(csvPath, []byte("id\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	// One valid file + one missing file → partial failure. The valid table is
	// still loaded, but importCommand returns errPartialImport so non-interactive
	// runs exit non-zero. The loaded table remains queryable.
	err = s.commands.importCommand(ctx, s, []string{csvPath, "missing.csv"})
	if !errors.Is(err, errPartialImport) {
		t.Errorf("expected errPartialImport for partial failure, got: %v", err)
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
		{"separated sheet flag", []string{"file.xlsx", "--sheet", "Summary"}, "Summary"},
		{"separated sheet flag with space value", []string{"--sheet", "Q1 Sales", "file.xlsx"}, "Q1 Sales"},
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

func TestImportCommand_MissingSheetValueErrors(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"sheet flag alone", []string{"--sheet"}},
		{"sheet flag at end after file", []string{"file.xlsx", "--sheet"}},
		{"sheet flag followed by another flag", []string{"--sheet", "--sheet=Data"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, cleanup, err := newShell(t, []string{"sqly"})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			if err := s.commands.importCommand(context.Background(), s, tt.argv); err == nil {
				t.Errorf("importCommand(%v) = nil error, want missing-value error", tt.argv)
			}
		})
	}
}

func TestSheetAppliesTo_UnreadableDirectoryDefersToImport(t *testing.T) {
	// An unreadable directory cannot be proven to lack Excel files, so --sheet
	// validation must defer to the import step (which reports the real access
	// error) instead of misclassifying it as a non-Excel input.
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits behave differently on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can traverse a 0000 directory, so the permission error never occurs")
	}

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	parent := t.TempDir()
	locked := filepath.Join(parent, "locked")
	if err := os.Mkdir(locked, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	// Restore permissions so t.TempDir cleanup can remove the directory.
	defer func() { _ = os.Chmod(locked, 0o750) }() //nolint:gosec // restore dir perms for cleanup

	if !s.sheetAppliesTo([]string{locked}) {
		t.Error("sheetAppliesTo(unreadable dir) = false, want true (defer to import for the real error)")
	}
}

func TestImportCommand_SheetSkipsWorkbooksMissingSheet(t *testing.T) {
	// A multi-workbook import with --sheet must skip workbooks that lack the
	// requested sheet instead of failing the whole import, so matching workbooks
	// still load.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s.argument.SheetName = "A test"

	ctx := context.Background()
	paths := []string{
		filepath.Join("testdata", "sheet_with_spaces.xlsx"),  // contains "A test"
		filepath.Join("testdata", "sample.xlsx"),             // lacks "A test"
		filepath.Join("testdata", "sheet_with_accents.xlsx"), // lacks "A test"
	}
	if err := s.commands.importCommand(ctx, s, paths); err != nil {
		t.Fatalf("importCommand with --sheet across multiple workbooks = %v, want nil (skip non-matching)", err)
	}

	tables, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var kept bool
	for _, tbl := range tables {
		if strings.HasPrefix(tbl.Name(), "sheet_with_spaces_") {
			kept = true
		}
		if strings.HasPrefix(tbl.Name(), "sample_") || strings.HasPrefix(tbl.Name(), "sheet_with_accents_") {
			t.Errorf("table %q from a non-matching workbook should have been dropped", tbl.Name())
		}
	}
	if !kept {
		t.Errorf("expected a table from sheet_with_spaces.xlsx (the matching workbook), got tables %v", tableNamesOf(tables))
	}
}

func tableNamesOf(tables []*model.Table) []string {
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name())
	}
	return names
}

func TestImportCommand_EmptySheetValueRejected(t *testing.T) {
	// An explicit empty helper --sheet value (separated "" or joined "--sheet=")
	// must be rejected instead of silently importing every sheet. The rejection
	// must happen before file/Excel checks, so it surfaces even for a CSV input.
	csv := filepath.Join("testdata", "sample.csv")
	tests := []struct {
		name string
		argv []string
	}{
		{"separated empty sheet value", []string{"--sheet", "", csv}},
		{"joined empty sheet value", []string{"--sheet=", csv}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, cleanup, err := newShell(t, []string{"sqly"})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			err = s.commands.importCommand(context.Background(), s, tt.argv)
			if err == nil {
				t.Fatalf("importCommand(%v) = nil error, want empty-sheet error", tt.argv)
			}
			if !strings.Contains(err.Error(), "sheet") {
				t.Errorf("importCommand(%v) error = %q, want it to mention the empty sheet value", tt.argv, err)
			}
		})
	}
}

func TestValidatePath_Import(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		unixOnly bool
	}{
		{"normal path", "testdata/sample.csv", false, false},
		{"relative path", "./foo/bar.csv", false, false},
		{"path traversal", "../../../etc/passwd", true, false},
		// A literal filename containing "..%2f" is not traversal: the filesystem
		// never URL-decodes it, so it must be accepted.
		{"literal ..%2f filename", "data/..%2fuser.csv", false, false},
		// Deep paths must import regardless of nesting depth.
		{"deep path", "a/b/c/d/e/f/g/h/i/j/k/user.csv", false, false},
		{"system dir /etc", "/etc/hosts", true, true},
		{"system dir /proc", "/proc/cpuinfo", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.unixOnly && runtime.GOOS == config.Windows {
				t.Skip("Unix-only system directory check")
			}
			_, err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

// TestValidatePath_Symlink verifies that the system-directory guard follows
// symlinks: an alias to a blocked target must be rejected just like the direct
// path, while an alias to an ordinary user file must still import. The guard
// normalizes the macOS /private prefix, so this runs on Linux and macOS alike.
func TestValidatePath_Symlink(t *testing.T) {
	if runtime.GOOS == config.Windows {
		t.Skip("Unix-only system directory check")
	}
	t.Parallel()

	dir := t.TempDir()

	t.Run("symlink alias to /etc/hosts is rejected like the direct path", func(t *testing.T) {
		t.Parallel()
		if _, err := os.Stat("/etc/hosts"); err != nil {
			t.Skip("/etc/hosts is not present on this host")
		}
		link := filepath.Join(dir, "hosts_alias.csv")
		if err := os.Symlink("/etc/hosts", link); err != nil {
			t.Fatalf("os.Symlink: %v", err)
		}
		if _, err := validatePath(link); err == nil {
			t.Errorf("validatePath(%q) = nil error, want rejection of a symlink to a blocked system path", link)
		}
	})

	t.Run("symlink to an ordinary user file is accepted", func(t *testing.T) {
		t.Parallel()
		target := filepath.Join(dir, "real.csv")
		if err := os.WriteFile(target, []byte("a,b\n1,2\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile: %v", err)
		}
		link := filepath.Join(dir, "user_alias.csv")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("os.Symlink: %v", err)
		}
		if _, err := validatePath(link); err != nil {
			t.Errorf("validatePath(%q) = %v, want nil for a symlink to a user file", link, err)
		}
	})
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

// TestStagePseudoFileScopedToPseudoFiles verifies that the pseudo-file CSV
// staging added for/ is scoped to the allowed Unix pseudo-files only: a
// normal extensionless file is not silently treated as CSV but still fails as an
// unsupported format.
func TestStagePseudoFileScopedToPseudoFiles(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()

	// A regular file without a recognized extension is not a pseudo-file, so
	// staging must decline it and import must report an unsupported format.
	dir := t.TempDir()
	plain := filepath.Join(dir, "noext")
	if err := os.WriteFile(plain, []byte("name,score\na,1\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, ok := shell.stagePseudoFileAsCSV(plain); ok {
		t.Errorf("stagePseudoFileAsCSV staged a non-pseudo extensionless file %q, want it declined", plain)
	}

	backout, backerr := config.Stdout, config.Stderr
	config.Stdout = &bytes.Buffer{}
	config.Stderr = &bytes.Buffer{}
	defer func() { config.Stdout, config.Stderr = backout, backerr }()
	if err := shell.importFile(context.Background(), plain, plain, ""); err == nil {
		t.Error("importFile accepted a non-pseudo extensionless file, want an unsupported-format error")
	}
}
