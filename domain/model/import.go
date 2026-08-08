package model

// ImportError is an input that failed to load, with the input named.
//
// It exists so the path travels as a value rather than as text. Both the layer
// that loads a file and the layer that reports the failure want to name it, and
// while they both did, one failure arrived as "failed to import file rm.csv:
// load file \"rm.csv\": ...: failed to stream file rm.csv: ..." — the same path
// three times. The layer that reports reads Path to decide which input the user
// named, and Err to say what went wrong, so the path is written once.
type ImportError struct {
	// Path is the file handed to the loader. It is the staged copy for an input
	// that was downloaded or re-encoded, which is why the reporting layer maps it
	// back to what the user typed instead of printing it.
	Path string
	// Err is what went wrong reading it, with nothing about the path in it.
	Err error
}

func (e *ImportError) Error() string { return e.Path + ": " + e.Err.Error() }
func (e *ImportError) Unwrap() error { return e.Err }

// SkippedRows is how much of one table's input the row-mismatch policy dropped.
//
// A skip is what the user asked for with --row-mismatch skip, so it is not a
// failure. Saying nothing about it is: an import that dropped one ragged row
// and one that dropped most of the file produced the same output, and a
// write-back afterwards makes either one permanent.
type SkippedRows struct {
	// Table is the table that lost rows.
	Table string
	// Count is how many data rows were dropped.
	Count int
	// Total is how many data rows the input held, dropped ones included, so a
	// message can say "2 of 4" rather than a bare number.
	Total int
}
