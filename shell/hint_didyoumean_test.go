package shell

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnknownCommandSuggestsTheNearestOne covers a mistyped dot-command. The
// name is one of sixteen sqly already knows, so a typo in it has an answer the
// shell can give rather than leaving the user to run .help and read.
func TestUnknownCommandSuggestsTheNearestOne(t *testing.T) {
	// Serial: newShell builds an in-memory DB and a temp history path.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tests := []struct {
		name       string
		input      string
		wantSuffix string
	}{
		{name: "a swapped pair of letters names the command", input: ".tabels", wantSuffix: `Did you mean ".tables"?`},
		{name: "a dropped letter names the command", input: ".modee", wantSuffix: `Did you mean ".mode"?`},
		{name: "a command nothing resembles is not guessed at", input: ".zzzzzzzz", wantSuffix: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.dispatch(context.Background(), tt.input)
			if err == nil {
				t.Fatalf("dispatch(%q) succeeded, want an unknown-command error", tt.input)
			}
			if !strings.Contains(err.Error(), "no such sqly command") {
				t.Fatalf("dispatch(%q) = %v, want an unknown-command error", tt.input, err)
			}
			if tt.wantSuffix == "" {
				if strings.Contains(err.Error(), "Did you mean") {
					t.Errorf("dispatch(%q) guessed at a command: %v", tt.input, err)
				}
				return
			}
			if !strings.Contains(err.Error(), tt.wantSuffix) {
				t.Errorf("dispatch(%q) = %v, want it to contain %q", tt.input, err, tt.wantSuffix)
			}
		})
	}
}

// TestMissingTableHintSuggestsTheNearestTable covers a mistyped table name. The
// hint already lists the session's tables, which answers "what is it called" for
// a small session; naming the one that is a typo away answers it directly, and
// for a session holding a table per sheet of a workbook the list is truncated
// and the nearest name may not even be in it.
func TestMissingTableHintSuggestsTheNearestTable(t *testing.T) {
	// Serial: newShell builds an in-memory DB and a temp history path.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()
	if err := s.commands.importCommand(ctx, s, []string{filepath.Join("testdata", "user.csv")}); err != nil {
		t.Fatalf("import: %v", err)
	}

	got := s.withMissingNameHint(ctx, errors.New(`SQL logic error: no such table: uesr (1)`))
	if !strings.Contains(got.Error(), `Did you mean "user"?`) {
		t.Errorf("hint for a mistyped table = %v, want it to name the table it is a typo of", got)
	}

	// A name nothing resembles still gets the list, and no guess.
	got = s.withMissingNameHint(ctx, errors.New(`SQL logic error: no such table: zzzzzzzz (1)`))
	if strings.Contains(got.Error(), "Did you mean") {
		t.Errorf("hint guessed at a table nothing resembles: %v", got)
	}
	if !strings.Contains(got.Error(), "Available tables") {
		t.Errorf("hint for an unrecognizable table dropped the list of what is there: %v", got)
	}
}

// TestMissingColumnHintNamesTheNearestColumn covers a mistyped column name.
// The hint used to say only "run .describe", which is a step the shell can take
// itself: it holds every column of every table it has imported.
func TestMissingColumnHintNamesTheNearestColumn(t *testing.T) {
	// Serial: newShell builds an in-memory DB and a temp history path.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()
	if err := s.commands.importCommand(ctx, s, []string{filepath.Join("testdata", "user.csv")}); err != nil {
		t.Fatalf("import: %v", err)
	}

	got := s.withMissingNameHint(ctx, errors.New(`SQL logic error: no such column: idnetifier (1)`))
	if !strings.Contains(got.Error(), `Did you mean "identifier"?`) {
		t.Errorf("hint for a mistyped column = %v, want it to name the column it is a typo of", got)
	}

	// A name nothing resembles keeps the advice that does not depend on guessing.
	got = s.withMissingNameHint(ctx, errors.New(`SQL logic error: no such column: zzzzzzzz (1)`))
	if strings.Contains(got.Error(), "Did you mean") {
		t.Errorf("hint guessed at a column nothing resembles: %v", got)
	}
	if !strings.Contains(got.Error(), ".describe") {
		t.Errorf("hint for an unrecognizable column dropped the advice: %v", got)
	}
}
