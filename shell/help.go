package shell

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
)

// helpCommand print all sqly command and their description.
func (c CommandList) helpCommand(_ context.Context, _ *Shell, _ []string) error {
	for _, cmdName := range c.sortCommandNameKey() {
		fmt.Fprintf(config.Stdout, "%20s: %s\n",
			color.CyanString(cmdName), c[cmdName].description)
	}
	return nil
}
