package shell

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

func newBoundaryTestShell(t *testing.T, usecases Usecases) *Shell {
	t.Helper()

	arg := &config.Arg{
		Output: &config.Output{
			Mode: model.PrintModeTable,
		},
	}
	state, err := newState(arg)
	if err != nil {
		t.Fatalf("newState: %v", err)
	}
	return &Shell{
		argument: arg,
		commands: NewCommands(),
		state:    state,
		usecases: usecases,
	}
}

func TestExec_RuntimeHistoryFailureDisablesHistoryAndContinues(t *testing.T) {
	ctrl := gomock.NewController(t)
	history := mock.NewMockHistoryUsecase(ctrl)
	query := mock.NewMockQueryUsecase(ctrl)
	query.EXPECT().Dialect().Return(dialect.SQLite).AnyTimes()

	table := model.NewTable("t", model.Header{"n"}, []model.Record{
		model.Record([]string{"1"}),
	})

	// First command: the history write fails as if the DB became read-only after
	// startup. The write is a single insert (no preceding List scan), and the
	// query must still run.
	history.EXPECT().Append(gomock.Any(), gomock.Any()).
		Return(errors.New("attempt to write a readonly database"))
	query.EXPECT().ExecSQL(gomock.Any(), "SELECT 1").Return(table, int64(0), nil)

	s := newBoundaryTestShell(t, Usecases{history: history, query: query})
	s.historyEnabled = true

	_ = captureStdout(t, func() {
		if err := s.exec(context.Background(), "SELECT 1"); err != nil {
			t.Fatalf("exec aborted on a best-effort history failure: %v", err)
		}
	})

	if s.historyEnabled {
		t.Error("historyEnabled should be false after a runtime history failure")
	}

	// Second command: history must not be touched again. No further history
	// expectations are set, so gomock fails the test if List or Create is called.
	query.EXPECT().ExecSQL(gomock.Any(), "SELECT 2").Return(table, int64(0), nil)
	_ = captureStdout(t, func() {
		if err := s.exec(context.Background(), "SELECT 2"); err != nil {
			t.Fatalf("second exec errored after history was disabled: %v", err)
		}
	})
}

func TestExec_HistoryWriteDoesNotScanHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	history := mock.NewMockHistoryUsecase(ctrl)
	query := mock.NewMockQueryUsecase(ctrl)
	query.EXPECT().Dialect().Return(dialect.SQLite).AnyTimes()

	table := model.NewTable("t", model.Header{"n"}, []model.Record{
		model.Record([]string{"1"}),
	})

	// A history write is a single insert: it must not call List to compute an id.
	// No List expectation is set, so gomock fails the test if List is called.
	history.EXPECT().Append(gomock.Any(), gomock.Any()).Return(nil)
	query.EXPECT().ExecSQL(gomock.Any(), "SELECT 1").Return(table, int64(0), nil)

	s := newBoundaryTestShell(t, Usecases{history: history, query: query})
	s.historyEnabled = true

	_ = captureStdout(t, func() {
		if err := s.exec(context.Background(), "SELECT 1"); err != nil {
			t.Fatalf("exec returned error: %v", err)
		}
	})

	if !s.historyEnabled {
		t.Error("historyEnabled should remain true after a successful history write")
	}
}

func TestExec_SQLReachesTheEngineWithQuotesInItsComments(t *testing.T) {
	// Shell quoting rules belong to the dot-commands. A SQL statement is not a
	// command line: an apostrophe inside a comment is text, and refusing the
	// statement for it means valid SQL never reaches the engine.
	tests := []struct {
		name  string
		input string
	}{
		{"apostrophe in a line comment", "SELECT 1 AS x -- don't panic"},
		{"apostrophe in a block comment", "SELECT 1 AS x /* it's fine */"},
		{"double quote in a line comment", `SELECT 1 AS x -- say "hi`},
		{"apostrophe in a block comment spanning lines", "SELECT 1 AS x\n/* it's\n   fine */"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := mock.NewMockQueryUsecase(ctrl)
			query.EXPECT().Dialect().Return(dialect.SQLite).AnyTimes()
			table := model.NewTable("t", model.Header{"x"}, []model.Record{
				model.Record([]string{"1"}),
			})
			query.EXPECT().ExecSQL(gomock.Any(), tt.input).Return(table, int64(0), nil)

			s := newBoundaryTestShell(t, Usecases{query: query})
			_ = captureStdout(t, func() {
				if err := s.exec(context.Background(), tt.input); err != nil {
					t.Fatalf("exec(%q) = %v, want the statement to run", tt.input, err)
				}
			})
		})
	}
}

