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

// runScript feeds a batch script to a fresh shell over stdin, which is how a
// non-interactive write-back happens now that .save is the only entry point. The
// returned diagnostics are stderr, where a failing statement's own message goes;
// Run's error only says which statement failed.
func runScript(t *testing.T, script string, inputs ...string) (stdout string, err error) {
	t.Helper()
	out, _, err := runScriptStreams(t, script, inputs...)
	return out, err
}

// runScriptStreams is runScript with stderr as well, for the tests that assert
// on the message a failing helper command printed.
func runScriptStreams(t *testing.T, script string, inputs ...string) (stdout, stderr string, err error) {
	t.Helper()
	shell, cleanup, newErr := newShell(t, append([]string{"sqly"}, inputs...))
	if newErr != nil {
		t.Fatalf("newShell: %v", newErr)
	}
	defer cleanup()
	shell.isTTY = func() bool { return false }
	shell.stdin = strings.NewReader(script)

	backupOut, backupErr := config.Stdout, config.Stderr
	var out, errOut strings.Builder
	config.Stdout, config.Stderr = &out, &errOut
	defer func() { config.Stdout, config.Stderr = backupOut, backupErr }()

	err = shell.Run(context.Background())
	return out.String(), errOut.String(), err
}

func TestWriteBack_SaveToDirIsNonDestructive(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")
	outDir := filepath.Join(dir, "out")

	if _, err := runScript(t, "UPDATE people SET age = '99' WHERE name = 'Alice';\n.save "+outDir+"\n", src); err != nil {
		t.Fatalf(".save DIR: %v", err)
	}

	orig, err := os.ReadFile(src) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(orig), "99") {
		t.Errorf("source file was modified by .save DIR:\n%s", orig)
	}

	saved, err := os.ReadFile(filepath.Join(outDir, "people.csv")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("saved file not written: %v", err)
	}
	if !strings.Contains(string(saved), "99") {
		t.Errorf("saved file missing the update:\n%s", saved)
	}
}

func TestWriteBack_SaveInPlaceTruncates(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "nums.csv", "id\n1\n2\n3\n")

	if _, err := runScript(t, "DELETE FROM nums WHERE id > 1;\n.save --in-place\n", src); err != nil {
		t.Fatalf(".save --in-place: %v", err)
	}

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

