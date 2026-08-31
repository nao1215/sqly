package sqltext_test

import (
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/sqltext"
)

// region is one thing Regions reported, in a form a test can write down.
type region struct {
	kind sqltext.Kind
	text string
}

func regionsOf(s string, d dialect.Dialect) []region {
	var out []region
	for tok := range sqltext.Regions(s, d) {
		out = append(out, region{kind: tok.Kind, text: tok.Text(s)})
	}
	return out
}

// TestRegionsReportsWhatTokensSkips covers the walk that names the parts of a
// statement rather than seeing past them: the string literals, quoted
// identifiers, and comments Tokens leaves out, each with the extent it covers.
func TestRegionsReportsWhatTokensSkips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		dialect dialect.Dialect
		want    []region
	}{
		{
			name:    "a string literal is one region, quotes included",
			text:    "SELECT 'a b'",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.String, "'a b'"},
			},
		},
		{
			name:    "a doubled quote stays inside the literal that holds it",
			text:    "SELECT 'it''s'",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.String, "'it''s'"},
			},
		},
		{
			name:    "a doubled quote stays inside the name that holds it",
			text:    `SELECT "a""b"`,
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.QuotedIdentifier, `"a""b"`},
			},
		},
		{
			// SQLite's bracket quoting has no escape, so a doubled closer is two
			// brackets and the name ends at the first of them.
			name:    "a bracketed name has no doubling to undo",
			text:    "SELECT [a]]b]",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.QuotedIdentifier, "[a]"},
				{sqltext.Word, "b"},
			},
		},
		{
			// The newline is what ends a line comment, so it belongs to it.
			name:    "a line comment runs to the end of its line",
			text:    "SELECT -- note\nFROM t",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.Comment, "-- note\n"},
				{sqltext.Word, "FROM"},
				{sqltext.Word, "t"},
			},
		},
		{
			name:    "a block comment is one region",
			text:    "SELECT /* a\nb */ 1",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.Comment, "/* a\nb */"},
				{sqltext.Word, "1"},
			},
		},
		{
			name:    "a double-quoted run is a name in SQLite",
			text:    `SELECT "my col"`,
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.QuotedIdentifier, `"my col"`},
			},
		},
		{
			name:    "and a string in MySQL, which reads it that way",
			text:    `SELECT "my col"`,
			dialect: dialect.MySQL,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.String, `"my col"`},
			},
		},
		{
			name:    "a backtick quotes a name",
			text:    "SELECT `my col`",
			dialect: dialect.MySQL,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.QuotedIdentifier, "`my col`"},
			},
		},
		{
			name:    "a bracket quotes a name in SQLite",
			text:    "SELECT [my col]",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.QuotedIdentifier, "[my col]"},
			},
		},
		{
			name:    "a hash comment is one in MySQL",
			text:    "SELECT 1 # note",
			dialect: dialect.MySQL,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.Word, "1"},
				{sqltext.Comment, "# note"},
			},
		},
		{
			name:    "a dollar-quoted string is one region in PostgreSQL",
			text:    "SELECT $tag$a;b$tag$",
			dialect: dialect.PostgreSQL,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.String, "$tag$a;b$tag$"},
			},
		},
		{
			name:    "an unclosed literal runs to the end of the text",
			text:    "SELECT 'unfinished",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.String, "'unfinished"},
			},
		},
		{
			name:    "a semicolon is still reported",
			text:    "SELECT 1;",
			dialect: dialect.SQLite,
			want: []region{
				{sqltext.Word, "SELECT"},
				{sqltext.Word, "1"},
				{sqltext.Semicolon, ";"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := regionsOf(tt.text, tt.dialect)
			if len(got) != len(tt.want) {
				t.Fatalf("Regions(%q, %v) = %+v, want %+v", tt.text, tt.dialect, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("region %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestTokensStillReportsOnlyCode is the other half: the walk every existing
// caller uses is unchanged by Regions existing, and still sees past a literal,
// a name in quotes, and a comment.
func TestTokensStillReportsOnlyCode(t *testing.T) {
	t.Parallel()

	const text = "SELECT 'a', \"b\", `c` -- note\nFROM t;"
	var got []string
	for tok := range sqltext.Tokens(text, dialect.SQLite) {
		if tok.Kind != sqltext.Word && tok.Kind != sqltext.Semicolon {
			t.Fatalf("Tokens yielded a %v, which is not code", tok.Kind)
		}
		got = append(got, tok.Text(text))
	}

	want := []string{"SELECT", "FROM", "t", ";"}
	if len(got) != len(want) {
		t.Fatalf("Tokens(%q) = %q, want %q", text, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRegionsCoverTheirText checks that every region reported is a real slice
// of the text and that they arrive in order without overlapping. A caller
// coloring the input walks them expecting exactly that.
func TestRegionsCoverTheirText(t *testing.T) {
	t.Parallel()

	texts := []string{
		"", "SELECT", "SELECT 'a' /* b */ -- c\n`d` [e] \"f\" $$g$$ 1;",
		"'unclosed", "/* unclosed", "SELECT 日本語 AS '名前'", ";;;", "((()))",
	}
	dialects := []dialect.Dialect{dialect.SQLite, dialect.MySQL, dialect.PostgreSQL, dialect.GoogleSQL}

	for _, text := range texts {
		for _, d := range dialects {
			end := 0
			for tok := range sqltext.Regions(text, d) {
				switch {
				case tok.Start < end:
					t.Errorf("Regions(%q, %v): a region starts at %d, inside the one ending at %d", text, d, tok.Start, end)
				case tok.Start > tok.End:
					t.Errorf("Regions(%q, %v): a region runs backwards, %d to %d", text, d, tok.Start, tok.End)
				case tok.End > len(text):
					t.Errorf("Regions(%q, %v): a region ends at %d, past the text", text, d, tok.End)
				}
				end = tok.End
			}
		}
	}
}

// TestUnquote covers reading the name out of a quoted one: the delimiters off,
// a doubled delimiter collapsed, and a backslash escape resolved where the
// dialect has them.
//
// Getting it wrong costs a caller a match rather than a crash -- a highlighter
// simply does not recognize the name -- which is why it is worth pinning.
func TestUnquote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		dialect dialect.Dialect
		want    string
	}{
		{name: "a double-quoted name", text: `"col"`, dialect: dialect.SQLite, want: "col"},
		{name: "a backtick-quoted name", text: "`col`", dialect: dialect.MySQL, want: "col"},
		{name: "a bracketed name", text: "[col]", dialect: dialect.SQLite, want: "col"},
		{name: "a name holding a space", text: `"my col"`, dialect: dialect.SQLite, want: "my col"},
		{name: "a doubled double quote is one quote", text: `"a""b"`, dialect: dialect.SQLite, want: `a"b`},
		{name: "a doubled backtick is one backtick", text: "`a``b`", dialect: dialect.MySQL, want: "a`b"},
		{name: "a doubled single quote is one quote", text: "'it''s'", dialect: dialect.SQLite, want: "it's"},
		{
			// GoogleSQL alone escapes inside a backtick-quoted name, which is
			// the reason this takes the dialect at all.
			name: "a backslash escape inside a backtick resolves in GoogleSQL",
			text: "`a\\`b`", dialect: dialect.GoogleSQL, want: "a`b",
		},
		{
			name: "and stays as written where the dialect has no such escape",
			text: "`a\\`b`", dialect: dialect.MySQL, want: "a\\`b",
		},
		{
			name: "a backslash escape inside a string resolves in MySQL",
			text: `'a\'b'`, dialect: dialect.MySQL, want: "a'b",
		},
		{
			// SQLite has no backslash escape, so the backslash is a character.
			name: "and stays as written in SQLite",
			text: `'a\b'`, dialect: dialect.SQLite, want: `a\b`,
		},
		{
			// SQLite's bracket quoting has no escape at all.
			name: "a bracketed name has nothing to undouble",
			text: "[a]]b]", dialect: dialect.SQLite, want: "a]]b",
		},
		{name: "an empty quoted name", text: `""`, dialect: dialect.SQLite, want: ""},
		{name: "something too short to be quoted is left alone", text: `"`, dialect: dialect.SQLite, want: `"`},
		{name: "text that is not quoted is left alone", text: "col", dialect: dialect.SQLite, want: "col"},
		{name: "text opened but not closed by a delimiter is left alone", text: `"col`, dialect: dialect.SQLite, want: `"col`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sqltext.Unquote(tt.text, tt.dialect); got != tt.want {
				t.Errorf("Unquote(%q, %v) = %q, want %q", tt.text, tt.dialect, got, tt.want)
			}
		})
	}
}