func TestExec_DotCommandQuotingIsStillParsed(t *testing.T) {
	// The other half of the same rule: a line that starts with "." is a command
	// line, so its quoting is parsed and an unterminated quote is an error.
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"unterminated single quote", ".import 'oops", "unterminated single quote in command"},
		{"unterminated double quote", `.import "oops`, "unterminated double quote in command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newBoundaryTestShell(t, Usecases{})
			err := s.exec(context.Background(), tt.input)
			if err == nil {
				t.Fatalf("exec(%q) = nil, want %q", tt.input, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("exec(%q) = %v, want it to mention %q", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestExec_RecordsHistoryForAnInputItCannotParse(t *testing.T) {
	// History is what the up-arrow edits. A line rejected before it ran is the
	// one most worth recalling, so the record happens before the parse rather
	// than after it.
	ctrl := gomock.NewController(t)
	history := mock.NewMockHistoryUsecase(ctrl)
	const input = ".import 'oops"
	history.EXPECT().Append(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, h model.History) error {
			if h.Request != input {
				t.Errorf("history recorded %q, want %q", h.Request, input)
			}
			return nil
		})

	s := newBoundaryTestShell(t, Usecases{history: history})
	s.historyEnabled = true
	if err := s.exec(context.Background(), input); err == nil {
		t.Fatal("exec accepted an unterminated quote")
	}
}

func TestCommandList_cdCommand(t *testing.T) {
	// Note: Cannot use t.Parallel() because of t.Chdir() and t.Setenv() usage
	tests := []struct {
		name      string
		argv      []string
		wantError bool
		setup     func() (string, func()) // Returns initial dir and cleanup function
	}{
		{
			name: "change to specified directory",
			argv: []string{"."},
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				return cwd, func() {
					t.Chdir(cwd)
				}
			},
		},
		{
			name: "change to home directory when no args",
			argv: []string{},
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				originalHome := os.Getenv("HOME")
				// Set a test HOME directory (use existing directory to avoid Windows cleanup issues)
				t.Setenv("HOME", cwd)
				return cwd, func() {
					t.Chdir(cwd)
					t.Setenv("HOME", originalHome)
				}
			},
		},
		{
			name:      "error with too many arguments",
			argv:      []string{"dir1", "dir2"},
			wantError: true,
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				return cwd, func() {
					t.Chdir(cwd)
				}
			},
		},
		{
			name:      "error with non-existent directory",
			argv:      []string{"/nonexistent/directory"},
			wantError: true,
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				return cwd, func() {
					t.Chdir(cwd)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure we return to the original directory after each test
			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Could not get working directory: %v", err)
			}
			defer func() {
				t.Chdir(originalWd)
			}()

			// Setup
			originalDir, cleanup := tt.setup()
			defer cleanup()

			// Create shell instance
			arg := &config.Arg{
				Output: &config.Output{
					Mode: model.PrintModeTable,
				},
			}
			state, err := newState(arg)
			if err != nil {
				t.Fatalf("Failed to create state: %v", err)
			}
			shell := &Shell{
				argument: arg,
				config:   &config.Config{},
				state:    state,
			}
			shell.state.cwd = originalDir

			// Create command list
			commandList := CommandList{}

			// Execute command
			err = commandList.cdCommand(context.Background(), shell, tt.argv)

			// Check result
			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCommandList_lsCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		argv      []string
		wantError bool
	}{
		{
			name: "list current directory",
			argv: []string{},
		},
		{
			name: "list specified directory",
			argv: []string{"."},
		},
		{
			name:      "error with too many arguments",
			argv:      []string{"dir1", "dir2"},
			wantError: true,
		},
		{
			name:      "error with non-existent directory",
			argv:      []string{"/nonexistent/directory"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create shell instance
			arg := &config.Arg{
				Output: &config.Output{
					Mode: model.PrintModeTable,
				},
			}
			state, err := newState(arg)
			if err != nil {
				t.Fatalf("Failed to create state: %v", err)
			}
			shell := &Shell{
				argument: arg,
				config:   &config.Config{},
				state:    state,
			}

			// Create command list
			commandList := CommandList{}

			// Execute command
			err = commandList.lsCommand(context.Background(), shell, tt.argv)

			// Check result
			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCommandList_lsCommand_Output(t *testing.T) {
	// Regression for: .ls must list directory contents in-process with
	// deterministic, OS-independent output instead of shelling out to ls/dir.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.csv"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.csv"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}

	commandList := NewCommands()

	backup := config.Stdout
	defer func() { config.Stdout = backup }()
	var buf bytes.Buffer
	config.Stdout = &buf

	if err := commandList.lsCommand(context.Background(), &Shell{}, []string{dir}); err != nil {
		t.Fatalf("lsCommand returned error: %v", err)
	}

	got := buf.String()
	want := "a.csv\nb.csv\nsub/\n"
	if got != want {
		t.Fatalf("lsCommand output = %q, want %q", got, want)
	}
}

func TestCommandList_cdCommand_StoresAbsolutePath(t *testing.T) {
	// Regression for: .cd must store a normalized absolute path so the
	// prompt stays correct after a relative move.
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	shell := newBoundaryTestShell(t, Usecases{})
	commandList := NewCommands()

	if err := commandList.cdCommand(context.Background(), shell, []string{"sub"}); err != nil {
		t.Fatalf("cdCommand returned error: %v", err)
	}

	if !filepath.IsAbs(shell.state.cwd) {
		t.Fatalf("state.cwd = %q, want an absolute path", shell.state.cwd)
	}
	wantResolved, err := filepath.EvalSymlinks(sub)
	if err != nil {
		t.Fatal(err)
	}
	gotResolved, err := filepath.EvalSymlinks(shell.state.cwd)
	if err != nil {
		t.Fatal(err)
	}
	if gotResolved != wantResolved {
		t.Fatalf("state.cwd resolved = %q, want %q", gotResolved, wantResolved)
	}
}

func TestCommandList_cdCommand_ExpandsTilde(t *testing.T) {
	// Regression for: .cd ~ must expand to the home directory instead of
	// passing a literal "~" to os.Chdir.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home directory: %v", err)
	}

	// cdCommand calls os.Chdir directly, mutating the process working directory.
	// Anchor to a temp dir so t.Chdir restores the original cwd after the test,
	// keeping other tests' relative testdata/ paths valid.
	t.Chdir(t.TempDir())

	shell := newBoundaryTestShell(t, Usecases{})
	commandList := NewCommands()

	if err := commandList.cdCommand(context.Background(), shell, []string{"~"}); err != nil {
		t.Fatalf("cdCommand(~) returned error: %v", err)
	}

	wantResolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	gotResolved, err := filepath.EvalSymlinks(shell.state.cwd)
	if err != nil {
		t.Fatal(err)
	}
	if gotResolved != wantResolved {
		t.Fatalf("state.cwd resolved = %q, want home %q", gotResolved, wantResolved)
	}
}

func TestShell_getFilePathCompletions_errorHandling(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Test with a directory that should cause a permission error or similar
	// This is tricky to test reliably across platforms, so we test the fallback behavior
	// Create a temporary directory and use t.Chdir
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	// Create some test files
	for _, filename := range []string{"test.csv", "test.tsv", "readme.txt"} {
		func() {
			file, err := os.Create(filepath.Clean(filename))
			if err != nil {
				t.Fatalf("Could not create test file %s: %v", filename, err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Errorf("Could not close test file %s: %v", filename, err)
				}
			}()
		}()
	}

	// Test completion - should work normally. An empty prefix scopes to the
	// current directory.
	completions := shell.getFilePathCompletions("")

	if len(completions) == 0 {
		t.Error("Expected some completions in test directory")
	}

	// Verify only valid files are returned
	hasCSV := false
	hasTSV := false
	hasInvalidFile := false

	for _, comp := range completions {
		if comp.Text == "test.csv" {
			hasCSV = true
		}
		if comp.Text == "test.tsv" {
			hasTSV = true
		}
		if comp.Text == "readme.txt" {
			hasInvalidFile = true
		}
		if comp.Description != "Importable file" {
			t.Errorf("Expected description 'Importable file', got '%s'", comp.Description)
		}
	}

	if !hasCSV {
		t.Error("Expected to find test.csv in completions")
	}
	if !hasTSV {
		t.Error("Expected to find test.tsv in completions")
	}
	if hasInvalidFile {
		t.Error("Should not find readme.txt in completions")
	}
}

func TestShell_getFilePathCompletions_edgeCases(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Test with different directory structures
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	// Create nested directory structure
	nestedDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(nestedDir, 0o750)
	if err != nil {
		t.Fatalf("Could not create nested directory: %v", err)
	}

	// Create files in nested directory
	nestedFile := filepath.Join(nestedDir, "nested.csv")
	func() {
		file, err := os.Create(filepath.Clean(nestedFile))
		if err != nil {
			t.Fatalf("Could not create nested file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("Could not close nested file: %v", err)
			}
		}()
	}()

	// Create hidden directory (should be skipped)
	hiddenDir := filepath.Join(tempDir, ".hidden")
	err = os.MkdirAll(hiddenDir, 0o750)
	if err != nil {
		t.Fatalf("Could not create hidden directory: %v", err)
	}

	hiddenFile := filepath.Join(hiddenDir, "hidden.csv")
	func() {
		file, err := os.Create(filepath.Clean(hiddenFile))
		if err != nil {
			t.Fatalf("Could not create hidden file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("Could not close hidden file: %v", err)
			}
		}()
	}()

	// An empty prefix scopes to the current directory: the subdirectory is
	// offered (so the user can descend) while the hidden directory is skipped.
	topLevel := shell.getFilePathCompletions("")
	foundSubdir := false
	foundHiddenDir := false
	for _, comp := range topLevel {
		switch filepath.ToSlash(comp.Text) {
		case "subdir/":
			foundSubdir = true
		case ".hidden/":
			foundHiddenDir = true
		}
	}
	if !foundSubdir {
		t.Error("Expected to find subdir/ at the top level")
	}
	if foundHiddenDir {
		t.Error("Should not find .hidden/ directory at the top level")
	}

	// Descending into the subdirectory lists its importable files.
	nested := shell.getFilePathCompletions("subdir/")
	foundNested := false
	for _, comp := range nested {
		if filepath.ToSlash(comp.Text) == "subdir/nested.csv" {
			foundNested = true
		}
	}
	if !foundNested {
		t.Error("Expected to find nested.csv when scoping to subdir/")
	}
}

// captureStdout runs f with config.Stdout redirected to a buffer and returns
// what was written. It lets command tests assert on user-facing output without
// a full shell.
//
// Tests using this helper must not run in parallel because config.Stdout is a
// package-global writer shared across shell tests.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	backup := config.Stdout
	defer func() { config.Stdout = backup }()
	var buf bytes.Buffer
	config.Stdout = &buf
	f()
	return buf.String()
}

// captureStderr captures what f writes to config.Stderr. File-output status
// messages (--output, .dump, .save) go to stderr to keep stdout data-only.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	backup := config.Stderr
	defer func() { config.Stderr = backup }()
	var buf bytes.Buffer
	config.Stderr = &buf
	f()
	return buf.String()
}

// TestCommandList_tablesCommand_dependsOnMetadataUsecase verifies that .tables
// is satisfied by a MetadataUsecase mock alone: given two schema objects, it lists
// both. .tables enumerates every queryable object via SchemaObjects (tables and
// views, including TEMP), not only the file-imported base tables.
func TestCommandList_tablesCommand_dependsOnMetadataUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().SchemaObjects(gomock.Any()).Return([]*model.Table{
		model.NewTable("users", nil, nil),
		model.NewTable("orders", nil, nil),
	}, nil)

	st, err := newState(&config.Arg{Output: &config.Output{Mode: model.PrintModeTable}})
	if err != nil {
		t.Fatal(err)
	}
	s := &Shell{usecases: Usecases{metadata: metadata}, state: st}
	out := captureStdout(t, func() {
		if err := NewCommands().tablesCommand(context.Background(), s, nil); err != nil {
			t.Fatalf("tablesCommand returned error: %v", err)
		}
	})

	for _, want := range []string{"users", "orders"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain table name %q", out, want)
		}
	}
}

