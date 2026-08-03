package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

func TestWritableExportTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     string
		wantOK     bool
		wantFormat model.ExportFormat
		wantComp   model.Compression
	}{
		{name: "csv is writable", source: "data.csv", wantOK: true, wantFormat: model.ExportCSV, wantComp: model.CompressionNone},
		{name: "tsv is writable", source: "data.tsv", wantOK: true, wantFormat: model.ExportTSV, wantComp: model.CompressionNone},
		{name: "ltsv is writable", source: "data.ltsv", wantOK: true, wantFormat: model.ExportLTSV, wantComp: model.CompressionNone},
		{name: "parquet is writable", source: "data.parquet", wantOK: true, wantFormat: model.ExportParquet, wantComp: model.CompressionNone},
		{name: "csv.gz keeps gzip", source: "data.csv.gz", wantOK: true, wantFormat: model.ExportCSV, wantComp: model.CompressionGzip},
		{name: "tsv.zst keeps zstd", source: "data.tsv.zst", wantOK: true, wantFormat: model.ExportTSV, wantComp: model.CompressionZstd},
		{name: "json is not writable", source: "data.json", wantOK: false},
		{name: "jsonl is not writable", source: "data.jsonl", wantOK: false},
		{name: "xlsx is not writable", source: "data.xlsx", wantOK: false},
		{name: "ach is not writable", source: "data.ach", wantOK: false},
		{name: "fed is not writable", source: "data.fed", wantOK: false},
		{name: "compressed parquet is not writable", source: "data.parquet.gz", wantOK: false},
		{name: "unknown extension is not writable", source: "data.bin", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			format, comp, ok := writableExportTarget(tt.source)
			if ok != tt.wantOK {
				t.Fatalf("writableExportTarget(%q) ok = %v, want %v", tt.source, ok, tt.wantOK)
			}
			if ok && (format != tt.wantFormat || comp != tt.wantComp) {
				t.Errorf("writableExportTarget(%q) = (%v, %v), want (%v, %v)", tt.source, format, comp, tt.wantFormat, tt.wantComp)
			}
		})
	}
}

func TestValidateSaveFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := writeCSV(t, dir, "input.csv", "id\n1\n")

	tests := []struct {
		name          string
		save          bool
		saveTablesDir string
		query         string
		inputs        []string
		tty           bool
		wantErr       bool
	}{
		{name: "no save flags is allowed", wantErr: false},
		{name: "save-in-place with a query is allowed", save: true, query: "SELECT 1", inputs: []string{input}, wantErr: false},
		{name: "save-in-place on an interactive session is rejected", save: true, inputs: []string{input}, tty: true, wantErr: true},
		{name: "save-tables on an interactive session is rejected", saveTablesDir: "out", inputs: []string{input}, tty: true, wantErr: true},
		{name: "save-tables with query is allowed", saveTablesDir: "out", query: "SELECT 1", inputs: []string{input}, wantErr: false},
		{name: "save-tables in batch (non-tty) is allowed", saveTablesDir: "out", inputs: []string{input}, tty: false, wantErr: false},
		{name: "save-in-place with a query but no input is rejected", save: true, query: "SELECT 1", wantErr: true},
		{name: "save-tables with a query but no input is rejected", saveTablesDir: "out", query: "SELECT 1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Shell{
				argument: &config.Arg{SaveInPlace: tt.save, SaveTablesDir: tt.saveTablesDir, Query: tt.query, FilePaths: tt.inputs},
				isTTY:    func() bool { return tt.tty },
			}
			err := s.validateSaveFlags("")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSaveFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteBack_SaveTablesDirIsNonDestructive(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")
	outDir := filepath.Join(dir, "out")

	runWithArgs(t, []string{"sqly", "--sql", "UPDATE people SET age = '99' WHERE name = 'Alice'", "--save-tables", outDir, src})

	orig, err := os.ReadFile(src) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(orig), "99") {
		t.Errorf("source file was modified by --save-tables:\n%s", orig)
	}

	saved, err := os.ReadFile(filepath.Join(outDir, "people.csv")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("saved file not written: %v", err)
	}
	if !strings.Contains(string(saved), "99") {
		t.Errorf("saved file missing the update:\n%s", saved)
	}
}

