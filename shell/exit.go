package shell

// exitCommand return ErrExitSqly. The caller shall terminate the sqly command.
func (c CommandList) exitCommand(s *Shell, argv []string) error {
	return ErrExitSqly
}