// TestShell_execSQL_dependsOnQueryUsecase verifies that execSQL routes through
// the QueryUsecase boundary: a statement that affects rows reports the count,
// and an execution error is propagated unchanged.
func TestShell_execSQL_dependsOnQueryUsecase(t *testing.T) {
	t.Run("DELETE prints affected row count", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Dialect().Return(dialect.SQLite).AnyTimes()
		query.EXPECT().ExecSQL(gomock.Any(), "DELETE FROM users").Return(nil, int64(3), nil)

		s := &Shell{usecases: Usecases{query: query}}
		out := captureStdout(t, func() {
			if err := s.execSQL(context.Background(), "DELETE FROM users;"); err != nil {
				t.Fatalf("execSQL returned error: %v", err)
			}
		})
		if !strings.Contains(out, "affected is 3 row(s)") {
			t.Errorf("output %q does not report affected rows", out)
		}
	})

	t.Run("query error is propagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		wantErr := errors.New("boom")
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Dialect().Return(dialect.SQLite).AnyTimes()
		query.EXPECT().ExecSQL(gomock.Any(), "SELECT 1").Return(nil, int64(0), wantErr)

		s := &Shell{usecases: Usecases{query: query}}
		if err := s.execSQL(context.Background(), "SELECT 1"); !errors.Is(err, wantErr) {
			t.Errorf("execSQL error = %v, want %v", err, wantErr)
		}
	})
}

