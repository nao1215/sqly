package infrastructure

import (
	"testing"
)

// TestQuoteTableRef verifies that a bare name is backtick-quoted and a
// schema-qualified name is quoted per part, so helper commands can reference
// schema-qualified tables.
func TestQuoteTableRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"user", "`user`"},
		{"main.user", "`main`.`user`"},
		{"temp.t", "`temp`.`t`"},
		{"TEMP.t", "`TEMP`.`t`"}, // schema name match is case-insensitive
		// A dotted name whose prefix is not a real schema (main/temp) is a single
		// literal identifier, since sqly rejects ATTACH so no other schema exists.
		{"weird.name", "`weird.name`"},
		{"a.b", "`a.b`"},
		{".leadingdot", "`.leadingdot`"},
		{"trailingdot.", "`trailingdot.`"},
		{"has`tick", "`has``tick`"},
	}
	for _, tt := range tests {
		if got := QuoteTableRef(tt.in); got != tt.want {
			t.Errorf("QuoteTableRef(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
