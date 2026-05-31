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
