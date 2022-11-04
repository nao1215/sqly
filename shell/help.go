package shell

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

// helpCommand print all sqly command and their description.
func (c CommandList) helpCommand(s *Shell, argv []string) error {
	for _, cmdName := range c.sortCommandNameKey() {
		fmt.Fprintf(os.Stdout, "%20s: %s\n",
			color.CyanString(cmdName), c[cmdName].description)
	}
	return nil
}