func TestCommandList_dumpCommand_RejectsDirectoryTarget(t *testing.T) {
	// Regression for: a directory destination must be rejected, not
	// rewritten to a sibling .csv file.
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	exporter := mock.NewMockExportUsecase(ctrl)
	// Neither usecase should be called: the directory is rejected up front.

	dir := t.TempDir()
	s := newBoundaryTestShell(t, Usecases{metadata: metadata, export: exporter})

	err := NewCommands().dumpCommand(context.Background(), s, []string{"users", dir})
	if err == nil {
		t.Fatal("dumpCommand returned nil for a directory destination, want error")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("error = %q, want it to mention a directory", err.Error())
	}
}

func TestCommandList_dumpCommand_dependsOnMetadataAndExportUsecases(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	exporter := mock.NewMockExportUsecase(ctrl)

	outputPath := filepath.Join(t.TempDir(), "report.out")
	normalizedPath := model.BuildOutputPath(outputPath, model.ExportCSV, model.CompressionNone)
	table := model.NewTable("users", model.Header{"id", "name"}, nil)

	metadata.EXPECT().List(gomock.Any(), "users").Return(table, nil)
	// The exporter writes a scratch file beside the destination, which is then
	// moved onto it, so the path it is handed is not the one the user named. The
	// scratch file has to be created for the commit to have something to move, and
	// the assertion that the destination is the path the user named is the stderr
	// line below plus TestDumpWrite_SucceedsToANewPath.
	var staged string
	// UTF-8 rather than gomock.Any(): .dump creates a new file, so it must not
	// forward the session's import encoding the way a write-back does.
	exporter.EXPECT().
		DumpTable(gomock.Any(), table, model.ExportCSV, model.CompressionNone, model.TextEncodingUTF8).
		DoAndReturn(func(path string, _ *model.Table, _ model.ExportFormat, _ model.Compression, _ model.TextEncoding) error {
			staged = path
			return os.WriteFile(path, []byte("id,name\n"), 0o600)
		})

	s := newBoundaryTestShell(t, Usecases{
		metadata: metadata,
		export:   exporter,
	})
	s.files = defaultFileOps()

	// The dump status line is control-plane output and goes to stderr.
	out := captureStderr(t, func() {
		if err := NewCommands().dumpCommand(context.Background(), s, []string{"users", outputPath}); err != nil {
			t.Fatalf("dumpCommand returned error: %v", err)
		}
	})
	if !strings.Contains(out, "dump `") || !strings.Contains(out, "table to") {
		t.Fatalf("stderr %q does not describe dump execution", out)
	}
	if !strings.Contains(out, normalizedPath) {
		t.Fatalf("stderr %q does not include normalized csv path %q", out, normalizedPath)
	}
	if staged == normalizedPath {
		t.Error("the exporter wrote straight to the destination; a failure part-way would destroy it")
	}
	if filepath.Dir(staged) != filepath.Dir(normalizedPath) {
		t.Errorf("staged at %q, want it beside the destination %q so the commit is a rename within one filesystem",
			staged, normalizedPath)
	}
	if _, err := os.Stat(normalizedPath); err != nil {
		t.Errorf("the destination was not created: %v", err)
	}
}
