package sqltext_test

import (
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/sqltext"
)

// TestOpenDepth covers the nesting a continuation line would be written at,
// and the reason it cannot be counted off the raw string: a parenthesis inside
// a literal or a comment is text.
func TestOpenDepth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "nothing open", text: "SELECT 1", want: 0},
		{name: "one open", text: "SELECT count(", want: 1},
		{name: "two open", text: "SELECT count(sum(", want: 2},
		{name: "closed again", text: "SELECT count(x)", want: 0},
		{name: "a parenthesis in a string is text", text: "SELECT '('", want: 0},
		{name: "a parenthesis in a comment is text", text: "SELECT -- (\n", want: 0},
		{name: "a parenthesis in a quoted name is text", text: `SELECT "a("`, want: 0},
		{name: "more closers than openers cannot go below zero", text: "SELECT x))", want: 0},
		{name: "an open subquery", text: "SELECT * FROM (SELECT a FROM t WHERE b IN (", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sqltext.OpenDepth(tt.text, dialect.SQLite); got != tt.want {
				t.Errorf("OpenDepth(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}