func TestWriteBack_SaveInPlaceWithForce(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "nums.csv", "id\n1\n2\n3\n")

	runWithArgs(t, []string{"sqly", "--sql", "DELETE FROM nums WHERE id > 1", "--save-in-place", src})

	got, err := os.ReadFile(src) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	// Header plus one remaining row; the deleted rows must be gone (O_TRUNC).
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 {
		t.Errorf("in-place save did not truncate; got %d lines:\n%s", len(lines), got)
	}
}

// TestRunSaveRejectsPragma verifies that a non-interactive --save/--save-tables run
// rejects a side-effecting PRAGMA before execution, so it never implies a durable
// effect or prints a rowset that cannot be written back.
func TestRunSaveRejectsPragma(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"setter PRAGMA with --save-in-place", []string{"sqly", "--sql", "PRAGMA user_version=1", "--save-in-place"}},
		{"command PRAGMA with --save-in-place", []string{"sqly", "--sql", "PRAGMA incremental_vacuum", "--save-in-place"}},
		{"rowset PRAGMA with --save-in-place", []string{"sqly", "--sql", "PRAGMA journal_mode=OFF", "--save-in-place"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeCSV(t, dir, "psample.csv", "user_name,identifier\na,1\n")
			args := append(append([]string{}, tc.args...), src)

			shell, cleanup, err := newShell(t, args)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			shell.isTTY = func() bool { return true }

			backup := config.Stdout
			var buf strings.Builder
			config.Stdout = &buf
			defer func() { config.Stdout = backup }()

			if runErr := shell.Run(context.Background()); runErr == nil {
				t.Fatal("expected a PRAGMA save-incompatibility error, got nil")
			}
			if buf.Len() != 0 {
				t.Errorf("stdout should stay empty on rejection, got %q", buf.String())
			}
		})
	}
}

// TestRunSaveTablesDirRejectsPragma covers the --save-tables variant of the PRAGMA
// save-incompatibility rejection.
func TestRunSaveTablesDirRejectsPragma(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"setter PRAGMA with --save-tables", "PRAGMA user_version=1"},
		{"command PRAGMA with --save-tables", "PRAGMA incremental_vacuum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeCSV(t, dir, "psample.csv", "user_name,identifier\na,1\n")
			outDir := filepath.Join(dir, "out")

			shell, cleanup, err := newShell(t, []string{"sqly", "--sql", tc.query, "--save-tables", outDir, src})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			shell.isTTY = func() bool { return true }

			if runErr := shell.Run(context.Background()); runErr == nil {
				t.Fatal("expected a PRAGMA save-incompatibility error, got nil")
			}
			if _, statErr := os.Stat(outDir); statErr == nil {
				t.Errorf("save directory %s should not be created on rejection", outDir)
			}
		})
	}
}

