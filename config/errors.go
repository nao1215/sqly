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
