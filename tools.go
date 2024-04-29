//go:build tools
// +build tools

package tools

// https://github.com/google/wire/issues/299
import (
	_ "github.com/google/wire/cmd/wire"
)