// TestSaveCommandReadOnlySessionLeavesSourceUntouched verifies that interactive
// .save --in-place after a read-only session does not rewrite the source file (which
// would normalize its bytes), and .save DIR writes no export.
func TestSaveCommandReadOnlySessionLeavesSourceUntouched(t *testing.T) {
	// No trailing newline, so any rewrite that normalizes it is detectable.
	const content = "user_name,identifier\nalice,1"

	setup := func(t *testing.T, name string) (*Shell, func(), string) {
		t.Helper()
		dir := t.TempDir()
		src := filepath.Join(dir, name)
		if err := os.WriteFile(src, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
			cleanup()
			t.Fatal(err)
		}
		// A read-only query must not mark the session as changed.
		if _, err := getExecStdOutput(t, shell.exec, "SELECT * FROM "+strings.TrimSuffix(name, ".csv")); err != nil {
			cleanup()
			t.Fatal(err)
		}
		return shell, cleanup, src
	}

	t.Run(".save --in-place does not rewrite an unchanged source", func(t *testing.T) {
		shell, cleanup, src := setup(t, "readonly.csv")
		defer cleanup()

		backup := config.Stderr
		config.Stderr = &strings.Builder{}
		defer func() { config.Stderr = backup }()

		if err := shell.commands.saveCommand(context.Background(), shell, []string{inPlaceArg}); err != nil {
			t.Fatalf(".save --in-place returned error: %v", err)
		}
		after, _ := os.ReadFile(src) //nolint:gosec // test path
		if string(after) != content {
			t.Errorf("read-only .save --in-place rewrote the source:\n got %q\nwant %q", after, content)
		}
	})

	t.Run(".save DIR writes no export for an unchanged session", func(t *testing.T) {
		shell, cleanup, src := setup(t, "readonly2.csv")
		defer cleanup()
		outDir := filepath.Join(filepath.Dir(src), "out")

		backup := config.Stderr
		config.Stderr = &strings.Builder{}
		defer func() { config.Stderr = backup }()

		if err := shell.commands.saveCommand(context.Background(), shell, []string{outDir}); err != nil {
			t.Fatalf(".save DIR returned error: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(outDir, "readonly2.csv")); statErr == nil {
			t.Error("read-only .save DIR wrote an export when no data changed")
		}
	})
}

// TestSaveCommandRejectsFlagLikeDestination checks that a flag the command does
// not have is reported as such rather than created as a directory. `.save
// --force` was the spelling before `.save --in-place`, so someone typing it from
// memory would otherwise get a directory literally named "--force" and a success
// exit code, with the sources they asked to overwrite left alone.
func TestSaveCommandRejectsFlagLikeDestination(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "flagged.csv", "user_name,identifier\nalice,1\n")

	shell, cleanup, err := newShell(t, []string{"sqly", src})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The guard must fire regardless of session state, so mark data as changed:
	// an unchanged session short-circuits before any destination is resolved.
	shell.dataChanged = true

	for _, arg := range []string{"--force", "-f", "--save-in-place"} {
		t.Run(arg, func(t *testing.T) {
			err := shell.commands.saveCommand(context.Background(), shell, []string{arg})
			if err == nil {
				t.Fatalf(".save %s returned nil error, want a rejection", arg)
			}
			if !strings.Contains(err.Error(), inPlaceArg) {
				t.Errorf("error should point at %s, got: %v", inPlaceArg, err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, arg)); statErr == nil {
				t.Errorf(".save %s created a directory named after the flag", arg)
			}
		})
	}
}

// TestSaveCommandPersistsAfterDataChange guards that the read-only no-op does not
// also suppress a legitimate save after the session modified table data.
func TestSaveCommandPersistsAfterDataChange(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "changed.csv")
	if err := os.WriteFile(src, []byte("user_name,identifier\nalice,1"), 0o600); err != nil {
		t.Fatal(err)
	}
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
		t.Fatal(err)
	}
	if _, err := getExecStdOutput(t, shell.exec, "UPDATE changed SET identifier=2 WHERE user_name='alice'"); err != nil {
		t.Fatal(err)
	}

	backup := config.Stderr
	config.Stderr = &strings.Builder{}
	defer func() { config.Stderr = backup }()

	if err := shell.commands.saveCommand(context.Background(), shell, []string{inPlaceArg}); err != nil {
		t.Fatalf(".save --in-place after a change returned error: %v", err)
	}
	after, _ := os.ReadFile(src) //nolint:gosec // test path
	if !strings.Contains(string(after), "alice,2") {
		t.Errorf(".save --in-place did not persist the change; got %q", after)
	}
}

