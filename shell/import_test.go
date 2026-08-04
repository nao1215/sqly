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
	"syscall"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
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
	imported, err := s.importDirectory(context.Background(), emptyDir, emptyDir)
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
	imported, err := s.importDirectory(ctx, dir, dir)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if !imported {
		t.Error("expected first import to succeed")
	}

	// Re-importing the same directory overwrites the existing table. The
	// directory still contains a supported file, so the import is reported as
	// successful (it overwrote data) rather than as "No supported files". Ref
	imported, err = s.importDirectory(ctx, dir, dir)
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
	if _, err := s.importDirectory(ctx, dir, dir); err != nil {
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

	_, err = s.importDirectory(context.Background(), dir, dir)
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

	_, err = s.importDirectory(context.Background(), dir, dir)
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
	// a directory import, so later .save --in-place cannot write the directory rows
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
	if err := s.importFile(ctx, orig, orig); err != nil {
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

	imported, err := s.importDirectory(ctx, dir, dir)
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
	// then .save --in-place must refuse to write back a directory import, leaving the
	// original untouched.
	if err := s.exec(ctx, "INSERT INTO user VALUES ('alt2',2,'ALT','Two')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.writeBack(ctx, "", false); err == nil {
		t.Error("expected .save --in-place to be rejected for a directory-imported table")
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

// TestImportCommand_StopsAtTheFirstUnreadableInput replaces a test of the
// helper that condensed a list of per-input failures into one line. There is no
// list any more: an import stops at the first input it cannot read, so the
// message names that one and the inputs after it are never opened.
func TestImportCommand_StopsAtTheFirstUnreadableInput(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	good := filepath.Join(dir, "good.csv")
	if err := os.WriteFile(good, []byte("v\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = s.commands.importCommand(context.Background(), s,
		[]string{good, filepath.Join(dir, "first-missing.csv"), filepath.Join(dir, "second-missing.csv")})
	if err == nil {
		t.Fatal("importCommand succeeded with two unreadable inputs")
	}
	if !strings.Contains(err.Error(), "first-missing.csv") {
		t.Errorf("error %q should name the first input it could not read", err)
	}
	if strings.Contains(err.Error(), "second-missing.csv") {
		t.Errorf("error %q reports an input after the failure; the import stops at the first", err)
	}
	if got := ExitCode(err); got != ExitInput {
		t.Errorf("ExitCode = %d, want %d", got, ExitInput)
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
		if !strings.Contains(got, "no table was created or changed") {
			t.Errorf("error %q should say the import left nothing behind", got)
		}
		if !strings.Contains(got, "missing_a.csv") {
			t.Errorf("error %q should name the first failing path", got)
		}
	})

	// One good input and one bad one used to be a "partial" import that kept the
	// good table. It is now a failure that keeps nothing, and the error still has
	// to name the input that caused it.
	t.Run("one unreadable input fails the whole import and names it", func(t *testing.T) {
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
		var failed *importFailedError
		if !errors.As(importErr, &failed) {
			t.Errorf("a failed import must stay detectable as one: %v", importErr)
		}
		if got := ExitCode(importErr); got != ExitInput {
			t.Errorf("ExitCode = %d, want %d", got, ExitInput)
		}
		if !strings.Contains(importErr.Error(), "missing_b.csv") {
			t.Errorf("error %q should name the failing path", importErr.Error())
		}
		// The good input must not have survived the failure.
		tables, err := s.usecases.importer.GetTableNames(ctx)
		if err != nil {
			t.Fatalf("list tables: %v", err)
		}
		for _, table := range tables {
			if table.Name() == "user" {
				t.Error("the table from the readable input survived a failed import")
			}
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
		imported, err = s.importDirectory(context.Background(), dir, "fixtures")
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

	err = s.importFile(context.Background(), tmpFile, tmpFile)
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
	if err := s.importFile(ctx, tmpFile, tmpFile); err != nil {
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

	err = s.importFile(context.Background(), "/nonexistent/file.csv", "/nonexistent/file.csv")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
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
	imported, err := s.importDirectory(ctx, dir, dir)
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

// TestImportCommand_NoPartialSuccess is the inversion of a test that asserted
// the opposite. One valid file and one missing file used to leave the valid
// table loaded and report a partial failure; it now leaves nothing at all,
// which is what makes a failed import something a caller can act on rather than
// something they have to inspect the database to understand.
func TestImportCommand_NoPartialSuccess(t *testing.T) {
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
	err = s.commands.importCommand(ctx, s, []string{csvPath, "missing.csv"})
	if err == nil {
		t.Fatal("importCommand succeeded with a missing input")
	}
	var failed *importFailedError
	if !errors.As(err, &failed) {
		t.Errorf("error = %v, want an importFailedError", err)
	}

	tables, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	for _, table := range tables {
		if table.Name() == "ok" {
			t.Fatal("the readable input's table survived a failed import")
		}
	}
	if _, recorded := s.tableSources["ok"]; recorded {
		t.Error("a failed import left a source recorded for a table that does not exist")
	}
	if _, baseline := s.importBaseline["ok"]; baseline {
		t.Error("a failed import left a content baseline behind")
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
	if err := shell.importFile(context.Background(), plain, plain); err == nil {
		t.Error("importFile accepted a non-pseudo extensionless file, want an unsupported-format error")
	}
}

func TestUnfetchableURLScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"s3 url", "s3://bucket/data.csv", "s3"},
		{"file url", "file:///tmp/user.csv", "file"},
		{"ftp url", "ftp://example.com/a.csv", "ftp"},
		{"scheme case is normalized", "S3://bucket/data.csv", "s3"},
		{"scheme may carry digits and punctuation", "git+ssh://host/repo", "git+ssh"},
		// http and https are downloaded, so they are not unfetchable.
		{"http url", "http://example.com/a.csv", ""},
		{"https url", "https://example.com/a.csv", ""},
		{"uppercase http url", "HTTP://example.com/a.csv", ""},
		// Local paths that only look URL-ish must stay local paths, so the caller
		// keeps reporting them as missing files.
		{"plain relative path", "data.csv", ""},
		{"plain absolute path", "/tmp/data.csv", ""},
		{"colon in file name", "weird:name.csv", ""},
		{"windows drive path", "C://data.csv", ""},
		{"scheme starting with a digit", "1a://host/x", ""},
		{"empty scheme", "://host/x", ""},
		{"empty input", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := unfetchableURLScheme(tt.input); got != tt.want {
				t.Errorf("unfetchableURLScheme(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestLocalImportAccessError_URLSchemeReportedOnAnyError pins that an input
// written as a URL sqly cannot fetch is named by its scheme regardless of which
// error the filesystem produced. The scheme check used to sit inside the
// not-exist branch, which is what Unix returns; Windows rejects
// "s3://bucket/x.csv" as an invalid filename instead, so the explanation never
// appeared on the platform where the raw error is least readable.
func TestLocalImportAccessError_URLSchemeReportedOnAnyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"not exist, as Unix reports it", os.ErrNotExist},
		{"invalid filename, as Windows reports it", syscall.EINVAL},
		{"permission denied", os.ErrPermission},
		{"an unclassified error", errors.New("some other failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := localImportAccessError("s3://bucket/data.csv", tt.err).Error()
			if !strings.Contains(got, "only http and https URLs") {
				t.Errorf("localImportAccessError(...) = %q, want it to name the unsupported scheme", got)
			}
			if !strings.Contains(got, `"s3"`) {
				t.Errorf("localImportAccessError(...) = %q, want it to quote the scheme", got)
			}
		})
	}
}

// TestLocalImportAccessError_PlainPathsKeepTheirMessage keeps the scheme check
// from swallowing the ordinary path errors.
func TestLocalImportAccessError_PlainPathsKeepTheirMessage(t *testing.T) {
	t.Parallel()

	if got := localImportAccessError("data.csv", os.ErrNotExist).Error(); got != "path does not exist: data.csv" {
		t.Errorf("missing file = %q, want the path-does-not-exist message", got)
	}
	if got := localImportAccessError("data.csv", os.ErrPermission).Error(); !strings.Contains(got, "permission denied") {
		t.Errorf("permission error = %q, want the permission message", got)
	}
}
