package shell

import "testing"

// TestCurrentCompletionWordCountsTheBackslashesBeforeASpace checks where the
// word being completed starts: after the last space no odd run of backslashes
// escapes, including at the very start of the input.
func TestCurrentCompletionWordCountsTheBackslashesBeforeASpace(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]string{
		`.import my\ dir/in`:   `my\ dir/in`,
		`.import my\\ dir/in`:  `dir/in`,
		`.import my\\\ dir/in`: `my\\\ dir/in`,
		`\ lead`:               `\ lead`,
		`a b`:                  `b`,
		`ab`:                   `ab`,
		`.import `:             ``,
		``:                     ``,
	} {
		if got := currentCompletionWord(text); got != want {
			t.Errorf("currentCompletionWord(%q) = %q, want %q", text, got, want)
		}
	}
}

// TestOpenQuotePrefixSkipsEscapedQuotes checks which quoted words are still
// open: an escaped quote or backslash inside double quotes does not close it,
// a backslash at the very end escapes nothing, and single quotes have no
// escapes.
func TestOpenQuotePrefixSkipsEscapedQuotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		word  string
		inner string
		open  bool
	}{
		{`"a\"b`, `a\"b`, true},
		{`"a\\"`, ``, false},
		{`"a\`, `a\`, true},
		{`"ab"`, ``, false},
		{`'a\'`, ``, false},
		{`'ab`, `ab`, true},
		{`ab`, ``, false},
		{``, ``, false},
	}
	for _, tt := range tests {
		_, inner, open := openQuotePrefix(tt.word)
		if open != tt.open || inner != tt.inner {
			t.Errorf("openQuotePrefix(%q) = (%q, %v), want (%q, %v)", tt.word, inner, open, tt.inner, tt.open)
		}
	}
}

// TestInjectTempKeywordAfterLeadingWhitespace checks that TEMP goes after the
// CREATE however the stored statement is indented, and that a statement that
// is temp already, or is not a CREATE, is left alone.
func TestInjectTempKeywordAfterLeadingWhitespace(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"CREATE TABLE t (a)":         "CREATE TEMP TABLE t (a)",
		" \t\r\nCREATE TABLE t (a)":  " \t\r\nCREATE TEMP TABLE t (a)",
		"create view v AS SELECT 1":  "create TEMP view v AS SELECT 1",
		"CREATE TEMPORARY TABLE t()": "CREATE TEMPORARY TABLE t()",
		"SELECT 1":                   "SELECT 1",
		"   ":                        "   ",
	} {
		if got := injectTempKeyword(in); got != want {
			t.Errorf("injectTempKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSplitTableQualifierOnlySplitsASchema checks that a name is split only
// at a real schema with something after the dot.
func TestSplitTableQualifierOnlySplitsASchema(t *testing.T) {
	t.Parallel()

	for in, want := range map[string][2]string{
		"main.user": {"main", "user"},
		"TEMP.t":    {"TEMP", "t"},
		"main.":     {"", "main."},
		".user":     {"", ".user"},
		"a.b":       {"", "a.b"},
		"user":      {"", "user"},
	} {
		schema, object := splitTableQualifier(in)
		if got := [2]string{schema, object}; got != want {
			t.Errorf("splitTableQualifier(%q) = %q, want %q", in, got, want)
		}
	}
}
