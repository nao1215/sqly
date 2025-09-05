package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// lsCommand list files and directories.
// If there is no argument, list the files and directories in the current directory.
// If there is one argument, list the files and directories in the specified directory.
// If there are multiple arguments, return an error.
func (c CommandList) lsCommand(_ context.Context, _ *Shell, argv []string) error {
	path, err := func() (string, error) {
		if len(argv) == 0 {
			return ".", nil
		}
		if len(argv) > 1 {
			return "", errors.New("too many arguments")
		}
		return argv[0], nil
	}()
	if err != nil {
		return err
	}

	if err := func() error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("no such file or directory: %s", path)
		}

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(context.Background(), "cmd", "/c", "dir", "/q", path) //nolint:gosec // Controlled command for ls functionality
		} else {
			cmd = exec.CommandContext(context.Background(), "ls", "-l", path) //nolint:gosec // Controlled command for ls functionality
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}(); err != nil {
		return err
	}
	return nil
}