// TestSaveCommandSkipsWhenNoImportedTableChanged covers the cases where the
// session reports a data change but no file-backed imported table actually
// differs: only a TEMP or SQL-created scratch table changed, or edits to an
// imported table cancel out (net-zero). Write-back must touch no source file and
// must not fail on an unwritable or non-file-backed table it should ignore.
func TestSaveCommandSkipsWhenNoImportedTableChanged(t *testing.T) {
	const content = "user_name,identifier\nalice,1"

	setup := func(t *testing.T, name string, stmts ...string) (*Shell, func(), string) {
		t.Helper()
		dir := t.TempDir()
		src := filepath.Join(dir, name)
		if err := os.WriteFile(src, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
			cleanup()
			t.Fatal(err)
		}
		for _, stmt := range stmts {
			if err := shell.exec(context.Background(), stmt); err != nil {
				cleanup()
				t.Fatalf("exec %q: %v", stmt, err)
			}
		}
		return shell, cleanup, src
	}

	cases := []struct {
		name  string
		file  string
		stmts []string
	}{
		{
			name: "only a TEMP table changed",
			file: "temp_only.csv",
			stmts: []string{
				"CREATE TEMP TABLE scratch(id INTEGER)",
				"INSERT INTO scratch VALUES (1)",
			},
		},
		{
			name: "only a SQL-created scratch table changed",
			file: "scratch_only.csv",
			stmts: []string{
				"CREATE TABLE scratch(id INTEGER)",
				"INSERT INTO scratch VALUES (1)",
			},
		},
		{
			name: "net-zero edits cancel out",
			file: "netzero.csv",
			stmts: []string{
				"UPDATE netzero SET identifier=99 WHERE user_name='alice'",
				"UPDATE netzero SET identifier=1 WHERE user_name='alice'",
			},
		},
	}

	for _, tc := range cases {
		t.Run(".save --in-place leaves the source untouched when "+tc.name, func(t *testing.T) {
			shell, cleanup, src := setup(t, tc.file, tc.stmts...)
			defer cleanup()

			backup := config.Stderr
			config.Stderr = &strings.Builder{}
			defer func() { config.Stderr = backup }()

			if err := shell.commands.saveCommand(context.Background(), shell, []string{inPlaceArg}); err != nil {
				t.Fatalf(".save --in-place returned error: %v", err)
			}
			after, _ := os.ReadFile(src) //nolint:gosec // test path
			if string(after) != content {
				t.Errorf(".save --in-place rewrote an unchanged source:\n got %q\nwant %q", after, content)
			}
		})

		t.Run(".save DIR writes no export when "+tc.name, func(t *testing.T) {
			shell, cleanup, src := setup(t, tc.file, tc.stmts...)
			defer cleanup()
			outDir := filepath.Join(filepath.Dir(src), "out")

			backup := config.Stderr
			config.Stderr = &strings.Builder{}
			defer func() { config.Stderr = backup }()

			if err := shell.commands.saveCommand(context.Background(), shell, []string{outDir}); err != nil {
				t.Fatalf(".save DIR returned error: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(outDir, tc.file)); statErr == nil {
				t.Error(".save DIR wrote an export when no imported table changed")
			}
		})
	}
}

// TestSaveCommandSkipsUnwritableImportWhenUnchanged covers a JSONL import (which
// write-back cannot persist) left untouched while only a scratch table changed:
// the unchanged unwritable import must be ignored, not reported as unwritable.
func TestSaveCommandSkipsUnwritableImportWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.jsonl")
	const content = "{\"id\":1}\n{\"id\":2}\n"
	if err := os.WriteFile(src, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
		t.Fatal(err)
	}
	if err := shell.exec(context.Background(), "CREATE TEMP TABLE scratch(id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if err := shell.exec(context.Background(), "INSERT INTO scratch VALUES (1)"); err != nil {
		t.Fatal(err)
	}

	backup := config.Stderr
	config.Stderr = &strings.Builder{}
	defer func() { config.Stderr = backup }()

	if err := shell.commands.saveCommand(context.Background(), shell, []string{inPlaceArg}); err != nil {
		t.Fatalf(".save --in-place reported an error for an unchanged JSONL import: %v", err)
	}
	after, _ := os.ReadFile(src) //nolint:gosec // test path
	if string(after) != content {
		t.Errorf(".save --in-place rewrote the unchanged JSONL source: got %q", after)
	}
}

