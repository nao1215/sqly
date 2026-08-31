package shell

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/prompt"
	"github.com/nao1215/sqly/config"
)

// TestIsEditRequest covers which lines the interactive loop takes for itself.
func TestIsEditRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantEdit  bool
		wantError bool
	}{
		{name: ".edit alone is the command", input: ".edit", wantEdit: true},
		{name: "surrounding space does not hide it", input: "  .edit  ", wantEdit: true},
		{name: ".edit with an argument is the command, written wrong", input: ".edit query.sql", wantEdit: true, wantError: true},
		{name: "a command that merely starts with the same letters is not it", input: ".exit", wantEdit: false},
		{name: "a SQL statement is not it", input: "SELECT 1", wantEdit: false},
		{name: "an empty line is not it", input: "", wantEdit: false},
		{name: "an unterminated quote is not it", input: `.edit "oops`, wantEdit: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotEdit, err := isEditRequest(tt.input)
			if gotEdit != tt.wantEdit {
				t.Fatalf("isEditRequest(%q) = %v, want %v", tt.input, gotEdit, tt.wantEdit)
			}
			if (err != nil) != tt.wantError {
				t.Errorf("isEditRequest(%q) error = %v, want an error: %v", tt.input, err, tt.wantError)
			}
		})
	}
}

// newEditorShell builds a shell whose editor is a function rather than a
// process, so what .edit hands the editor and what it does with what comes back
// can be checked without a terminal.
func newEditorShell(t *testing.T, env map[string]string, edit func(path string) error) *Shell {
	t.Helper()

	s := newBoundaryTestShell(t, Usecases{})
	s.getenv = func(name string) string { return env[name] }
	s.editorRunner = func(_ context.Context, _ []string, path string) error { return edit(path) }
	return s
}

// TestRunEditorRoundTripsThroughTheFile covers the whole exchange: the last
// statement reaches the editor, and what the editor saves comes back.
func TestRunEditorRoundTripsThroughTheFile(t *testing.T) {
	t.Parallel()

	var seen string
	s := newEditorShell(t, map[string]string{"EDITOR": "fake"}, func(path string) error {
		contents, err := os.ReadFile(path) //nolint:gosec // the path is the temp file under test
		if err != nil {
			return err
		}
		seen = string(contents)
		return os.WriteFile(path, []byte("SELECT 2;\n"), 0o600)
	})

	got, err := s.runEditor(context.Background(), "SELECT 1;")
	if err != nil {
		t.Fatalf("runEditor: %v", err)
	}
	if want := "SELECT 1;\n"; seen != want {
		t.Errorf("the editor was given %q, want %q", seen, want)
	}
	if want := "SELECT 2;"; got != want {
		t.Errorf("runEditor returned %q, want %q", got, want)
	}
}

// TestRunEditorGivesTheEditorASQLFile checks the suffix. It is what makes an
// editor highlight the buffer as SQL, which is most of the reason to leave the
// prompt for one.
func TestRunEditorGivesTheEditorASQLFile(t *testing.T) {
	t.Parallel()

	var seenPath string
	s := newEditorShell(t, map[string]string{"EDITOR": "fake"}, func(path string) error {
		seenPath = path
		return nil
	})

	if _, err := s.runEditor(context.Background(), ""); err != nil {
		t.Fatalf("runEditor: %v", err)
	}
	if filepath.Ext(seenPath) != ".sql" {
		t.Errorf("the editor was given %q, want a path ending in .sql", seenPath)
	}
}

