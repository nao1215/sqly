package shell

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

func shellQuickConfig() *quick.Config {
	return &quick.Config{
		MaxCount: 400,
		Rand:     rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
	}
}

// cmdToken is a single command-line argument generator biased toward the
// characters that matter for quoting: spaces, quotes, backslashes, tabs.
type cmdToken string

// Generate implements quick.Generator.
func (cmdToken) Generate(r *rand.Rand, _ int) reflect.Value {
	alphabet := []rune(`ab12 ` + "\t" + `"'\/.-_` + "\n")
	n := r.Intn(6)
	var b strings.Builder
	for range n {
		b.WriteRune(alphabet[r.Intn(len(alphabet))])
	}
	return reflect.ValueOf(cmdToken(b.String()))
}

// quoteForShell wraps a token in double quotes, escaping backslash and double
// quote, which is exactly what splitArgs understands inside double quotes.
func quoteForShell(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// TestSplitArgs_DoubleQuoteRoundTripProperty asserts the metamorphic relation
// splitArgs(quote(tokens)) == tokens. If a user quotes each argument, the parser
// must recover exactly those arguments regardless of spaces or quote characters
// inside them.
func TestSplitArgs_DoubleQuoteRoundTripProperty(t *testing.T) {
	property := func(tokens []cmdToken) bool {
		strs := make([]string, len(tokens))
		quoted := make([]string, len(tokens))
		for i, tok := range tokens {
			strs[i] = string(tok)
			quoted[i] = quoteForShell(string(tok))
		}

		got, err := splitArgs(strings.Join(quoted, " "))
		if err != nil {
			return false
		}
		// Empty input yields a nil slice; treat nil and empty as equal.
		if len(strs) == 0 {
			return len(got) == 0
		}
		return reflect.DeepEqual(got, strs)
	}
	if err := quick.Check(property, shellQuickConfig()); err != nil {
		t.Error(err)
	}
}

// pathName generates a filename biased toward the characters escapeCompletionPath
// must handle: spaces, tabs, quotes, and backslashes. Newline and carriage return
// are excluded because escapeCompletionPath intentionally does not encode them:
// splitArgs cannot decode an escaped newline.
type pathName string

// Generate implements quick.Generator.
func (pathName) Generate(r *rand.Rand, _ int) reflect.Value {
	alphabet := []rune(`ab12 ` + "\t" + `"'\.-_`)
	n := r.Intn(8) + 1 // never empty: a completion always names something
	var b strings.Builder
	for range n {
		b.WriteRune(alphabet[r.Intn(len(alphabet))])
	}
	return reflect.ValueOf(pathName(b.String()))
}

// TestEscapeCompletionPath_RoundTripProperty asserts the metamorphic relation
// splitArgs(".import " + escapeCompletionPath(name))[1] == name. An accepted
// completion is re-tokenized by exec, so escaping must let any entry name survive
// as a single argument.
func TestEscapeCompletionPath_RoundTripProperty(t *testing.T) {
	property := func(name pathName) bool {
		argv, err := splitArgs(".import " + escapeCompletionPath(string(name)))
		if err != nil {
			return false
		}
		return len(argv) == 2 && argv[1] == string(name)
	}
	if err := quick.Check(property, shellQuickConfig()); err != nil {
		t.Error(err)
	}
}

// TestUnescapeCompletionPath_InvertsEscapeProperty asserts the metamorphic
// relation unescapeCompletionPath(escapeCompletionPath(name)) == name, so the
// completion code can escape a name for display and decode it for filesystem
// lookups without drift.
func TestUnescapeCompletionPath_InvertsEscapeProperty(t *testing.T) {
	property := func(name pathName) bool {
		return unescapeCompletionPath(escapeCompletionPath(string(name))) == string(name)
	}
	if err := quick.Check(property, shellQuickConfig()); err != nil {
		t.Error(err)
	}
}

// TestSplitCompletionPrefix_SplitsAtRealSeparator asserts that an escaped space
// before a "/" does not fool the splitter into cutting at the escaping backslash:
// the base keeps the escaped directory and the partial is the entry fragment.
func TestSplitCompletionPrefix_SplitsAtRealSeparator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prefix      string
		wantBase    string
		wantPartial string
	}{
		{name: "escaped space then slash splits at the slash", prefix: `my\ dir/in`, wantBase: `my\ dir/`, wantPartial: "in"},
		{name: "escaped space without a slash is one partial", prefix: `my\ dir`, wantBase: "", wantPartial: `my\ dir`},
		{name: "plain nested path splits at the slash", prefix: "testdata/ac", wantBase: "testdata/", wantPartial: "ac"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, base, partial := splitCompletionPrefix(tt.prefix)
			if base != tt.wantBase || partial != tt.wantPartial {
				t.Errorf("splitCompletionPrefix(%q) = base %q, partial %q; want base %q, partial %q",
					tt.prefix, base, partial, tt.wantBase, tt.wantPartial)
			}
		})
	}
}

// TestSplitArgs_NeverPanicsProperty asserts splitArgs returns (slice,nil) or
// (nil,err) for arbitrary input without panicking. Robustness guard for the
// tokenizer against untrusted shell input.
func TestSplitArgs_NeverPanicsProperty(t *testing.T) {
	property := func(s string) bool {
		args, err := splitArgs(s)
		if err != nil {
			return args == nil
		}
		return true
	}
	if err := quick.Check(property, shellQuickConfig()); err != nil {
		t.Error(err)
	}
}

// TestTrimGaps_IdempotentProperty asserts trimGaps is idempotent: collapsing
// whitespace a second time changes nothing.
func TestTrimGaps_IdempotentProperty(t *testing.T) {
	property := func(s string) bool {
		once := trimGaps(s)
		return trimGaps(once) == once
	}
	if err := quick.Check(property, shellQuickConfig()); err != nil {
		t.Error(err)
	}
}