func TestWriteBack_UnsupportedSourceErrors(t *testing.T) {
	dir := t.TempDir()
	// JSON loads into a single data column and does not round-trip, so write-back
	// must refuse it rather than corrupt the file.
	jsonPath := filepath.Join(dir, "data.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"a":1}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	// A modifying statement triggers write-back (a read-only query would skip it),
	// so the unsupported-source rejection is exercised.
	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "DELETE FROM data WHERE 1=0", "--save-in-place", jsonPath})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if err := shell.Run(context.Background()); err == nil {
		t.Fatal("expected an error saving back to a JSON source, got nil")
	}
}

func TestWriteBack_SaveTablesDirRejectsSourceParent(t *testing.T) {
	// --save-tables pointed at the source's own directory resolves the destination to
	// the source file, which would overwrite it without --save-in-place. Reject
	// it and leave the source untouched.
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	orig, _ := os.ReadFile(src) //nolint:gosec // test path

	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "UPDATE user SET first_name='P' WHERE identifier=1", "--save-tables", dir, src})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	if runErr := shell.Run(context.Background()); runErr == nil {
		t.Fatal("expected an error when --save-tables resolves to the source file, got nil")
	}
	after, _ := os.ReadFile(src) //nolint:gosec // test path
	if string(after) != string(orig) {
		t.Errorf("source file was overwritten:\n got %q\nwant %q", after, orig)
	}
}

func TestWriteBack_OutputRejectsSourceAlias(t *testing.T) {
	// --output that aliases an imported source file would destroy the dataset
	// without --save-in-place. Reject it and leave the source untouched.
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	orig, _ := os.ReadFile(src) //nolint:gosec // test path

	shell, cleanup, err := newShell(t, []string{"sqly", "--output-format", "csv", "--sql", "SELECT * FROM user WHERE identifier=1", "--output", src, src})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	runErr := shell.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected an error when --output aliases the source file, got nil")
	}
	if !strings.Contains(runErr.Error(), "--output") {
		t.Errorf("error = %q, want it to mention --output", runErr)
	}
	after, _ := os.ReadFile(src) //nolint:gosec // test path
	if string(after) != string(orig) {
		t.Errorf("source file was overwritten by --output:\n got %q\nwant %q", after, orig)
	}
}

func TestWriteBack_SaveTablesDirRejectsExistingDestination(t *testing.T) {
	// --save-tables must not silently overwrite a pre-existing file in the
	// destination directory.
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o750); err != nil {
		t.Fatal(err)
	}
	sentinel := "PRE-EXISTING\n"
	dest := filepath.Join(out, "user.csv")
	if err := os.WriteFile(dest, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}

	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "UPDATE user SET first_name='Q' WHERE identifier=1", "--save-tables", out, src})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	if runErr := shell.Run(context.Background()); runErr == nil {
		t.Fatal("expected an error when --save-tables destination already exists, got nil")
	}
	after, _ := os.ReadFile(dest) //nolint:gosec // test path
	if string(after) != sentinel {
		t.Errorf("pre-existing destination was overwritten:\n got %q\nwant %q", after, sentinel)
	}
}

func TestWriteBack_FailedWriteBackKeepsStdoutClean(t *testing.T) {
	// When a run ultimately fails during write-back, stdout must stay free of the
	// DML success count so scripts do not treat it as partially successful.
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	xlsx := filepath.Join(dir, "sample.xlsx")
	copyTestFile(t, "sample.xlsx", xlsx)
	out := filepath.Join(dir, "out")

	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "UPDATE user SET first_name='X' WHERE identifier=1", "--save-tables", out, src, xlsx})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	stdout, runErr := getStdoutForErr(t, shell.Run)
	if runErr == nil {
		t.Fatal("expected the run to fail because the xlsx source cannot be written back, got nil")
	}
	if strings.Contains(string(stdout), "affected") {
		t.Errorf("stdout leaked a success count on a failed run: %q", stdout)
	}
}

