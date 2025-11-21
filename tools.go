//go:build tools

package tools

import (
	_ "github.com/google/wire/cmd/wire" // https://github.com/google/wire/issues/299
	_ "go.uber.org/mock/gomock"
)
