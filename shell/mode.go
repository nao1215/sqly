package shell

import (
	"context"
	"fmt"

	"github.com/nao1215/sqly/config"
)

// modeCommand change output mode.
func (c CommandList) modeCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		_, _ = fmt.Fprintln(config.Stdout, "[Usage]")
		_, _ = fmt.Fprintf(config.Stdout, "  .mode OUTPUT_MODE   ※ current mode=%s\n", s.state.mode.String())
		_, _ = fmt.Fprintln(config.Stdout, "[Output mode list]")
		_, _ = fmt.Fprintln(config.Stdout, "  table")
		_, _ = fmt.Fprintln(config.Stdout, "  markdown")
		_, _ = fmt.Fprintln(config.Stdout, "  csv")
		_, _ = fmt.Fprintln(config.Stdout, "  tsv")
		_, _ = fmt.Fprintln(config.Stdout, "  ltsv")
		_, _ = fmt.Fprintln(config.Stdout, "  excel ※ active only when executing .dump, otherwise same as csv mode")
		return nil
	}
	return s.state.mode.changeOutputModeIfNeeded(argv[0])
}