func TestWriteBack_ReadOnlyQuerySkipsWriteBack(t *testing.T) {
	// A read-only query under --save-in-place must not rewrite the source file.
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	orig, _ := os.ReadFile(src) //nolint:gosec // test path

	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "SELECT * FROM user WHERE identifier=1", "--save-in-place", src})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	if runErr := shell.Run(context.Background()); runErr != nil {
		t.Fatalf("read-only query with --save-in-place should succeed without writing: %v", runErr)
	}
	after, _ := os.ReadFile(src) //nolint:gosec // test path
	if string(after) != string(orig) {
		t.Errorf("read-only query rewrote the source file:\n got %q\nwant %q", after, orig)
	}
}

func TestWriteBack_SaveTablesDirIsAllOrNothing(t *testing.T) {
	// --save-tables must validate every target before writing any, so one bad target
	// cannot leave partial output behind.
	dir := t.TempDir()
	idSrc := writeCSV(t, dir, "identifier.csv", "identifier\n1\n2\n")
	userSrc := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	out := filepath.Join(dir, "out")
	// A directory at out/user.csv makes the user table unwritable.
	if err := os.MkdirAll(filepath.Join(out, "user.csv"), 0o750); err != nil {
		t.Fatal(err)
	}

	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "DELETE FROM identifier WHERE 1=0", "--save-tables", out, idSrc, userSrc})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	if runErr := shell.Run(context.Background()); runErr == nil {
		t.Fatal("expected an error when one --save-tables target is unwritable, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(out, "identifier.csv")); statErr == nil {
		t.Error("identifier.csv was written despite the run failing; --save-tables must be all-or-nothing")
	}
}

