package sqlname

import "testing"

// TestSplitQualifierOnlySplitsASchema checks that a name is split only
// at a real schema with something after the dot.
func TestSplitQualifierOnlySplitsASchema(t *testing.T) {
	t.Parallel()

	for in, want := range map[string][2]string{
		"main.user": {"main", "user"},
		"TEMP.t":    {"TEMP", "t"},
		"main.":     {"", "main."},
		".user":     {"", ".user"},
		"a.b":       {"", "a.b"},
		"user":      {"", "user"},
	} {
		schema, object := SplitQualifier(in)
		if got := [2]string{schema, object}; got != want {
			t.Errorf("SplitQualifier(%q) = %q, want %q", in, got, want)
		}
	}
}
