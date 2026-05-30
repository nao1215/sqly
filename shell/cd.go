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
	if len(argv) > 1 {
		return errors.New("too many arguments")
	}

	var target string
	if len(argv) == 1 {
		target = argv[0]
	} else {
		// Resolve the home directory cross-platform. os.UserHomeDir reads
		// %USERPROFILE% on Windows, where $HOME is usually unset.
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		target = home
	}

	if err := os.Chdir(target); err != nil {
		return err
	}
	// Store the normalized absolute path (via os.Getwd), not the raw argument,
	// so the prompt stays correct after relative moves (e.g. ".." or "sub").
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	s.state.cwd = cwd
	return nil
}
