package shell

import (
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/prompt"
)

// testTheme is a theme whose colors are distinct enough to name in a failure,
// so a test says which role was wrong rather than which RGB triple was.
func testTheme(t *testing.T) syntaxTheme {
	t.Helper()
	theme, ok := lookupTheme("dracula")
	if !ok {
		t.Fatal("the dracula theme is missing, so these tests cover nothing")
	}
	return theme
}

// roleOf names the role a color belongs to, for a readable failure.
func roleOf(theme syntaxTheme, c prompt.Color) string {
	switch c {
	case theme.keyword:
		return "keyword"
	case theme.str:
		return "string"
	case theme.comment:
		return "comment"
	case theme.number:
		return "number"
	case theme.quoted:
		return "quoted"
	case theme.command:
		return "command"
	default:
		return "unknown"
	}
}

// highlighted returns each colored run of input as "role:text", which is what a
// test can write down and read back.
func highlighted(t *testing.T, input string, d dialect.Dialect) []string {
	t.Helper()

	theme := testTheme(t)
	runes := []rune(input)
	spans := highlightSQL(input, theme, d)
	out := make([]string, 0, len(spans))
	for _, span := range spans {
		out = append(out, roleOf(theme, span.Color)+":"+string(runes[span.Start:span.End]))
	}
	return out
}