// TestRunEditorRemovesWhatItWrote covers the scratch directory: an editor is
// opened often in a long session, and a temp file per edit is a leak.
func TestRunEditorRemovesWhatItWrote(t *testing.T) {
	t.Parallel()

	var seenPath string
	s := newEditorShell(t, map[string]string{"EDITOR": "fake"}, func(path string) error {
		seenPath = path
		// An editor that saves through a temp file of its own leaves it beside
		// ours; the whole directory is what has to go.
		return os.WriteFile(filepath.Join(filepath.Dir(path), "swap.swp"), []byte("x"), 0o600)
	})

	if _, err := s.runEditor(context.Background(), "SELECT 1;"); err != nil {
		t.Fatalf("runEditor: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(seenPath)); !os.IsNotExist(err) {
		t.Errorf("the edit scratch directory %q outlived the edit (stat err: %v)", filepath.Dir(seenPath), err)
	}
}

// TestRunEditorReportsAnAbandonedEdit covers the editor exiting non-zero, which
// is how an editor says the edit was abandoned (":cq" in vim). Running whatever
// happens to be in the file is the one outcome the user did not ask for.
func TestRunEditorReportsAnAbandonedEdit(t *testing.T) {
	t.Parallel()

	refused := errors.New("vim exited with status 1; nothing was run")
	s := newEditorShell(t, map[string]string{"EDITOR": "vim"}, func(path string) error {
		// The file still holds a statement, so only the error can stop it running.
		if err := os.WriteFile(path, []byte("DROP TABLE t;\n"), 0o600); err != nil {
			return err
		}
		return refused
	})

	got, err := s.runEditor(context.Background(), "SELECT 1;")
	if !errors.Is(err, refused) {
		t.Fatalf("runEditor error = %v, want the editor's own refusal", err)
	}
	if got != "" {
		t.Errorf("runEditor returned %q after an abandoned edit, want nothing to run", got)
	}
}

// TestRunEditorReturnsNothingForAnEmptyFile covers the buffer emptied and
// saved, which is how a user says "never mind" without the editor failing.
func TestRunEditorReturnsNothingForAnEmptyFile(t *testing.T) {
	t.Parallel()

	for _, contents := range []string{"", "\n", "   \n\t\n"} {
		s := newEditorShell(t, map[string]string{"EDITOR": "fake"}, func(path string) error {
			return os.WriteFile(path, []byte(contents), 0o600)
		})
		got, err := s.runEditor(context.Background(), "SELECT 1;")
		if err != nil {
			t.Fatalf("runEditor with %q saved: %v", contents, err)
		}
		if got != "" {
			t.Errorf("runEditor with %q saved returned %q, want nothing to run", contents, got)
		}
	}
}

// TestEditorCommand covers which editor is chosen and how it is read.
func TestEditorCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    []string
		wantErr string
	}{
		{
			name: "EDITOR names the editor",
			env:  map[string]string{"EDITOR": "vim"},
			want: []string{"vim"},
		},
		{
			name: "VISUAL wins where both are set",
			env:  map[string]string{"VISUAL": "nvim", "EDITOR": "vim"},
			want: []string{"nvim"},
		},
		{
			name: "an editor's flags come with it",
			env:  map[string]string{"EDITOR": "code -w"},
			want: []string{"code", "-w"},
		},
		{
			name: "a quoted path holding a space stays one word",
			env:  map[string]string{"EDITOR": `"/opt/my editor/bin" -f`},
			want: []string{"/opt/my editor/bin", "-f"},
		},
		{
			name: "a variable set to blanks is not an editor",
			env:  map[string]string{"VISUAL": "   ", "EDITOR": "vim"},
			want: []string{"vim"},
		},
		{
			name:    "neither set says how to set one",
			env:     map[string]string{},
			wantErr: "no editor is set",
		},
		{
			name:    "an unreadable value says so rather than running half of it",
			env:     map[string]string{"EDITOR": `vim "unterminated`},
			wantErr: "cannot be read as a command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newEditorShell(t, tt.env, func(string) error { return nil })
			got, err := s.editorCommand()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("editorCommand() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("editorCommand(): %v", err)
			}
			if strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
				t.Errorf("editorCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEditCommandOutsideTheLoopSaysWhatToDoInstead covers a script reaching
// .edit. The command is in the table so .help lists it and completion offers
// it, but a script has no terminal to hand to an editor.
func TestEditCommandOutsideTheLoopSaysWhatToDoInstead(t *testing.T) {
	t.Parallel()

	err := NewCommands().editCommand(context.Background(), nil, nil)
	if err == nil {
		t.Fatal(".edit succeeded outside an interactive session, want a refusal")
	}
	var invocationErr *invocationError
	if !errors.As(err, &invocationErr) {
		t.Errorf(".edit outside an interactive session = %T, want an invocationError so the exit code says the invocation was wrong", err)
	}
	if !strings.Contains(err.Error(), "--sql-file") {
		t.Errorf(".edit refusal = %v, want it to name the way to run SQL from a file", err)
	}
}

// TestLastStatementIsWhatEditOpens covers what .edit is seeded with: the SQL
// last submitted, including one that failed, and never a helper command.
func TestLastStatementIsWhatEditOpens(t *testing.T) {
	// Serial: newShell builds an in-memory DB and a temp history path.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()
	if err := s.execInteractive(ctx, "CREATE TABLE t (a);"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	if want := "CREATE TABLE t (a);"; s.lastStatement != want {
		t.Fatalf("lastStatement = %q, want %q", s.lastStatement, want)
	}

	// A helper command is retyped in less time than an editor takes to start, so
	// it does not displace the statement worth editing.
	if err := s.execInteractive(ctx, ".tables"); err != nil {
		t.Fatalf(".tables: %v", err)
	}
	if want := "CREATE TABLE t (a);"; s.lastStatement != want {
		t.Errorf("a helper command displaced the statement .edit opens: %q, want %q", s.lastStatement, want)
	}

	// A statement that failed is the one most worth opening again.
	if err := s.execInteractive(ctx, "SELECT nope FROM t;"); err == nil {
		t.Fatal("the failing statement succeeded, so this case proves nothing")
	}
	if want := "SELECT nope FROM t;"; s.lastStatement != want {
		t.Errorf("a failed statement was not kept for .edit: %q, want %q", s.lastStatement, want)
	}
}

// TestCommunicateEditClosesAndReopensThePromptSession covers the exchange the
// interactive loop performs for .edit. A prompt owns the terminal while it
// lives -- raw mode, and a goroutine reading the terminal -- so an editor
// started underneath it would draw on a terminal it does not control and lose
// its keystrokes to sqly. The session is given up for the editor and a new one
// is taken after.
func TestCommunicateEditClosesAndReopensThePromptSession(t *testing.T) {
	// Serial: replaces config.Stdout, which is a package-level writer.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	sessions := []*fakePromptSession{
		{results: []string{"SELECT 1 AS first;", ".edit"}, exhaustErr: io.EOF},
		{results: []string{".exit"}},
	}
	factoryCalls := 0
	s.newPrompt = func(string, func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		session := sessions[min(factoryCalls, len(sessions)-1)]
		factoryCalls++
		return session, nil
	}
	s.isTTY = func() bool { return true }

	var seededWith string
	s.getenv = func(name string) string {
		if name == "EDITOR" {
			return "fake"
		}
		return ""
	}
	s.editorRunner = func(_ context.Context, _ []string, path string) error {
		contents, err := os.ReadFile(path) //nolint:gosec // the path is the temp file under test
		if err != nil {
			return err
		}
		seededWith = string(contents)
		return os.WriteFile(path, []byte("SELECT 'EDITED' AS e;\n"), 0o600)
	}

	backupStdout := config.Stdout
	defer func() { config.Stdout = backupStdout }()
	var stdout bytes.Buffer
	config.Stdout = &stdout

	if err := s.communicate(context.Background()); err != nil {
		t.Fatalf("communicate: %v", err)
	}

	// The editor was given the statement that ran before .edit.
	if want := "SELECT 1 AS first;\n"; seededWith != want {
		t.Errorf("the editor was seeded with %q, want %q", seededWith, want)
	}
	// The first session was closed for the editor and a second was opened.
	if sessions[0].closeCalls == 0 {
		t.Error("the prompt session was not closed for the editor, so the editor ran under a terminal sqly still owned")
	}
	if factoryCalls < 2 {
		t.Errorf("the prompt factory ran %d time(s), want a fresh session opened after the editor", factoryCalls)
	}
	// What the editor saved ran, and was echoed so it can be told from the
	// result before it.
	if !strings.Contains(stdout.String(), "EDITED") {
		t.Errorf("what the editor saved did not run: %q", stdout.String())
	}
}

// TestCommunicateEditWithoutAnEditorKeepsTheSessionAlive covers .edit in a
// session that has no editor to open. It is a command that failed, not a
// session that ends: the prompt comes back and the next statement runs.
func TestCommunicateEditWithoutAnEditorKeepsTheSessionAlive(t *testing.T) {
	// Serial: replaces config.Stdout and config.Stderr, which are package-level.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	sessions := []*fakePromptSession{
		{results: []string{".edit"}, exhaustErr: io.EOF},
		{results: []string{"SELECT 'ALIVE' AS a;", ".exit"}},
	}
	factoryCalls := 0
	s.newPrompt = func(string, func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		session := sessions[min(factoryCalls, len(sessions)-1)]
		factoryCalls++
		return session, nil
	}
	s.isTTY = func() bool { return true }
	s.getenv = func(string) string { return "" } // neither VISUAL nor EDITOR

	backupStdout, backupStderr := config.Stdout, config.Stderr
	defer func() { config.Stdout, config.Stderr = backupStdout, backupStderr }()
	var stdout, stderr bytes.Buffer
	config.Stdout, config.Stderr = &stdout, &stderr

	if err := s.communicate(context.Background()); err != nil {
		t.Fatalf("communicate: %v", err)
	}

	if !strings.Contains(stderr.String(), "no editor is set") {
		t.Errorf("stderr = %q, want it to say no editor is set", stderr.String())
	}
	if !strings.Contains(stdout.String(), "ALIVE") {
		t.Errorf("the session did not survive a .edit with no editor: %q", stdout.String())
	}
}
