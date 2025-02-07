package shell

import (
	"context"
	"errors"
	"os"
)

// cdCommand change directory.
// If there is no argument, change to the home directory.
// If there is one argument, change to the specified directory.
// If there are multiple arguments, return an error.
func (c CommandList) cdCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		home := os.Getenv("HOME")
		if err := os.Chdir(home); err != nil {
			return err
		}
		s.state.cwd = home
		return nil
	}
	if len(argv) > 1 {
		return errors.New("too many arguments")
	}

	if err := os.Chdir(argv[0]); err != nil {
		return err
	}
	s.state.cwd = argv[0]
	return nil
}
