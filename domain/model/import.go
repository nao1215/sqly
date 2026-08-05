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
