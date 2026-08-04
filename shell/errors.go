package shell

import "fmt"

// The failures below are the ones a caller — a test, or a future feature —
// might want to tell apart. Each is a type rather than a formatted string, so
// `errors.As` answers "was this a bad invocation or a bad script?" without
// matching on wording, and the wording stays free to improve.
//
// The user-facing text is still the error's own message: sqly prints what these
// say, and nothing about the type reaches stderr.

// invocationError is a combination of options that cannot mean anything: two
// flags that contradict, a flag with nothing to apply to, a mode that has no
// input. It is the CLI's fault-of-use category, decided before any file is read.
type invocationError struct{ Err error }

func (e *invocationError) Error() string { return e.Err.Error() }
func (e *invocationError) Unwrap() error { return e.Err }

// scriptError is a script sqly cannot run as written: an unparseable boundary, a
// helper command where only SQL is allowed. It is reported before the first
// statement executes.
type scriptError struct{ Err error }

func (e *scriptError) Error() string { return e.Err.Error() }
func (e *scriptError) Unwrap() error { return e.Err }

// scriptSourceError is a script sqly could not read: a --sql-file path that does
// not exist, a stdin stream that failed mid-read. It is separate from
// scriptError, which is a script that was read and cannot be run as written —
// the first is a problem with the file, the second with its contents.
type scriptSourceError struct{ Err error }

func (e *scriptSourceError) Error() string { return e.Err.Error() }
func (e *scriptSourceError) Unwrap() error { return e.Err }

// batchStopError ends a script at the statement that failed. The detail — which
// statement, which line, and what it said — is already on stderr by the time
// this is returned, so its own message stays the one-line summary and does not
// repeat it.
//
// It still wraps the cause. Nothing prints the wrapped error, but the exit code
// is decided from it: a `.save` that could not write is an output failure
// whether it ran at the prompt or as line 9 of a script, and a wrapper that
// replaced the cause with a fresh error made every one of them look the same.
type batchStopError struct{ Err error }

func (e *batchStopError) Error() string { return "batch stopped: statement failed" }
func (e *batchStopError) Unwrap() error { return e.Err }

// resultCountError is a run that produced a number of result sets the chosen
// destination or format cannot carry.
type resultCountError struct {
	// Produced is how many result sets the run made.
	Produced int
	Err      error
}

func (e *resultCountError) Error() string { return e.Err.Error() }
func (e *resultCountError) Unwrap() error { return e.Err }

// outputPathError is a destination sqly will not write to: a directory, a
// missing parent, an input file, an input-only format.
type outputPathError struct {
	// Path is the destination as the user wrote it.
	Path string
	Err  error
}

func (e *outputPathError) Error() string { return e.Err.Error() }
func (e *outputPathError) Unwrap() error { return e.Err }

// writeBackError is a save that cannot be planned: an unwritable source format,
// a destination collision, an input with no file behind it.
type writeBackError struct{ Err error }

func (e *writeBackError) Error() string { return e.Err.Error() }
func (e *writeBackError) Unwrap() error { return e.Err }

// fileOpError is a filesystem step of a write that failed, named by the step so
// a failure says which half of the write it happened in. The staged and commit
// phases mean different things to a caller: a staging failure changed nothing,
// while a commit failure may have.
type fileOpError struct {
	// Op is the step: "stage", "backup", "commit", "rollback", or "cleanup".
	Op string
	// Path is the file the step was working on.
	Path string
	Err  error
}

func (e *fileOpError) Error() string {
	return fmt.Sprintf("failed to %s %s: %v", e.Op, e.Path, e.Err)
}
func (e *fileOpError) Unwrap() error { return e.Err }

// Op values for fileOpError, named once so the phases cannot drift apart.
const (
	opStage    = "stage"
	opBackup   = "back up"
	opCommit   = "commit"
	opRollback = "roll back"
)
