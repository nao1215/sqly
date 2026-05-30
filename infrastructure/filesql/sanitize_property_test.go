package filesql

import (
	"math/rand"
	"regexp"
	"testing"
	"testing/quick"
)

// sanitizedPattern is the contract SanitizeForSQL output must satisfy: a
// non-empty identifier of word characters that does not start with a digit.
var sanitizedPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
