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
