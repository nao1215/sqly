package shell

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
)

// helpCommand print all sqly command and their description.
func (c CommandList) helpCommand(_ context.Context, _ *Shell, argv []string) error {
	if len(argv) > 0 {
		return fmt.Errorf(".help takes no arguments, got %d", len(argv))
	}
	for _, cmdName := range c.sortCommandNameKey() {
		fmt.Fprintf(config.Stdout, "%20s: %s\n",
			color.CyanString(cmdName), c[cmdName].description)
	}
	return nil
}