// runWithArgs builds a shell from args and runs it, failing the test on error.
func runWithArgs(t *testing.T, args []string) {
	t.Helper()
	shell, cleanup, err := newShell(t, args)
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()
	_ = captureStdout(t, func() {
		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

func TestNoTablesToSaveError(t *testing.T) {
	t.Parallel()

	t.Run("interactive empty session is told to .import a file", func(t *testing.T) {
		t.Parallel()
		got := noTablesToSaveError(true).Error()
		if !strings.Contains(got, ".import") {
			t.Errorf("interactive empty-save error %q should suggest .import", got)
		}
	})

	t.Run("non-interactive empty session is told to pass input files", func(t *testing.T) {
		t.Parallel()
		got := noTablesToSaveError(false).Error()
		if !strings.Contains(got, "input files") {
			t.Errorf("non-interactive empty-save error %q should suggest passing input files", got)
		}
	})
}

func TestSaveCommand_EmptyInteractiveSessionGuidesToImport(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s.isTTY = func() bool { return true }

	err = s.commands.saveCommand(context.Background(), s, []string{"--in-place"})
	if err == nil {
		t.Fatal("expected an error when saving an empty interactive session")
	}
	if !strings.Contains(err.Error(), "no tables to save") || !strings.Contains(err.Error(), ".import") {
		t.Errorf("error %q should explain the empty session and suggest .import", err.Error())
	}
}

func TestSave_EmptyNonInteractiveRunGuidesToInputFiles(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly", "--save-in-place", "--sql", "UPDATE foo SET x=1"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s.isTTY = func() bool { return false }

	// Preflight rejects the save before any query output, so Run can be called
	// directly without capturing stdout.
	runErr := s.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected an error for --save with no input files")
	}
	if !strings.Contains(runErr.Error(), "no input file or directory was given") {
		t.Errorf("error %q should explain the empty run and name the missing input", runErr.Error())
	}
}

// TestCommitStagedFile covers the commit half of the staged write-back,
// including the copy taken when the platform refuses the rename. Windows does
// refuse it when another handle still has the destination open, which is every
// in-place save, so the fallback is not a rare path there.
func TestCommitStagedFile(t *testing.T) {
	t.Parallel()

	t.Run("replaces an existing destination", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		staging := filepath.Join(dir, "staging")
		dest := filepath.Join(dir, "dest")
		if err := os.WriteFile(staging, []byte("new"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte("old content that is longer"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := commitStagedFile(staging, dest); err != nil {
			t.Fatalf("commitStagedFile() error = %v", err)
		}
		got, err := os.ReadFile(dest) //nolint:gosec // Test path from t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new" {
			t.Errorf("destination = %q, want %q", got, "new")
		}
	})

	t.Run("creates a destination that does not exist", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		staging := filepath.Join(dir, "staging")
		dest := filepath.Join(dir, "dest")
		if err := os.WriteFile(staging, []byte("new"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := commitStagedFile(staging, dest); err != nil {
			t.Fatalf("commitStagedFile() error = %v", err)
		}
		got, err := os.ReadFile(dest) //nolint:gosec // Test path from t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new" {
			t.Errorf("destination = %q, want %q", got, "new")
		}
	})

	t.Run("reports a staged file that is gone", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if err := commitStagedFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dest")); err == nil {
			t.Error("commitStagedFile() succeeded with no staged file, want an error")
		}
	})

	// The copy fallback is what runs on Windows whenever the destination is open.
	// It is driven directly because a plain rename succeeds on Unix, so
	// commitStagedFile never reaches it on this platform.
	t.Run("the copy path replaces the destination", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		staging := filepath.Join(dir, "staging")
		dest := filepath.Join(dir, "dest")
		if err := os.WriteFile(staging, []byte("new"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte("old content that is longer"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := copyOnto(staging, dest); err != nil {
			t.Fatalf("commitByCopy() error = %v", err)
		}
		got, err := os.ReadFile(dest) //nolint:gosec // Test path from t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new" {
			t.Errorf("destination = %q, want %q", got, "new")
		}
		assertNoBackupLeft(t, dir)
	})

	t.Run("the copy path reports a source that is gone", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		dest := filepath.Join(dir, "dest")
		if err := os.WriteFile(dest, []byte("precious"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := copyOnto(filepath.Join(dir, "missing"), dest); err == nil {
			t.Error("copyOnto() succeeded with no source, want an error")
		}
		got, err := os.ReadFile(dest) //nolint:gosec // Test path from t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "precious" {
			t.Errorf("destination = %q, want it untouched when the source cannot be opened", got)
		}
	})
}

// assertNoBackupLeft fails when the commit's own backup file survived the call.
func assertNoBackupLeft(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "sqly-bak") {
			t.Errorf("the backup must not be left behind: %s", e.Name())
		}
	}
}

// TestRollbackCommitted covers the undo the commit phase runs when a later
// target fails to land: every destination already replaced goes back to what it
// held, and one this save created is removed again.
func TestRollbackCommitted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Two destinations that existed and were replaced, and one this save created.
	replaced := filepath.Join(dir, "replaced.csv")
	if err := os.WriteFile(replaced, []byte("committed"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "replaced.bak")
	if err := os.WriteFile(backup, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(dir, "created.csv")
	if err := os.WriteFile(created, []byte("committed"), 0o600); err != nil {
		t.Fatal(err)
	}

	rollbackCommitted([]stagedWrite{
		{target: writeTarget{dest: replaced}, backup: backup},
		{target: writeTarget{dest: created}},
	})

	got, err := os.ReadFile(replaced) //nolint:gosec // Test path from t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Errorf("replaced destination = %q, want the original %q back", got, "original")
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Errorf("a destination this save created must be removed again, stat err = %v", err)
	}
}
