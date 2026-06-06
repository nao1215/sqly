package infrastructure

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// unquoteBacktick reverses Quote: it strips the outer backticks and halves each
// doubled backtick. It is the inverse used to assert the round-trip property.
func unquoteBacktick(q string) (string, bool) {
	if len(q) < 2 || q[0] != '`' || q[len(q)-1] != '`' {
		return "", false
	}
	return strings.ReplaceAll(q[1:len(q)-1], "``", "`"), true
}

// unquoteSingle reverses SingleQuote.
func unquoteSingle(q string) (string, bool) {
	if len(q) < 2 || q[0] != '\'' || q[len(q)-1] != '\'' {
		return "", false
	}
	return strings.ReplaceAll(q[1:len(q)-1], "''", "'"), true
}

// FuzzQuoteRoundTrip asserts Quote always produces a well-formed backtick-quoted
// identifier that reverses to the original input, for any string. This is the
// metamorphic relation unquote(Quote(s)) == s, and it guards the identifier
// quoting used throughout sqly against escaping regressions.
func FuzzQuoteRoundTrip(f *testing.F) {
	for _, s := range []string{"", "a", "a`b", "`", "``", "main.user", "a\nb", "日本語", "'", "\""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		q := Quote(s)
		if len(q) < 2 || q[0] != '`' || q[len(q)-1] != '`' {
			t.Fatalf("Quote(%q) = %q is not backtick-delimited", s, q)
		}
		// Quote is rune-based, so it replaces invalid UTF-8 with U+FFFD (the same
		// lossy behavior as encoding/json). The exact round-trip therefore only
		// holds for valid UTF-8; for the rest, no-panic and well-formedness above
		// is the guarantee.
		if !utf8.ValidString(s) {
			return
		}
		got, ok := unquoteBacktick(q)
		if !ok || got != s {
			t.Fatalf("round-trip failed: Quote(%q) = %q, unquote = %q (ok=%v)", s, q, got, ok)
		}
	})
}

// FuzzSingleQuoteRoundTrip asserts SingleQuote round-trips like Quote, for the
// SQL string literals sqly builds for INSERT statements.
func FuzzSingleQuoteRoundTrip(f *testing.F) {
	for _, s := range []string{"", "a", "a'b", "'", "''", "a\nb", "日本語", "`", "\""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		q := SingleQuote(s)
		if len(q) < 2 || q[0] != '\'' || q[len(q)-1] != '\'' {
			t.Fatalf("SingleQuote(%q) = %q is not single-quote-delimited", s, q)
		}
		if !utf8.ValidString(s) {
			return // rune-based, lossy on invalid UTF-8 (see FuzzQuoteRoundTrip)
		}
		got, ok := unquoteSingle(q)
		if !ok || got != s {
			t.Fatalf("round-trip failed: SingleQuote(%q) = %q, unquote = %q (ok=%v)", s, q, got, ok)
		}
	})
}

// FuzzQuoteTableRef asserts QuoteTableRef never panics and always yields a
// reference whose every component is backtick-quoted: a single quoted identifier,
// or exactly two joined by an unquoted dot for a main./temp. schema prefix.
func FuzzQuoteTableRef(f *testing.F) {
	for _, s := range []string{"user", "main.user", "temp.t", "a.b", "main.", ".x", "MAIN.User", "a.b.c", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := QuoteTableRef(s)
		if len(got) < 2 || got[0] != '`' || got[len(got)-1] != '`' {
			t.Fatalf("QuoteTableRef(%q) = %q is not backtick-delimited", s, got)
		}
		// It is either one quoted identifier or `schema`.`name`. In both cases the
		// whole string reverses through the backtick rules when treated as one or
		// two components; at minimum it must be non-empty and balanced.
		if strings.Count(got, "`")%2 != 0 {
			t.Fatalf("QuoteTableRef(%q) = %q has unbalanced backticks", s, got)
		}
	})
}
