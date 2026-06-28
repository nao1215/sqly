package shell

import (
	"context"
	"fmt"
)

// exitCommand return ErrExitSqly. The caller shall terminate the sqly command.
//
// Unexpected trailing arguments are rejected rather than ignored, so a typo
// like ".exit now" cannot silently terminate a batch run with status 0.
func (c CommandList) exitCommand(_ context.Context, _ *Shell, argv []string) error {
	if len(argv) > 0 {
		return fmt.Errorf(".exit takes no arguments, got %d", len(argv))
	}
	return ErrExitSqly
}
