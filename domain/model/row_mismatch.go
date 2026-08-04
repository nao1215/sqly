package model

import "fmt"

// RowMismatchPolicy selects how a delimited (CSV/TSV) row whose field count
// differs from the header is handled during import. It mirrors the policy
// filesql applies, so the shell and CLI can offer the choice the user asked for
// without depending on the filesql type directly.
type RowMismatchPolicy int

const (
	// RowMismatchError aborts the import on the first row whose field count
	// differs from the header. It is the default so a corrupt or misaligned file
	// is not imported as partial or empty data without the user noticing.
	RowMismatchError RowMismatchPolicy = iota
	// RowMismatchSkip drops mismatched rows and imports the well-formed ones.
	RowMismatchSkip
	// RowMismatchPad keeps short rows by padding missing fields with empty
	// strings. A long row remains an error so no input data is discarded.
	RowMismatchPad
)

// Policy names accepted by the --row-mismatch flag and the .row-mismatch shell
// command, and printed by String().
const (
	rowMismatchErrorName = "error"
	rowMismatchSkipName  = "skip"
	rowMismatchPadName   = "pad"
)

// RowMismatchPolicyNames lists the accepted policy names in the order help and
// error messages present them, so the flag, the shell command, and the docs
// cannot drift into offering different sets.
var RowMismatchPolicyNames = []string{rowMismatchErrorName, rowMismatchSkipName, rowMismatchPadName}

// String returns the lowercase policy name used by the --row-mismatch flag and
// the .row-mismatch shell command.
func (p RowMismatchPolicy) String() string {
	switch p {
	case RowMismatchError:
		return rowMismatchErrorName
	case RowMismatchSkip:
		return rowMismatchSkipName
	case RowMismatchPad:
		return rowMismatchPadName
	default:
		return rowMismatchErrorName
	}
}

// ParseRowMismatchPolicy converts a policy name ("error", "skip", or "pad")
// into a RowMismatchPolicy. It rejects any other value so a mistyped flag or
// command argument fails loudly instead of silently defaulting.
func ParseRowMismatchPolicy(name string) (RowMismatchPolicy, error) {
	switch name {
	case rowMismatchErrorName:
		return RowMismatchError, nil
	case rowMismatchSkipName:
		return RowMismatchSkip, nil
	case rowMismatchPadName:
		return RowMismatchPad, nil
	default:
		return RowMismatchError, fmt.Errorf("invalid row-mismatch policy %q: want %s, %s, or %s",
			name, rowMismatchErrorName, rowMismatchSkipName, rowMismatchPadName)
	}
}
