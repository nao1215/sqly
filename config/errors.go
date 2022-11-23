package config

import "errors"

var (
	// ErrEmptyArg is argument for NewArg() is empty
	ErrEmptyArg = errors.New("argument is empty")
)
