package shell

import (
	"testing"

	"github.com/nao1215/filesql/dialect"
)

// TestSQLIndent covers what a continuation line opens with. The rule is the
// indentation of the line being continued, plus or minus a level for each
// parenthesis that line opened or closed.
func TestSQLIndent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		before string
		want   string
	}{
		{
			name:   "a first line at the margin continues at the margin",
			before: "SELECT name",
			want:   "",
		},
		{
			name:   "an open parenthesis steps in one level",
			before: "SELECT * FROM users WHERE id IN (",
			want:   indentUnit,
		},
		{
			name:   "two open parentheses on one line step in two",
			before: "SELECT count((",
			want:   indentUnit + indentUnit,
		},
		{
			name:   "a parenthesis opened and closed on one line changes nothing",
			before: "SELECT count(x)",
			want:   "",
		},
		{
			name:   "the indentation of the line being continued is kept",
			before: "SELECT a\n  , b",
			want:   "  ",
		},
		{
			name:   "a line already indented steps in from where it is",
			before: "SELECT a\n  FROM (",
			want:   "  " + indentUnit,
		},
		{
			name:   "closing a parenthesis steps back out",
			before: "SELECT * FROM (\n" + indentUnit + "SELECT 1)",
			want:   "",
		},
		{
			name:   "a line that closes more than it opened stops at the margin",
			before: "SELECT x)))",
			want:   "",
		},
		{
			name:   "a tab counts as a level when stepping out",
			before: "SELECT * FROM (\n\tSELECT 1)",
			want:   "",
		},
		{
			name:   "a parenthesis inside a string literal indents nothing",
			before: "SELECT '('",
			want:   "",
		},
		{
			name:   "a parenthesis inside a comment indents nothing",
			before: "SELECT 1 -- (",
			want:   "",
		},
		{
			name:   "a parenthesis inside a quoted name indents nothing",
			before: `SELECT "a("`,
			want:   "",
		},
		{
			name:   "a helper command takes no continuation",
			before: ".import (",
			want:   "",
		},
		{
			name:   "an empty line continues at the margin",
			before: "",
			want:   "",
		},
		{
			name:   "a blank line after an indented one keeps its blanks",
			before: "SELECT (\n" + indentUnit,
			want:   indentUnit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sqlIndent(tt.before, dialect.SQLite); got != tt.want {
				t.Errorf("sqlIndent(%q) = %q, want %q", tt.before, got, tt.want)
			}
		})
	}
}

// TestSQLIndentReadsTheDialect checks that the count asks the dialect what a
// comment is: "#" opens one in MySQL, where the parenthesis behind it is text,
// and opens nothing in PostgreSQL, where it is a parenthesis.
func TestSQLIndentReadsTheDialect(t *testing.T) {
	t.Parallel()

	const before = "SELECT 1 # ("
	if got := sqlIndent(before, dialect.MySQL); got != "" {
		t.Errorf("MySQL: sqlIndent(%q) = %q, want no indent (the parenthesis is inside a comment)", before, got)
	}
	if got := sqlIndent(before, dialect.PostgreSQL); got != indentUnit {
		t.Errorf("PostgreSQL: sqlIndent(%q) = %q, want one level (# opens no comment there)", before, got)
	}
}

// TestSQLIndentIsOnlyBlanks is a property over the shapes a statement takes
// while it is being written: whatever the indenter returns is inserted into the
// input, so it has to be whitespace. Anything else would be writing SQL on the
// user's behalf.
func TestSQLIndentIsOnlyBlanks(t *testing.T) {
	t.Parallel()

	fragments := []string{
		"", " ", "(", ")", "SELECT", "'", "\"", "--", "/*", "#", "\n", "\t", "$$", "[", "`", ";",
	}
	dialects := []dialect.Dialect{dialect.SQLite, dialect.MySQL, dialect.PostgreSQL, dialect.GoogleSQL}

	// Every pair and triple of the fragments above, which reaches the unclosed
	// and unbalanced shapes a line passes through as it is typed.
	for _, a := range fragments {
		for _, b := range fragments {
			for _, c := range fragments {
				before := a + b + c
				for _, d := range dialects {
					got := sqlIndent(before, d)
					for _, r := range got {
						if r != ' ' && r != '\t' {
							t.Fatalf("sqlIndent(%q, %v) = %q, which is not whitespace", before, d, got)
						}
					}
				}
			}
		}
	}
}
