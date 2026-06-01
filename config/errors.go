package config

import "errors"

// ErrEmptyArg is argument for NewArg() is empty
var ErrEmptyArg = errors.New("argument is empty")

// errEmptySheet is returned when --sheet is given an explicit empty value, which
// would otherwise be indistinguishable from the flag being absent.
var errEmptySheet = errors.New("--sheet requires a non-empty sheet name")

// errInvalidStdinName is returned when --stdin-name is not a valid table
// identifier (empty, path-like, or containing characters that filesql would
// sanitize), which would otherwise stage odd files or leave the advertised
// table name unqueryable.
var errInvalidStdinName = errors.New("--stdin-name must be a valid table name: letters, digits, and underscores only, not starting with a digit")

// errEmptyOutput, errEmptySQLFile, errEmptySaveDir, and errEmptyStdin are
// returned when their flag is given an explicit empty value. For each flag the
// empty string is the "flag absent" sentinel, so accepting an explicit "" would
// silently behave like the flag was never passed instead of surfacing the
// malformed value. Ref #349, #350, #352, #353.
var (
	errEmptyOutput  = errors.New("--output requires a non-empty destination path")
	errEmptySQLFile = errors.New("--sql-file requires a non-empty file path")
	errEmptySaveDir = errors.New("--save-dir requires a non-empty directory path")
	errEmptyStdin   = errors.New("--stdin requires a non-empty dataset format (csv|tsv|ltsv|json|jsonl)")
)

// errStdinNameReserved is returned when --stdin-name is a SQLite keyword. Such a
// name is a valid identifier shape but is not queryable as a bare table name
// (e.g. "SELECT * FROM select" is a syntax error), so it is rejected up front
// instead of advertising an unusable table name. Ref #423.
var errStdinNameReserved = errors.New("--stdin-name is a SQLite keyword and is not queryable as a bare table name; choose another name")

// errStdinNameWithoutStdin and errInspectSampleWithoutInspect are returned when
// a dependent flag is set without the flag that gives it meaning, so the flag is
// not silently ignored. Ref #391, #392.
var (
	errStdinNameWithoutStdin       = errors.New("--stdin-name has no effect without --stdin FORMAT")
	errInspectSampleWithoutInspect = errors.New("--inspect-sample has no effect without --inspect")
)

// errForceWithoutSave is returned when --force is set without --save or
// --save-dir. --force only confirms the destructive in-place write-back, so it
// is meaningless on its own and is rejected instead of silently ignored. Ref #393.
var errForceWithoutSave = errors.New("--force has no effect without --save or --save-dir")
