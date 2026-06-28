package shell

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/config"
)

func TestCommandList_pwdCommand(t *testing.T) {
	t.Run("print current working directory", func(t *testing.T) {
		c := CommandList{}

		oldStdout := config.Stdout
		b := bytes.NewBuffer(nil)
		config.Stdout = b
		t.Cleanup(func() {
			config.Stdout = oldStdout
		})

		want, err := filepath.Abs(".")
		if err != nil {
			t.Fatalf("filepath.Abs failed: %v", err)
		}
		t.Chdir(want)

		err = c.pwdCommand(context.Background(), nil, []string{})
		if err != nil {
			t.Errorf("mismatch got=%v, want=nil", err)
		}

		got := strings.ReplaceAll(b.String(), "\n", "")
		got = strings.ReplaceAll(got, "\r", "")
		if diff := cmp.Diff(got, want); diff != "" {
			t.Error(diff)
		}
	})

	t.Run("rejects unexpected extra arguments", func(t *testing.T) {
		c := CommandList{}

		err := c.pwdCommand(context.Background(), nil, []string{"extra"})
		if err == nil {
			t.Fatal("expected error for extra argument, got nil")
		}
		if !strings.Contains(err.Error(), ".pwd") {
			t.Errorf("error %q should mention .pwd", err.Error())
		}
	})
}
