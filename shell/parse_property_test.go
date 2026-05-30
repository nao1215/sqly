package shell

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/nao1215/sqly/domain/model"
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

// TestNormalizeDumpExt_Property asserts the normalized path always ends with the
// format extension and that normalization is idempotent.
func TestNormalizeDumpExt_Property(t *testing.T) {
	formats := []model.ExportFormat{
		model.ExportCSV, model.ExportTSV, model.ExportLTSV,
		model.ExportMarkdown, model.ExportExcel, model.ExportJSON, model.ExportNDJSON,
	}
	property := func(base string, idx uint8) bool {
		ef := formats[int(idx)%len(formats)]
		path := base + ".tmp"
		got := normalizeDumpExt(path, ef)
		if !strings.HasSuffix(got, ef.Extension()) {
			return false
		}
		return normalizeDumpExt(got, ef) == got
	}
	if err := quick.Check(property, shellQuickConfig()); err != nil {
		t.Error(err)
	}
}
