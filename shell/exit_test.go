package shell

import (
	"context"
	"errors"
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
}
