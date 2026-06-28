package shell

import (
	"context"
	"fmt"
)

// modeCommand change output mode.
func (c CommandList) modeCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		// A missing mode name is a command error, not a no-op: returning nil would
		// let a batch script that meant ".mode csv" continue in the wrong mode and
		// still exit 0. The current mode and the mode list ride on the error path,
		// so an interactive user still sees them (on stderr).
		return fmt.Errorf(".mode requires a mode name\n"+
			"[Usage]\n"+
			"  .mode OUTPUT_MODE   ※ current mode=%s\n"+
			"[Output mode list]\n"+
			"  table\n"+
			"  markdown\n"+
			"  csv\n"+
			"  tsv\n"+
			"  ltsv\n"+
			"  json\n"+
			"  ndjson\n"+
			"  json-typed ※ json output with native numbers, booleans, and nulls\n"+
			"  ndjson-typed ※ ndjson output with native numbers, booleans, and nulls\n"+
			"  excel ※ active only when executing .dump, otherwise same as csv mode\n"+
			"  parquet ※ active only when executing .dump, otherwise same as csv mode",
			s.state.mode.displayName())
	}
	if len(argv) > 1 {
		return fmt.Errorf(".mode accepts a single mode name, got %d arguments", len(argv))
	}
	return s.state.mode.changeOutputModeIfNeeded(argv[0])
}
