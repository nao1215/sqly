package shell

import (
	"context"
	"fmt"

	"github.com/nao1215/sqly/domain/model"
)

// modeCommand change output mode.
func (c CommandList) modeCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		// Reporting the mode is the answer to the question ".mode" asks. See
		// sessionsetting.go for why this is not an error and why it is on stderr.
		printSessionSetting(settingOutputMode, s.state.mode.String(), model.PrintModeNames())
		return nil
	}
	if len(argv) > 1 {
		return fmt.Errorf(".mode accepts a single mode name, got %d arguments", len(argv))
	}
	return s.state.mode.changeOutputModeIfNeeded(argv[0])
}
