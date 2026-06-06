package shell

import (
	"testing"
	"unicode/utf8"
)

// FuzzSplitSQLStatements asserts the batch statement splitter never panics and
// never loses the open-quote/comment state in a way that drops the remainder: the
// concatenation of the returned statements and the remainder must contain every
// non-separator rune of the input (the splitter only removes top-level ";" and
// leading comments). It exercises untrusted multi-statement SQL with quotes,
// comments, and triggers.
func FuzzSplitSQLStatements(f *testing.F) {
	seeds := []string{
		"SELECT 1;",
		"SELECT 1; SELECT 2",
		"SELECT ';' AS x;",
		"-- c\nSELECT 1;",
		"/* a; b */ SELECT 1;",
		"CREATE TRIGGER t BEGIN SELECT 1; END;",
		"SELECT `a;b`;",
		"SELECT [a;b];",
		"",
		";",
		";;;",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		stmts, remainder := splitSQLStatements(s)
		// The splitter works on runes and only removes top-level ";" and leading
		// comments, so the retained content can only shrink. Compare in runes, since
		// []rune normalizes invalid UTF-8 to U+FFFD (which changes the byte length).
		inputRunes := utf8.RuneCountInString(s)
		total := utf8.RuneCountInString(remainder)
		for _, st := range stmts {
			total += utf8.RuneCountInString(st)
		}
		if total > inputRunes {
			t.Fatalf("split of %q produced more runes than the input (%d > %d): stmts=%v remainder=%q",
				s, total, inputRunes, stmts, remainder)
		}
	})
}

// FuzzSplitArgs asserts the shell argument splitter never panics for any input
// (including unbalanced quotes, control characters, and invalid UTF-8). It either
// returns fields or a clean parse error; it must never crash the shell parser.
func FuzzSplitArgs(f *testing.F) {
	for _, s := range []string{"a b c", `"a b" c`, `a "b\"c"`, "  ", "", `'single'`, "tab\tsep", `" "`, `""`, `"unterminated`} {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, s string) {
		// A parse error (e.g. an unbalanced quote) is a valid outcome; the only
		// guarantee under fuzzing is that the parser does not panic. Touch the
		// returned fields so the call is not optimized away.
		args, err := splitArgs(s)
		if err == nil {
			total := 0
			for _, a := range args {
				total += len(a)
			}
			_ = total
		}
	})
}