// TestHighlightSQL covers which parts of a statement are colored and as what.
func TestHighlightSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		dialect dialect.Dialect
		want    []string
	}{
		{
			name:    "keywords are colored and names are not",
			input:   "SELECT name FROM users",
			dialect: dialect.SQLite,
			want:    []string{"keyword:SELECT", "keyword:FROM"},
		},
		{
			name:    "a keyword typed in lower case is still a keyword",
			input:   "select name from users",
			dialect: dialect.SQLite,
			want:    []string{"keyword:select", "keyword:from"},
		},
		{
			name:    "a string literal is colored whole, quotes included",
			input:   "WHERE city = 'Tokyo'",
			dialect: dialect.SQLite,
			want:    []string{"keyword:WHERE", "string:'Tokyo'"},
		},
		{
			name:    "a keyword inside a string literal is text",
			input:   "SELECT 'FROM users'",
			dialect: dialect.SQLite,
			want:    []string{"keyword:SELECT", "string:'FROM users'"},
		},
		{
			name:    "a comment is colored to the end of its line",
			input:   "SELECT 1 -- FROM users",
			dialect: dialect.SQLite,
			want:    []string{"keyword:SELECT", "number:1", "comment:-- FROM users"},
		},
		{
			name:    "a number is colored",
			input:   "WHERE age = 25",
			dialect: dialect.SQLite,
			want:    []string{"keyword:WHERE", "number:25"},
		},
		{
			name:    "a quoted name is colored as one",
			input:   `SELECT "my col" FROM t`,
			dialect: dialect.SQLite,
			want:    []string{"keyword:SELECT", `quoted:"my col"`, "keyword:FROM"},
		},
		{
			name:    "a helper command marks its name and nothing else",
			input:   ".import ./data.csv",
			dialect: dialect.SQLite,
			want:    []string{"command:.import"},
		},
		{
			name:    "an unfinished string is still colored as one",
			input:   "SELECT 'unfinished",
			dialect: dialect.SQLite,
			want:    []string{"keyword:SELECT", "string:'unfinished"},
		},
		{
			name:    "a hash comment is one in MySQL",
			input:   "SELECT 1 # note",
			dialect: dialect.MySQL,
			want:    []string{"keyword:SELECT", "number:1", "comment:# note"},
		},
		{
			name:    "and is not one in PostgreSQL",
			input:   "SELECT 1 # note",
			dialect: dialect.PostgreSQL,
			want:    []string{"keyword:SELECT", "number:1"},
		},
		{
			name:    "an empty line has nothing to color",
			input:   "",
			dialect: dialect.SQLite,
			want:    nil,
		},
		{
			name:    "a multi-line statement is colored across its lines",
			input:   "SELECT name\nFROM users",
			dialect: dialect.SQLite,
			want:    []string{"keyword:SELECT", "keyword:FROM"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := highlighted(t, tt.input, tt.dialect)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("highlightSQL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestHighlightSQLOffsetsAreRunes covers what the prompt measures spans in. A
// byte offset would put the color on the wrong characters as soon as the line
// holds anything outside ASCII, which a table named in Japanese does.
func TestHighlightSQLOffsetsAreRunes(t *testing.T) {
	t.Parallel()

	const input = "SELECT '日本語' FROM 売上"
	got := highlighted(t, input, dialect.SQLite)
	want := []string{"keyword:SELECT", "string:'日本語'", "keyword:FROM"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("highlightSQL(%q) = %v, want %v", input, got, want)
	}
}

// TestHighlightSQLWithNoThemeColorsNothing covers the way out. "none" is for a
// terminal or a reader that wants the input plain.
func TestHighlightSQLWithNoThemeColorsNothing(t *testing.T) {
	t.Parallel()

	theme, ok := lookupTheme(noHighlightTheme)
	if !ok {
		t.Fatalf("the %q theme is missing", noHighlightTheme)
	}
	if got := highlightSQL("SELECT 'a' -- b", theme, dialect.SQLite); got != nil {
		t.Errorf("the %q theme colored %d run(s), want none", noHighlightTheme, len(got))
	}
}

// TestHighlightSQLSpansAreWellFormed is a property over the shapes a line takes
// while it is being typed. The prompt normalizes what it is given, so a bad span
// costs nothing visible -- but a highlighter that produces them is wrong, and
// this is where that shows.
func TestHighlightSQLSpansAreWellFormed(t *testing.T) {
	t.Parallel()

	fragments := []string{
		"", " ", "'", `"`, "`", "[", "--", "/*", "#", "$$", "SELECT", "1", ".mode", "日本",
		"\n", ";", "(", ")",
	}
	dialects := []dialect.Dialect{dialect.SQLite, dialect.MySQL, dialect.PostgreSQL, dialect.GoogleSQL}
	theme := testTheme(t)

	for _, a := range fragments {
		for _, b := range fragments {
			for _, c := range fragments {
				input := a + b + c
				limit := len([]rune(input))
				for _, d := range dialects {
					end := 0
					for _, span := range highlightSQL(input, theme, d) {
						switch {
						case span.Start < 0 || span.End > limit:
							t.Fatalf("highlightSQL(%q, %v): span [%d,%d) is outside the input of %d runes", input, d, span.Start, span.End, limit)
						case span.Start >= span.End:
							t.Fatalf("highlightSQL(%q, %v): span [%d,%d) is empty or inverted", input, d, span.Start, span.End)
						case span.Start < end:
							t.Fatalf("highlightSQL(%q, %v): span [%d,%d) overlaps the one ending at %d", input, d, span.Start, span.End, end)
						}
						end = span.End
					}
				}
			}
		}
	}
}

// TestEveryThemeNamesEveryRole checks the themes themselves: a role left unset
// in one of them would draw that token in the prompt's input color, which reads
// as a missing highlight rather than a deliberate one.
func TestEveryThemeNamesEveryRole(t *testing.T) {
	t.Parallel()

	for _, name := range themeNames() {
		theme, ok := lookupTheme(name)
		if !ok {
			t.Fatalf("themeNames listed %q, which lookupTheme does not have", name)
		}
		if theme.name != name {
			t.Errorf("theme %q calls itself %q", name, theme.name)
		}
		if theme.prompt == nil {
			t.Errorf("theme %q names no prompt scheme", name)
		}
		if !theme.highlights() {
			continue // "none" names no colors on purpose
		}
		roles := map[string]prompt.Color{
			"keyword": theme.keyword, "string": theme.str, "comment": theme.comment,
			"number": theme.number, "quoted": theme.quoted, "command": theme.command,
		}
		for role, color := range roles {
			if (color == prompt.Color{}) {
				t.Errorf("theme %q leaves %s unset, so it draws in the input color", name, role)
			}
		}
	}
}

// TestLookupThemeIgnoresCaseAndBlanks covers a name as it is typed rather than
// as it is stored.
func TestLookupThemeIgnoresCaseAndBlanks(t *testing.T) {
	t.Parallel()

	for _, spelling := range []string{"dracula", "Dracula", "DRACULA", "  dracula  "} {
		theme, ok := lookupTheme(spelling)
		if !ok || theme.name != "dracula" {
			t.Errorf("lookupTheme(%q) = %q/%v, want dracula", spelling, theme.name, ok)
		}
	}
	if _, ok := lookupTheme("no-such-theme"); ok {
		t.Error("lookupTheme accepted a name no theme has")
	}
}
