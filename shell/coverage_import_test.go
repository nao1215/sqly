package shell

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// covImpArg builds a minimal *Shell whose only wired dependency is argument, for
// unit tests of methods that read s.argument alone (no usecases, no globals).
func covImpArg(arg *config.Arg) *Shell {
	return &Shell{argument: arg}
}

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

// TestValidateProfileFlags_Cov checks that --profile is rejected when combined
// with any flag that asks for a different action or side effect.
func TestValidateProfileFlags_Cov(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		arg     *config.Arg
		wantErr string
	}{
		{"not profile", &config.Arg{ProfileFlag: false, InspectFlag: true}, ""},
		{"clean profile", &config.Arg{ProfileFlag: true}, ""},
		{"conflict inspect", &config.Arg{ProfileFlag: true, InspectFlag: true}, "--inspect"},
		{"conflict compare", &config.Arg{ProfileFlag: true, CompareFlag: true}, "--compare"},
		{"conflict sql", &config.Arg{ProfileFlag: true, Query: "SELECT 1"}, "--sql"},
		{"conflict sql-file", &config.Arg{ProfileFlag: true, SQLFilePath: "q.sql"}, "--sql-file"},
		{"conflict output", &config.Arg{ProfileFlag: true, Output: &config.Output{FilePath: "o.csv", Mode: model.PrintModeTable}}, "--output"},
		{"conflict save", &config.Arg{ProfileFlag: true, SaveInPlace: true}, "--save"},
		{"conflict save-dir", &config.Arg{ProfileFlag: true, SaveDir: "out"}, "--save-dir"},
		{"conflict output mode", &config.Arg{ProfileFlag: true, Output: &config.Output{Mode: model.PrintModeCSV}}, "output mode flag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := covImpArg(tt.arg).validateProfileFlags()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateProfileFlags() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateProfileFlags() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateProfileFlags() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestRenderProfileText_Cov renders a multi-table report so the blank-line
// separator between tables and the per-column warning lines are both exercised.
func TestRenderProfileText_Cov(t *testing.T) {
	t.Parallel()
	report := profileReport{Tables: []profileTable{
		{
			Name: "users", RowCount: 2, ColumnCount: 1,
			Columns: []profileColumn{
				{Name: "id", Type: "INTEGER", NullCount: 0, BlankCount: 0, DistinctCount: 2, NumericCount: 2, Warnings: []string{}},
			},
		},
		{
			Name: "notes", RowCount: 1, ColumnCount: 1,
			Columns: []profileColumn{
				{Name: "note", Type: "TEXT", DistinctCount: 1, Warnings: []string{"looks like null placeholders"}},
			},
		},
	}}
	got := renderProfileText(report)
	if !strings.Contains(got, "table users: 2 rows, 1 columns") {
		t.Errorf("missing users header:\n%s", got)
	}
	if !strings.Contains(got, "table notes: 1 rows, 1 columns") {
		t.Errorf("missing notes header:\n%s", got)
	}
	if !strings.Contains(got, "warning: looks like null placeholders") {
		t.Errorf("missing warning line:\n%s", got)
	}
	// Two tables must be separated by a blank line (the ti>0 branch).
	if !strings.Contains(got, "\n\ntable notes") {
		t.Errorf("expected a blank line between tables:\n%s", got)
	}
}

// TestIsCacheArtifact_Cov verifies the cache database and its manifest sidecar are
// recognized as sqly's own artifacts, and unrelated inputs are not.
func TestIsCacheArtifact_Cov(t *testing.T) {
	t.Parallel()
	cache := filepath.Join(t.TempDir(), "snap.cache")

	if covImpArg(&config.Arg{}).isCacheArtifact(cache) {
		t.Error("with no --cache set, no path should be a cache artifact")
	}

	s := covImpArg(&config.Arg{CachePath: cache})
	if !s.isCacheArtifact(cache) {
		t.Error("the cache database path must be a cache artifact")
	}
	if !s.isCacheArtifact(cacheManifestPath(cache)) {
		t.Error("the manifest sidecar path must be a cache artifact")
	}
	if s.isCacheArtifact(filepath.Join(filepath.Dir(cache), "data.csv")) {
		t.Error("an unrelated input must not be a cache artifact")
	}
}

// TestCacheEnabled_Cov covers the opt-out conditions for the import cache.
func TestCacheEnabled_Cov(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csv := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csv, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ach := filepath.Join(dir, "pay.ach")
	if err := os.WriteFile(ach, []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}

	if covImpArg(&config.Arg{}).cacheEnabled([]string{csv}) {
		t.Error("cache must be disabled when --cache is unset")
	}
	if covImpArg(&config.Arg{CachePath: "c"}).cacheEnabled(nil) {
		t.Error("cache must be disabled when there are no input paths")
	}
	if covImpArg(&config.Arg{CachePath: "c", StdinFormat: "csv"}).cacheEnabled([]string{csv}) {
		t.Error("cache must be disabled for a --stdin dataset")
	}
	if covImpArg(&config.Arg{CachePath: "c"}).cacheEnabled([]string{ach}) {
		t.Error("cache must be disabled when an input is an ACH/Fedwire file")
	}
	if !covImpArg(&config.Arg{CachePath: "c"}).cacheEnabled([]string{csv}) {
		t.Error("cache must be enabled for a plain file input with --cache set")
	}
}

// TestHashFile_Cov checks a stable digest for known content and an error for a
// missing file.
func TestHashFile_Cov(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	// SHA-256 of "hello".
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	got, err := hashFile(f)
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}
	if got != want {
		t.Errorf("hashFile = %q, want %q", got, want)
	}
	if _, err := hashFile(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("hashFile of a missing file should return an error")
	}
}

// TestReadWriteCacheManifest_Cov round-trips a manifest and covers the read/write
// error paths.
func TestReadWriteCacheManifest_Cov(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.cache.manifest.json")
	want := cacheManifest{
		Version:      cacheManifestVersion,
		Sources:      []cacheSource{{Path: "/a.csv", Size: 3, Hash: "abc"}},
		TableSources: map[string]string{"a": "/a.csv"},
		DirImported:  []string{"a"},
	}
	if err := writeCacheManifest(path, want); err != nil {
		t.Fatalf("writeCacheManifest: %v", err)
	}
	got, err := readCacheManifest(path)
	if err != nil {
		t.Fatalf("readCacheManifest: %v", err)
	}
	if got.Version != want.Version || len(got.Sources) != 1 || got.Sources[0] != want.Sources[0] {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}

	if _, err := readCacheManifest(filepath.Join(dir, "nope.json")); err == nil {
		t.Error("readCacheManifest of a missing file should error")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCacheManifest(bad); err == nil {
		t.Error("readCacheManifest of invalid JSON should error")
	}
	// A destination whose parent directory does not exist cannot be written.
	if err := writeCacheManifest(filepath.Join(dir, "missing-dir", "m.json"), want); err == nil {
		t.Error("writeCacheManifest into a nonexistent directory should error")
	}
}

// TestCollectCacheSignatures_SkipAndDirSupported covers the skip predicate, the
// dirSupported filter for directory walks, and the missing-path error.
func TestCollectCacheSignatures_SkipAndDirSupported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.csv")
	drop := filepath.Join(dir, "drop.csv")
	note := filepath.Join(dir, "note.txt")
	for _, p := range []string{keep, drop, note} {
		if err := os.WriteFile(p, []byte("a\n1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Directory walk with a dirSupported filter that excludes the .txt sibling.
	dirSupported := func(path string) bool { return strings.HasSuffix(path, ".csv") }
	sigs, err := collectCacheSignatures([]string{dir}, nil, dirSupported)
	if err != nil {
		t.Fatalf("collectCacheSignatures: %v", err)
	}
	if len(sigs) != 2 {
		t.Fatalf("expected 2 signatures (the two csv files), got %d: %+v", len(sigs), sigs)
	}

	// Directly named files with a skip predicate that drops one of them.
	skip := func(path string) bool { return strings.HasSuffix(path, "drop.csv") }
	sigs, err = collectCacheSignatures([]string{keep, drop}, skip, nil)
	if err != nil {
		t.Fatalf("collectCacheSignatures: %v", err)
	}
	if len(sigs) != 1 || !strings.HasSuffix(sigs[0].Path, "keep.csv") {
		t.Fatalf("skip predicate did not drop drop.csv: %+v", sigs)
	}

	if _, err := collectCacheSignatures([]string{filepath.Join(dir, "missing.csv")}, nil, nil); err == nil {
		t.Error("collectCacheSignatures of a missing path should error")
	}
}

// TestClearCache_Cov removes both the cache database and its manifest, ignoring a
// missing file.
func TestClearCache_Cov(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "snap.cache")
	manifest := cacheManifestPath(cache)
	for _, p := range []string{cache, manifest} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Suppress the (unexpected) stderr path; the happy path prints nothing.
	_ = captureStderr(t, func() {
		(&Shell{}).clearCache(cache)
	})
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Error("clearCache did not remove the cache database")
	}
	if _, err := os.Stat(manifest); !os.IsNotExist(err) {
		t.Error("clearCache did not remove the manifest")
	}
	// A second call on already-removed files is a no-op and must not panic.
	_ = captureStderr(t, func() {
		(&Shell{}).clearCache(cache)
	})
}

// TestWriteCache_MkdirFails covers the branch where the cache directory cannot be
// created because a parent path component is a regular file.
func TestWriteCache_MkdirFails(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// cachePath's parent directory is "<file>/sub", which cannot be created because
	// "<file>" is a regular file.
	cachePath := filepath.Join(fileAsParent, "sub", "snap.cache")
	stderr := captureStderr(t, func() {
		(&Shell{}).writeCache(context.Background(), cachePath, nil)
	})
	if !strings.Contains(stderr, "cannot create cache directory") {
		t.Errorf("stderr = %q, want a cache-directory creation warning", stderr)
	}
}

// TestRestoreFromManifest_Cov rebuilds per-table session state from a manifest,
// including initializing nil maps.
func TestRestoreFromManifest_Cov(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	_ = captureStderr(t, func() {
		if impErr := s.commands.importCommand(context.Background(), s, []string{filepath.Join("testdata", "sample.csv")}); impErr != nil {
			t.Fatalf("import: %v", impErr)
		}
	})

	// Force the nil-map initialization branches.
	s.tableSources = nil
	s.dirImported = nil

	manifest := cacheManifest{
		Version:      cacheManifestVersion,
		TableSources: map[string]string{"sample": "/some/where/sample.csv"},
		DirImported:  []string{"sample"},
	}
	s.restoreFromManifest(context.Background(), manifest)

	if s.tableSources["sample"] != "/some/where/sample.csv" {
		t.Errorf("tableSources not restored: %v", s.tableSources)
	}
	if !s.dirImported["sample"] {
		t.Error("dirImported not restored")
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

// TestExcelWorkbooks_Cov covers the directory-walk branch, the direct-file branch,
// and the stat-error skip.
func TestExcelWorkbooks_Cov(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	copyTestFile(t, "sample.xlsx", filepath.Join(dir, "in-dir.xlsx"))
	standalone := filepath.Join(dir, "standalone.xlsx")
	copyTestFile(t, "sample.xlsx", standalone)
	// Also drop a non-Excel file so the walk filters it out.
	if err := os.WriteFile(filepath.Join(dir, "note.csv"), []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	books := s.excelWorkbooks([]string{dir, standalone, filepath.Join(dir, "missing")})
	var inDir, sole bool
	for _, b := range books {
		if strings.HasSuffix(b, "in-dir.xlsx") {
			inDir = true
		}
		if b == standalone {
			sole = true
		}
	}
	if !inDir {
		t.Errorf("excelWorkbooks did not find the workbook inside the directory: %v", books)
	}
	if !sole {
		t.Errorf("excelWorkbooks did not find the standalone workbook: %v", books)
	}
}

// TestSupportedFilesInDir_SkipsCacheArtifacts verifies a --cache manifest that
// lands inside the imported directory is not returned as a dataset input even
// though .json is a supported format.
func TestSupportedFilesInDir_SkipsCacheArtifacts(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "snap.cache")
	data := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(data, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheManifestPath(cache), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s.argument.CachePath = cache

	files, err := s.supportedFilesInDir(dir)
	if err != nil {
		t.Fatalf("supportedFilesInDir: %v", err)
	}
	for _, f := range files {
		if strings.Contains(f, "manifest") {
			t.Errorf("supportedFilesInDir returned a cache manifest as input: %v", files)
		}
	}
	found := false
	for _, f := range files {
		if strings.HasSuffix(f, "data.csv") {
			found = true
		}
	}
	if !found {
		t.Errorf("supportedFilesInDir did not return data.csv: %v", files)
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
		if saveErr := s.commands.saveCommand(context.Background(), s, []string{forceArg}); saveErr != nil {
			t.Fatalf(".save --force for Fedwire set: %v", saveErr)
		}
	})
	if !strings.Contains(stderr, "Saved FED set") {
		t.Errorf("stderr = %q, want a 'Saved FED set' confirmation", stderr)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("in-place Fedwire save removed the source: %v", statErr)
	}
}

// TestMaybeSaveInPlace covers the non-interactive --save --force path where a
// row-modifying query writes a CSV table back over its source file.
func TestMaybeSaveInPlace(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "t.csv")
	if err := os.WriteFile(src, []byte("id,name\n1,a\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, cleanup, err := newShell(t, []string{"sqly", "--save", "--force", "--sql", "UPDATE t SET name='b' WHERE id=1", src})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s.isTTY = func() bool { return false }

	_ = captureStderr(t, func() {
		_ = captureStdout(t, func() {
			if runErr := s.Run(context.Background()); runErr != nil {
				t.Fatalf("Run with --save --force: %v", runErr)
			}
		})
	})

	after, readErr := os.ReadFile(src) //nolint:gosec // test path
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(after), "1,b") {
		t.Errorf("in-place --save did not persist the change; source = %q", after)
	}
}
