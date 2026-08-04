package config

import "errors"

// ArgError marks an error from parsing or validating command-line arguments and
// flags, as opposed to a runtime startup failure (database, history file,
// working directory). The top-level command checks for it to choose user-facing
// wording: an ArgError is a bad invocation the user can fix on the command line,
// so it must not be reported as the interactive shell failing to start.
type ArgError struct {
	err error
}

// Error returns the underlying message unchanged so the wrapper is invisible to
// users; it only carries the classification.
func (e *ArgError) Error() string { return e.err.Error() }

// Unwrap exposes the wrapped error so errors.Is and errors.As keep matching the
// sentinel argument errors (for example ErrEmptyArg).
func (e *ArgError) Unwrap() error { return e.err }

// newArgError tags err as an ArgError. A nil err returns nil so callers can wrap
// a result unconditionally.
func newArgError(err error) error {
	if err == nil {
		return nil
	}
	return &ArgError{err: err}
}

// ErrEmptyArg is argument for NewArg() is empty
var ErrEmptyArg = errors.New("argument is empty")

// errInvalidStdinTable is returned when --stdin-table is not a valid table
// identifier (empty, path-like, or containing characters that filesql would
// sanitize), which would otherwise stage odd files or leave the advertised
// table name unqueryable.
var errInvalidStdinTable = errors.New("--stdin-table must be a valid table name: letters, digits, and underscores only, not starting with a digit")

// errEmptyOutput, errEmptySQLFile, and errEmptyStdinFormat are returned when
// their flag is given an explicit empty value. For each flag the
// empty string is the "flag absent" sentinel, so accepting an explicit "" would
// silently behave like the flag was never passed instead of surfacing the
// malformed value.
var (
	errEmptyQuery       = errors.New("--sql requires a non-empty SQL statement")
	errEmptyOutput      = errors.New("--output requires a non-empty destination path")
	errEmptySQLFile     = errors.New("--sql-file requires a non-empty file path")
	errEmptyScriptFile  = errors.New("--script-file requires a non-empty file path")
	errEmptyStdinFormat = errors.New("--stdin-format requires a non-empty format: csv, tsv, ltsv, json, or jsonl")
)

// errStdinTableReserved is returned when --stdin-table is a SQLite keyword. Such
// a name is a valid identifier shape but is not queryable as a bare table name
// (e.g. "SELECT * FROM select" is a syntax error), so it is rejected up front
// instead of advertising an unusable table name.
var errStdinTableReserved = errors.New("--stdin-table is a SQLite keyword and is not queryable as a bare table name; choose another name")

// errStdinTableWithoutFormat and errInspectSampleWithoutInspect are returned when
// a dependent flag is set without the flag that gives it meaning, so the flag is
// not silently ignored.
var (
	errStdinTableWithoutFormat     = errors.New("--stdin-table has no effect without --stdin-format FORMAT")
	errInspectSampleWithoutInspect = errors.New("--inspect-sample has no effect without --inspect")
)
