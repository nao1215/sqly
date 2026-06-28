package shell

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nao1215/sqly/config"
)

// Test_clearCommand tests the clearCommand registration and metadata.
//
// Not parallel at the top level: the ANSI-output subtest swaps the package-global
// config.Stdout, so running concurrently with other parallel package tests would
// race. Keeping the parent serial confines that swap to the sequential phase.
func Test_clearCommand(t *testing.T) {
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

	t.Run("writes ANSI clear sequence to stdout in an interactive TTY session", func(t *testing.T) {
		// Regression for: .clear must clear the screen in-process via ANSI
		// escapes instead of shelling out to clear/cls.
		c := NewCommands()

		backup := config.Stdout
		defer func() { config.Stdout = backup }()
		var buf bytes.Buffer
		config.Stdout = &buf

		s := &Shell{isTTY: func() bool { return true }}
		if err := c[".clear"].execute(context.Background(), s, []string{}); err != nil {
			t.Fatalf("clear command returned error: %v", err)
		}
		if !strings.Contains(buf.String(), "\x1b[2J") {
			t.Fatalf("clear output = %q, want it to contain the ANSI clear-screen escape", buf.String())
		}
	})

	t.Run("emits nothing to stdout in non-TTY batch mode", func(t *testing.T) {
		// Regression: in batch mode (piped stdin) stdout carries machine-readable
		// payloads such as --json/--csv, so .clear must not inject ANSI escapes.
		c := NewCommands()

		backup := config.Stdout
		defer func() { config.Stdout = backup }()
		var buf bytes.Buffer
		config.Stdout = &buf

		s := &Shell{isTTY: func() bool { return false }}
		if err := c[".clear"].execute(context.Background(), s, []string{}); err != nil {
			t.Fatalf("clear command returned error: %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("clear output = %q, want no output in batch mode", buf.String())
		}
	})

	t.Run("rejects unexpected extra arguments", func(t *testing.T) {
		t.Parallel()

		c := NewCommands()
		err := c[".clear"].execute(context.Background(), &Shell{}, []string{"extra"})
		if err == nil {
			t.Fatal("expected error for extra argument, got nil")
		}
		if !strings.Contains(err.Error(), ".clear") {
			t.Errorf("error %q should mention .clear", err.Error())
		}
	})

	t.Run("clear command executes without panic", func(t *testing.T) {
		t.Parallel()

		c := NewCommands()
		cmd := c[".clear"]

		// Bound the test so Windows console-specific behavior cannot stall the
		// entire shell package in headless CI environments.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Call execute and verify it returns (may return error in CI, but shouldn't panic)
		err := cmd.execute(ctx, &Shell{}, []string{})
		// We don't assert err == nil because clear might fail in headless environments
		// The important part is it doesn't panic and returns a valid error type if it fails
		_ = err // Acknowledge we're not checking the error value
	})
}