// TestSaveRejectsPragmaBeforeRunning verifies that a script ending in .save
// rejects a side-effecting PRAGMA before the first statement runs, so it never
// implies a durable effect or prints a rowset that cannot be written back.
func TestSaveRejectsPragmaBeforeRunning(t *testing.T) {
	for _, query := range []string{
		"PRAGMA user_version=1",
		"PRAGMA incremental_vacuum",
		"PRAGMA journal_mode=OFF",
	} {
		t.Run(query, func(t *testing.T) {
			dir := t.TempDir()
			src := writeCSV(t, dir, "psample.csv", "user_name,identifier\na,1\n")
			outDir := filepath.Join(dir, "out")

			stdout, err := runScript(t, query+";\n.save "+outDir+"\n", src)
			if err == nil {
				t.Fatal("expected a PRAGMA save-incompatibility error, got nil")
			}
			if stdout != "" {
				t.Errorf("stdout should stay empty on rejection, got %q", stdout)
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

	// The statement has to change a row: a save plans only the tables the session
	// changed, so an untouched JSON table would be skipped rather than rejected.
	if _, err := runScript(t, "UPDATE data SET data = '{}';\n.save --in-place\n", jsonPath); err == nil {
		t.Fatal("expected an error saving back to a JSON source, got nil")
	}
}

func TestWriteBack_SaveDirRejectsSourceParent(t *testing.T) {
	// .save DIR pointed at the source's own directory resolves the destination to
	// the source file, which would overwrite it without --in-place. Reject
	// it and leave the source untouched.
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	orig, _ := os.ReadFile(src) //nolint:gosec // test path

	script := "UPDATE user SET first_name='P' WHERE identifier=1" + ";\n.save " + dir + "\n"
	inputs := []string{src}

	if _, runErr := runScript(t, script, inputs...); runErr == nil {
		t.Fatal("expected an error when .save DIR resolves to the source file, got nil")
	}
	after, _ := os.ReadFile(src) //nolint:gosec // test path
	if string(after) != string(orig) {
		t.Errorf("source file was overwritten:\n got %q\nwant %q", after, orig)
	}
}

func TestWriteBack_OutputRejectsSourceAlias(t *testing.T) {
	// --output that aliases an imported source file would destroy the dataset
	// without .save --in-place. Reject it and leave the source untouched.
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

func TestWriteBack_SaveDirRejectsExistingDestination(t *testing.T) {
	// .save DIR must not silently overwrite a pre-existing file in the
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

	script := "UPDATE user SET first_name='Q' WHERE identifier=1" + ";\n.save " + out + "\n"
	inputs := []string{src}

	if _, runErr := runScript(t, script, inputs...); runErr == nil {
		t.Fatal("expected an error when the .save DIR destination already exists, got nil")
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

	// Both tables must change: an untouched xlsx table is skipped, not rejected.
	script := "UPDATE user SET first_name='X' WHERE identifier=1;\n" +
		"UPDATE sample_test_sheet SET name='X' WHERE id=1;\n" +
		".save " + out + "\n"
	inputs := []string{src, xlsx}

	stdout, runErr := runScript(t, script, inputs...)
	if runErr == nil {
		t.Fatal("expected the run to fail because the xlsx source cannot be written back, got nil")
	}
	if strings.Contains(stdout, "affected") {
		t.Errorf("stdout leaked a success count on a failed run: %q", stdout)
	}
}

func TestWriteBack_ReadOnlyQuerySkipsWriteBack(t *testing.T) {
	// A read-only query before .save --in-place must not rewrite the source file.
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	orig, _ := os.ReadFile(src) //nolint:gosec // test path

	script := "SELECT * FROM user WHERE identifier=1" + ";\n.save --in-place\n"
	inputs := []string{src}

	if _, runErr := runScript(t, script, inputs...); runErr != nil {
		t.Fatalf("read-only query before .save --in-place should succeed without writing: %v", runErr)
	}
	after, _ := os.ReadFile(src) //nolint:gosec // test path
	if string(after) != string(orig) {
		t.Errorf("read-only query rewrote the source file:\n got %q\nwant %q", after, orig)
	}
}

func TestWriteBack_SaveDirIsAllOrNothing(t *testing.T) {
	// .save DIR must validate every target before writing any, so one bad target
	// cannot leave partial output behind.
	dir := t.TempDir()
	idSrc := writeCSV(t, dir, "identifier.csv", "identifier\n1\n2\n")
	userSrc := writeCSV(t, dir, "user.csv", "user_name,identifier,first_name,last_name\na,1,A,One\n")
	out := filepath.Join(dir, "out")
	// A directory at out/user.csv makes the user table unwritable.
	if err := os.MkdirAll(filepath.Join(out, "user.csv"), 0o750); err != nil {
		t.Fatal(err)
	}

	// Both tables have to change: a save plans only the tables the session
	// changed, so leaving one untouched would take its unwritable destination out
	// of the plan and there would be nothing to be all-or-nothing about.
	script := "DELETE FROM identifier WHERE identifier = 2;\n" +
		"UPDATE user SET first_name = 'B' WHERE identifier = 1;\n" +
		".save " + out + "\n"
	inputs := []string{idSrc, userSrc}

	_, stderr, runErr := runScriptStreams(t, script, inputs...)
	if runErr == nil {
		t.Fatalf("expected an error when one .save DIR target is unwritable, got nil (stderr=%q)", stderr)
	}
	if _, statErr := os.Stat(filepath.Join(out, "identifier.csv")); statErr == nil {
		t.Error("identifier.csv was written despite the run failing; .save DIR must be all-or-nothing")
	}
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
	_, stderr, runErr := runScriptStreams(t, ".save --in-place\n")
	if runErr == nil {
		t.Fatal("expected an error for .save with no imported tables")
	}
	if !strings.Contains(stderr, "no tables to save") || !strings.Contains(stderr, "input files") {
		t.Errorf("stderr %q should explain the empty run and name the missing input", stderr)
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

		if _, err := (&Shell{}).commitStagedFile(staging, dest); err != nil {
			t.Fatalf("(&Shell{}).commitStagedFile() error = %v", err)
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

		if _, err := (&Shell{}).commitStagedFile(staging, dest); err != nil {
			t.Fatalf("(&Shell{}).commitStagedFile() error = %v", err)
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
		if _, err := (&Shell{}).commitStagedFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dest")); err == nil {
			t.Error("(&Shell{}).commitStagedFile() succeeded with no staged file, want an error")
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

		if err := copyFileContents(staging, dest); err != nil {
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

		if err := copyFileContents(filepath.Join(dir, "missing"), dest); err == nil {
			t.Error("copyFileContents() succeeded with no source, want an error")
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

	_ = (&Shell{}).rollbackCommitted([]stagedWrite{
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
