package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// clearCommand clears the terminal screen.
func (c CommandList) clearCommand(_ context.Context, _ *Shell, _ []string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clear screen: %w", err)
	}

	return nil
}
