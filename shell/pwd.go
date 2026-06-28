package shell

import (
	"context"
	"fmt"
	"os"

	"github.com/nao1215/sqly/config"
)

// pwdCommand print current working directory.
func (c CommandList) pwdCommand(_ context.Context, _ *Shell, argv []string) error {
	if len(argv) > 0 {
		return fmt.Errorf(".pwd takes no arguments, got %d", len(argv))
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Fprintln(config.Stdout, dir)
	return nil
}
