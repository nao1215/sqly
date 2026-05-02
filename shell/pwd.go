package shell

import (
	"context"
	"fmt"
	"os"

	"github.com/nao1215/sqly/config"
)

// pwdCommand print current working directory.
func (c CommandList) pwdCommand(_ context.Context, _ *Shell, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Fprintln(config.Stdout, dir)
	return nil
}
