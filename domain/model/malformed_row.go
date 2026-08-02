package model

import "fmt"

// MalformedRowPolicy selects how a delimited (CSV/TSV) row whose field count
// differs from the header is handled during import. It mirrors the policy
// filesql applies, so the shell and CLI can offer the choice the user asked for
// without depending on the filesql type directly.
type MalformedRowPolicy int

const (
	// MalformedRowStop aborts the import with an error on the first ragged row.
	// It is the default so a corrupt or misaligned file is not imported as
	// partial or empty data without the user noticing.
	MalformedRowStop MalformedRowPolicy = iota
	// MalformedRowSkip drops ragged rows and imports the well-formed ones.
	MalformedRowSkip
	// MalformedRowPad keeps short rows by padding missing fields with empty
	// strings. A long row remains an error so no input data is discarded.
	MalformedRowPad
)

// Policy names accepted by the --import-mode flag and the .import-mode shell
// command, and printed by String().
const (
	malformedRowStopName = "stop"
	malformedRowSkipName = "skip"
	malformedRowPadName  = "pad"
)

// String returns the lowercase policy name used by the --import-mode flag and
// the .import-mode shell command.
func (p MalformedRowPolicy) String() string {
	switch p {
	case MalformedRowStop:
		return malformedRowStopName
	case MalformedRowSkip:
		return malformedRowSkipName
	case MalformedRowPad:
		return malformedRowPadName
	default:
		return malformedRowStopName
	}
}

// ParseMalformedRowPolicy converts a policy name ("stop", "skip", or "pad")
// into a MalformedRowPolicy. It rejects any other value so a mistyped flag or
// command argument fails loudly instead of silently defaulting.
func ParseMalformedRowPolicy(name string) (MalformedRowPolicy, error) {
	switch name {
	case malformedRowStopName:
		return MalformedRowStop, nil
	case malformedRowSkipName:
		return MalformedRowSkip, nil
	case malformedRowPadName:
		return MalformedRowPad, nil
	default:
		return MalformedRowStop, fmt.Errorf("invalid import mode %q: want stop, skip, or pad", name)
	}
}
