package shell

import (
	"context"
	"fmt"

	"github.com/nao1215/sqly/config"
)

// modeCommand change output mode.
func (c CommandList) modeCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintf(config.Stdout, "  .mode OUTPUT_MODE   ※ current mode=%s\n", s.state.mode.PrintMode.String())
		fmt.Fprintln(config.Stdout, "[Output mode list]")
		fmt.Fprintln(config.Stdout, "  table")
		fmt.Fprintln(config.Stdout, "  markdown")
		fmt.Fprintln(config.Stdout, "  csv")
		fmt.Fprintln(config.Stdout, "  tsv")
		fmt.Fprintln(config.Stdout, "  ltsv")
		fmt.Fprintln(config.Stdout, "  json")
		fmt.Fprintln(config.Stdout, "  excel ※ active only when executing .dump, otherwise same as csv mode")
		return nil
	}
	return s.state.mode.changeOutputModeIfNeeded(argv[0])
}
