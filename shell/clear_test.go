package shell

import (
	"context"
	"testing"
)

// Test_clearCommand tests the clearCommand registration and metadata.
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
		if cmd.execute == nil {
			t.Error("Expected execute function to be set")
		}
	})

	t.Run("clear command executes without panic", func(t *testing.T) {
		t.Parallel()

		c := NewCommands()
		cmd := c[".clear"]

		// Call execute and verify it returns (may return error in CI, but shouldn't panic)
		err := cmd.execute(context.Background(), &Shell{}, []string{})
		// We don't assert err == nil because clear might fail in headless environments
		// The important part is it doesn't panic and returns a valid error type if it fails
		_ = err // Acknowledge we're not checking the error value
	})
}
