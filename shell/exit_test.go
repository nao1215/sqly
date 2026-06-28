package shell

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCommandListExitCommand(t *testing.T) {
	t.Run("exit sqly shell", func(t *testing.T) {
		c := CommandList{}

		want := ErrExitSqly
		got := c.exitCommand(context.Background(), nil, []string{})
		if !errors.Is(got, want) {
			t.Errorf("mismatch got=%v, want=%v", got, want)
		}
	})

	t.Run("rejects unexpected extra arguments instead of exiting", func(t *testing.T) {
		c := CommandList{}

		got := c.exitCommand(context.Background(), nil, []string{"extra"})
		if errors.Is(got, ErrExitSqly) {
			t.Fatal("extra argument must not trigger a clean exit")
		}
		if got == nil || !strings.Contains(got.Error(), ".exit") {
			t.Errorf("error %v should mention .exit", got)
		}
	})
}
