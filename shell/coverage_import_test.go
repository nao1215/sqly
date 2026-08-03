package shell

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestIsAllDigits_Cov exercises the numeric /proc/<pid> component check.
func TestIsAllDigits_Cov(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"0", true},
		{"123", true},
		{"12a", false},
		{"1 2", false},
		{"a", false},
	}
	for _, tt := range tests {
		if got := isAllDigits(tt.in); got != tt.want {
			t.Errorf("isAllDigits(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestEndsInsideBlockComment_Cov drives the quote- and comment-aware scanner
// through every state so a "/*" opener is only honored outside strings and line
// comments.
func TestEndsInsideBlockComment_Cov(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"open block", "SELECT 1 /* open", true},
		{"closed block", "SELECT 1 /* closed */", false},
		{"opener in single quote", "SELECT '/*'", false},
		{"opener in double quote", "SELECT \"/*\"", false},
		{"opener in backtick", "SELECT `/*`", false},
		{"opener in bracket", "SELECT [/*]", false},
		{"opener in line comment", "-- /* not a block\n", false},
		{"line comment then open block", "-- note\n/* open", true},
		{"plain line comment", "SELECT 1 -- trailing", false},
	}
	for _, tt := range tests {
		if got := endsInsideBlockComment(tt.in); got != tt.want {
			t.Errorf("%s: endsInsideBlockComment(%q) = %v, want %v", tt.name, tt.in, got, tt.want)
		}
	}
}

// TestWithMainVerb_Cov confirms that the main verb of a WITH statement is read at
// parenthesis depth 0, skipping CTE bodies, quoted identifiers, and comments.
func TestWithMainVerb_Cov(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"cte then update", "WITH cte AS (SELECT 1) UPDATE t SET x=1", "UPDATE"},
		{"cte then select", "WITH cte AS (SELECT 1) SELECT * FROM cte", "SELECT"},
		{"cte then insert", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte", "INSERT"},
		{"cte then delete", "WITH cte AS (SELECT 1) DELETE FROM t", "DELETE"},
		{"cte then replace", "WITH cte AS (SELECT 1) REPLACE INTO t VALUES(1)", "REPLACE"},
		{"cte then values", "WITH cte AS (SELECT 1) VALUES (1)", "VALUES"},
		{"line comment before verb", "WITH cte AS (SELECT 1) -- note\nUPDATE t SET x=1", "UPDATE"},
		{"block comment before verb", "WITH cte AS (SELECT 1) /* c */ UPDATE t SET x=1", "UPDATE"},
		{"keyword hidden in single quote", "WITH cte AS (SELECT 1) SELECT 'UPDATE'", "SELECT"},
		{"double-quoted cte name", "WITH \"c\" AS (SELECT 1) SELECT 1", "SELECT"},
		{"backtick cte name", "WITH `c` AS (SELECT 1) SELECT 1", "SELECT"},
		{"bracket cte name", "WITH [c] AS (SELECT 1) SELECT 1", "SELECT"},
		{"no main verb", "WITH cte AS (SELECT 1)", ""},
	}
	for _, tt := range tests {
		if got := withMainVerb(tt.in); got != tt.want {
			t.Errorf("%s: withMainVerb(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

// TestTablesMatchingFile_Cov confirms a single-table format claims only its exact
// table name, while a multi-table (ACH) format also claims its "<base>_" prefixed
// tables.
func TestTablesMatchingFile_Cov(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	csvNames := map[string]struct{}{"data": {}, "data_extra": {}}
	got := s.tablesMatchingFile("data.csv", csvNames)
	if len(got) != 1 || got[0] != "data" {
		t.Errorf("csv tablesMatchingFile = %v, want [data] only (no prefix claim)", got)
	}

	achNames := map[string]struct{}{"pay": {}, "pay_entries": {}, "pay_batches": {}, "other": {}}
	got = s.tablesMatchingFile("pay.ach", achNames)
	set := map[string]bool{}
	for _, n := range got {
		set[n] = true
	}
	if !set["pay"] || !set["pay_entries"] || !set["pay_batches"] || set["other"] {
		t.Errorf("ach tablesMatchingFile = %v, want base plus pay_ prefixed tables", got)
	}
}

// TestIsRecordedSource_Cov confirms the stdin sentinel is skipped and a real
// recorded source path is recognized.
func TestIsRecordedSource_Cov(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	src := filepath.Join(dir, "real.csv")
	if err := os.WriteFile(src, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.tableSources = map[string]string{
		"fromStdin": stdinTableSource,
		"fromFile":  src,
	}

	if !s.isRecordedSource(src) {
		t.Error("a recorded file source should be recognized")
	}
	if s.isRecordedSource(filepath.Join(dir, "other.csv")) {
		t.Error("an unrecorded path should not be recognized")
	}
	// The stdin sentinel must be skipped, not matched as a path.
	if s.isRecordedSource(stdinTableSource) {
		t.Error("the stdin sentinel must not be treated as a recorded file source")
	}
}

// TestTableChanged_Cov exercises the unknown-table, unchanged, and changed paths.
func TestTableChanged_Cov(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "t.csv")
	if err := os.WriteFile(src, []byte("id,name\n1,a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	_ = captureStderr(t, func() {
		if impErr := s.commands.importCommand(context.Background(), s, []string{src}); impErr != nil {
			t.Fatalf("import: %v", impErr)
		}
	})

	if !s.tableChanged(context.Background(), "no_such_table") {
		t.Error("a table with no baseline must count as changed")
	}
	if s.tableChanged(context.Background(), "t") {
		t.Error("a freshly imported table must match its baseline (unchanged)")
	}
	_ = captureStdout(t, func() {
		if execErr := s.exec(context.Background(), "UPDATE t SET name='b' WHERE id=1"); execErr != nil {
			t.Fatalf("update: %v", execErr)
		}
	})
	if !s.tableChanged(context.Background(), "t") {
		t.Error("a modified table must count as changed")
	}
}

// TestStagePseudoFileAsCSV_Success covers the successful staging path using a file
// under /dev/shm, which isAllowedPseudoFile treats as a legitimate pseudo-file.
func TestStagePseudoFileAsCSV_Success(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/dev/shm pseudo-file staging is Linux-specific")
	}
	if info, err := os.Stat("/dev/shm"); err != nil || !info.IsDir() {
		t.Skip("/dev/shm is not available")
	}

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	pseudo := filepath.Join("/dev/shm", "sqly-cov-pseudo.csv-data")
	if err := os.WriteFile(pseudo, []byte("id,name\n1,a\n"), 0o600); err != nil {
		t.Skipf("cannot write under /dev/shm: %v", err)
	}
	defer func() { _ = os.Remove(pseudo) }()

	staged, cleanupStage, ok := s.stagePseudoFileAsCSV(pseudo)
	if !ok {
		t.Fatalf("stagePseudoFileAsCSV declined an allowed /dev/shm pseudo-file %q", pseudo)
	}
	defer cleanupStage()
	if !strings.HasSuffix(staged, model.ExtCSV) {
		t.Errorf("staged path %q should carry a .csv extension", staged)
	}
	if _, err := os.Stat(staged); err != nil {
		t.Errorf("staged file was not created: %v", err)
	}
}

// TestSaveFinancialSetToDir reconstructs an ACH file from its table set into a
// --save destination directory, covering planFinancialSet, executeWriteBack's
// financial branch, and writeFinancialSet.
func TestSaveFinancialSetToDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "ppd-debit.ach")
	copyTestFile(t, "ppd-debit.ach", src)

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	_ = captureStderr(t, func() {
		if impErr := s.commands.importCommand(context.Background(), s, []string{src}); impErr != nil {
			t.Fatalf("import ACH: %v", impErr)
		}
	})

	// Force the set to count as changed independent of the ACH schema: drop a
	// member's baseline so tableChanged reports it changed, and set the
	// session-level flag that .save gates on. The write path still reconstructs the
	// whole set from its tables.
	delete(s.importBaseline, "ppd_debit_entries")
	s.dataChanged = true

	outDir := filepath.Join(dir, "out")
	stderr := captureStderr(t, func() {
		if saveErr := s.commands.saveCommand(context.Background(), s, []string{outDir}); saveErr != nil {
			t.Fatalf(".save DIR for ACH set: %v", saveErr)
		}
	})

	saved := filepath.Join(outDir, "ppd-debit.ach")
	if _, statErr := os.Stat(saved); statErr != nil {
		t.Fatalf("expected the ACH set to be written to %s, stderr=%q", saved, stderr)
	}
	if !strings.Contains(stderr, "Saved ACH set") {
		t.Errorf("stderr = %q, want an 'Saved ACH set' confirmation", stderr)
	}
}

// TestSaveFinancialSetFedwireInPlace covers the Fedwire branch of
// writeFinancialSet and an in-place (destDir=="") financial save.
func TestSaveFinancialSetFedwireInPlace(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "customer-transfer.fed")
	copyTestFile(t, "customer-transfer.fed", src)

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	_ = captureStderr(t, func() {
		if impErr := s.commands.importCommand(context.Background(), s, []string{src}); impErr != nil {
			t.Fatalf("import Fedwire: %v", impErr)
		}
	})

	// Force a change for the sole member table so the set is written.
	delete(s.importBaseline, "customer_transfer_message")
	s.dataChanged = true

	stderr := captureStderr(t, func() {
		if saveErr := s.commands.saveCommand(context.Background(), s, []string{inPlaceArg}); saveErr != nil {
			t.Fatalf(".save --in-place for Fedwire set: %v", saveErr)
		}
	})
	if !strings.Contains(stderr, "Saved FED set") {
		t.Errorf("stderr = %q, want a 'Saved FED set' confirmation", stderr)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("in-place Fedwire save removed the source: %v", statErr)
	}
}
