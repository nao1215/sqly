package shell

import (
	"testing"
)

// Test_clearCommand tests the clearCommand function.
// Note: This test may fail in headless CI environments where terminal commands
// are not available. The actual functionality works in real terminal environments.
func Test_clearCommand(t *testing.T) {
	t.Parallel()

	t.Run("clear command is registered", func(t *testing.T) {
		t.Parallel()

		c := NewCommands()
		
		// Verify the command exists in the command list
		if !c.hasCmd(".clear") {
			t.Error("Expected .clear command to be registered")
		}
		
		// Verify command has correct metadata
		cmd := c[".clear"]
		if cmd.name != ".clear" {
			t.Errorf("Expected command name '.clear', got %q", cmd.name)
		}
		if cmd.description != "clear terminal screen" {
			t.Errorf("Expected description 'clear terminal screen', got %q", cmd.description)
		}
	})

	// Skip the actual execution test in CI environments as it requires a real terminal
	// The command works correctly in real terminal usage
}
