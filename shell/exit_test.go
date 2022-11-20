package shell

import (
	"errors"
	"testing"
)

func TestCommandList_exitCommand(t *testing.T) {
	t.Run("exit sqly shell", func(t *testing.T) {
		c := CommandList{}

		want := ErrExitSqly
		got := c.exitCommand(nil, []string{})
		if !errors.Is(got, want) {
			t.Errorf("mismatch got=%v, want=%v", got, want)
		}
	})
}
