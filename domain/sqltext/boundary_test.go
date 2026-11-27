package sqltext

import (
	"slices"
	"testing"

	"github.com/nao1215/filesql/dialect"
)

// codeText returns the text of each token Tokens yields, so a case can say what
// the scanner took for code in one line.
func codeText(s string, d dialect.Dialect) []string {
	var got []string
	for tok := range Tokens(s, d) {
		got = append(got, tok.Text(s))
	}
	return got
}

// TestWordsIncludeTheEdgesOfEachCharacterRange pins the first and last letter
// and digit of each range a word is made of. An off-by-one there splits "z" or
// "x9" out of a word, which moves where a statement's keyword is read.
func TestWordsIncludeTheEdgesOfEachCharacterRange(t *testing.T) {
	t.Parallel()

	for _, word := range []string{"a", "z", "A", "Z", "x0", "x9", "_"} {
		if got := codeText(word, dialect.SQLite); !slices.Equal(got, []string{word}) {
			t.Errorf("Tokens(%q) = %q, want one word", word, got)
		}
	}
	for in, want := range map[string]string{"zap": "ZAP", "Zap": "ZAP", "a1": "A"} {
		if got := LeadingKeyword(in, dialect.SQLite); got != want {
			t.Errorf("LeadingKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestUnclosedBlockCommentEndingInAStar covers a comment cut off right after a
// "*": the scanner looks one byte ahead for "*/", and that byte is not there.
func TestUnclosedBlockCommentEndingInAStar(t *testing.T) {
	t.Parallel()

	for _, d := range []dialect.Dialect{dialect.SQLite, dialect.PostgreSQL} {
		for _, s := range []string{"/* a *", "/* a /", "SELECT 1 /* a *"} {
			if !EndsInsideBlockComment(s, d) {
				t.Errorf("EndsInsideBlockComment(%q, %v) = false, want true", s, d)
			}
		}
		if got := StripNoise("/* a *", d); got != "" {
			t.Errorf("StripNoise(%q, %v) = %q, want empty", "/* a *", d, got)
		}
	}
}

// TestNestedBlockCommentClosesAtTheOuterEnd checks that PostgreSQL's nested
// comment is open until its outer "*/", and closed after it.
func TestNestedBlockCommentClosesAtTheOuterEnd(t *testing.T) {
	t.Parallel()

	if EndsInsideBlockComment("/* /* */ */ SELECT 1", dialect.PostgreSQL) {
		t.Error("a nested comment closed by its outer */ reads as still open")
	}
	if !EndsInsideBlockComment("/* /* */ SELECT 1", dialect.PostgreSQL) {
		t.Error("a nested comment with only its inner */ reads as closed")
	}
}

// TestPostgreSQLEscapeStringPrefix covers where an E or e before a quote opens
// an escape string, in which \' does not end the literal. The semicolon inside
// the literal must not be reported as code.
func TestPostgreSQLEscapeStringPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []string
	}{
		{`E'a\'; b' x`, []string{"E", "x"}},
		{`e'a\'; b' x`, []string{"e", "x"}},
		{` E'a\'; b' x`, []string{"E", "x"}},
		// An E that ends a word is part of the word, not a prefix: the
		// backslash is ordinary and the second quote closes the literal.
		{`xE'a\'; b`, []string{"xE", ";", "b"}},
		{`xyE'a\'; b`, []string{"xyE", ";", "b"}},
	}
	for _, tt := range tests {
		if got := codeText(tt.in, dialect.PostgreSQL); !slices.Equal(got, tt.want) {
			t.Errorf("Tokens(%q, PostgreSQL) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestPostgreSQLDollarTags covers what may name a dollar-quoted string's tag: a
// word that does not start with a digit. "$0$" and "$9$" are parameters
// followed by a dollar, so the semicolon after them is code.
func TestPostgreSQLDollarTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []string
	}{
		{"$a0$;$a0$ x", []string{"x"}},
		{"$a9$;$a9$ x", []string{"x"}},
		{"$0$;$0$ x", []string{"0", ";", "0", "x"}},
		{"$9$;$9$ x", []string{"9", ";", "9", "x"}},
		// An empty body: the closing tag follows the opening one at once.
		{"$a$$a$; x", []string{";", "x"}},
	}
	for _, tt := range tests {
		if got := codeText(tt.in, dialect.PostgreSQL); !slices.Equal(got, tt.want) {
			t.Errorf("Tokens(%q, PostgreSQL) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
