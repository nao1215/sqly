package config

import "errors"

// ErrEmptyArg is argument for NewArg() is empty
var ErrEmptyArg = errors.New("argument is empty")

// errEmptySheet is returned when --sheet is given an explicit empty value, which
// would otherwise be indistinguishable from the flag being absent.
var errEmptySheet = errors.New("--sheet requires a non-empty sheet name")
