package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/shell"
)

// helpCommand print all sqly command and their description.
func (c CommandList) helpCommand(s *shell.Shell) error {
	for _, cmdName := range c.sortCommandNameKey() {
		fmt.Fprintf(os.Stdout, "      %10s: %s\n", color.CyanString(cmdName), c[cmdName].description)
	}
	return nil
}
