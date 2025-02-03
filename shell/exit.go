package shell

import "context"

// exitCommand return ErrExitSqly. The caller shall terminate the sqly command.
func (c CommandList) exitCommand(_ context.Context, _ *Shell, _ []string) error {
	return ErrExitSqly
}
