package filesql

import (
	"math/rand"
	"regexp"
	"testing"
	"testing/quick"
)

// sanitizedPattern is the contract SanitizeForSQL output must satisfy: a
// non-empty identifier that does not start with a digit.
//
// Letters and digits are matched by Unicode category rather than by ASCII range,
// because that is what filesql does: a file named 売上.csv becomes the table
// 売上, and a rule that dropped it would make every non-Latin name collapse to
// the same fallback.
// \p{Nd} and not \p{N}: SanitizeForSQL keeps a digit when unicode.IsDigit says
// so, and that is decimal digits only. A pattern allowing every numeric
// category would accept output the sanitizer cannot produce, which is a
// contract that describes something looser than the code.
var sanitizedPattern = regexp.MustCompile(`^[\p{L}\p{M}_][\p{L}\p{M}\p{Nd}_]*$`)

func sanitizeQuickConfig() *quick.Config {
	return &quick.Config{
		MaxCount: 500,
		Rand:     rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
	}
}

// TestSanitizeForSQL_OutputContractProperty asserts that for ANY input the
// result is a valid SQL identifier (word chars, no leading digit, non-empty).
// This guards table-name generation against producing names SQLite would reject.
func TestSanitizeForSQL_OutputContractProperty(t *testing.T) {
	property := func(s string) bool {
		return sanitizedPattern.MatchString(SanitizeForSQL(s))
	}
	if err := quick.Check(property, sanitizeQuickConfig()); err != nil {
		t.Error(err)
	}
}

// TestSanitizeForSQL_IdempotentProperty asserts sanitizing an already-sanitized
// name is a no-op. Idempotence means re-importing a file whose table name was
// already derived stays stable.
func TestSanitizeForSQL_IdempotentProperty(t *testing.T) {
	property := func(s string) bool {
		once := SanitizeForSQL(s)
		return SanitizeForSQL(once) == once
	}
	if err := quick.Check(property, sanitizeQuickConfig()); err != nil {
		t.Error(err)
	}
}
