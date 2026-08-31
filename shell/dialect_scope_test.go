package shell

import (
	"context"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
)

// TestParseScriptFollowsADialectSetInTheScript covers where a script's
// statements end. A ";" is a terminator only where the dialect says it is, and
// a script can change the dialect partway through with .dialect. The whole
// script was split up front by the dialect the process started with, so a
// statement written for the dialect the script had just selected was cut in the
// wrong place: with "#" opening a comment in MySQL, "SELECT 1 # note; more" is
// one statement there and two under SQLite's rules.
func TestParseScriptFollowsADialectSetInTheScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		script  string
		start   dialect.Dialect
		wantSQL []string
	}{
		{
			name:    "a hash comment after .dialect mysql does not split the statement",
			script:  ".dialect mysql\nSELECT 1 # note; more\n",
			start:   dialect.SQLite,
			wantSQL: []string{"SELECT 1 # note; more"},
		},
		{
			name:    "the same script under SQLite's rules splits at the semicolon",
			script:  "SELECT 1 # note; more\n",
			start:   dialect.SQLite,
			wantSQL: []string{"SELECT 1 # note", "more"},
		},
		{
			name:    "switching back to sqlite restores SQLite's rules",
			script:  ".dialect mysql\n.dialect sqlite\nSELECT 1 # note; more\n",
			start:   dialect.SQLite,
			wantSQL: []string{"SELECT 1 # note", "more"},
		},
		{
			name:    "a dollar-quoted string survives .dialect postgresql",
			script:  ".dialect postgresql\nSELECT $$a;b$$ AS s;\n",
			start:   dialect.SQLite,
			wantSQL: []string{"SELECT $$a;b$$ AS s"},
		},
		{
			name:    "a dialect name the command would reject leaves the rules alone",
			script:  ".dialect nosuch\nSELECT 1 # note; more\n",
			start:   dialect.SQLite,
			wantSQL: []string{"SELECT 1 # note", "more"},
		},
		{
			name:    "a bare .dialect prints the setting and changes nothing",
			script:  ".dialect\nSELECT 1 # note; more\n",
			start:   dialect.SQLite,
			wantSQL: []string{"SELECT 1 # note", "more"},
		},
		{
			name:    "the dialect the process started with applies before any .dialect",
			script:  "SELECT 1; # note\n",
			start:   dialect.MySQL,
			wantSQL: []string{"SELECT 1"},
		},
		{
			name:    "a .dialect that starts the script applies to every statement after it",
			script:  ".dialect mysql\nSELECT 1; # note\nSELECT 2; # note\n",
			start:   dialect.SQLite,
			wantSQL: []string{"SELECT 1", "SELECT 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			elements, err := parseScript(tt.script, tt.start)
			if err != nil {
				t.Fatalf("parseScript(%q, %v): %v", tt.script, tt.start, err)
			}
			var got []string
			for _, el := range elements {
				if el.kind == elementSQL {
					got = append(got, strings.TrimSpace(el.text))
				}
			}
			if len(got) != len(tt.wantSQL) {
				t.Fatalf("parseScript(%q, %v) SQL elements = %q, want %q", tt.script, tt.start, got, tt.wantSQL)
			}
			for i := range got {
				if got[i] != tt.wantSQL[i] {
					t.Errorf("statement %d = %q, want %q", i, got[i], tt.wantSQL[i])
				}
			}
		})
	}
}

// TestShellDialectFollowsTheDialectCommand covers which dialect the shell reads
// its own input with. Translation and execution used the dialect .dialect had
// set, while everything that decides where a statement ends used the one the
// process started with, so the two could disagree for the whole session.
func TestShellDialectFollowsTheDialectCommand(t *testing.T) {
	// Serial: newShell builds an in-memory DB and a temp history path.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if got := s.dialect(); got != dialect.SQLite {
		t.Fatalf("a session started without --dialect reads its input as %v, want %v", got, dialect.SQLite)
	}

	if err := s.commands.dialectCommand(context.Background(), s, []string{"mysql"}); err != nil {
		t.Fatalf(".dialect mysql: %v", err)
	}
	if got := s.dialect(); got != dialect.MySQL {
		t.Errorf("after .dialect mysql the shell reads its input as %v, want %v", got, dialect.MySQL)
	}
}

// TestSQLInputCompleteFollowsTheDialectCommand covers the interactive buffer.
// The prompt asks the shell whether what has been typed is a finished
// statement; asking under the startup dialect left "SELECT 1; # note" typed
// after .dialect mysql waiting on a continuation for a rest that had already
// been written, because "#" opens no comment in SQLite and the text after the
// terminator did not look like noise.
func TestSQLInputCompleteFollowsTheDialectCommand(t *testing.T) {
	// Serial: newShell builds an in-memory DB and a temp history path.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	const input = "SELECT 1; # note"

	if sqlInputComplete(input, s.dialect()) {
		t.Fatalf("%q is unfinished under SQLite's rules, where # opens no comment", input)
	}
	if err := s.commands.dialectCommand(context.Background(), s, []string{"mysql"}); err != nil {
		t.Fatalf(".dialect mysql: %v", err)
	}
	if !sqlInputComplete(input, s.dialect()) {
		t.Errorf("%q is a finished statement under MySQL's rules, but the prompt kept buffering it", input)
	}
}
