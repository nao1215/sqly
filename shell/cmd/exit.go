package cmd

import "github.com/nao1215/sqly/shell"

// exitCommand return ErrExitSqly. The caller shall terminate the sqly command.
func (c CommandList) exitCommand(s *shell.Shell) error {
	return ErrExitSqly
}
