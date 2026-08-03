package shell

import (
	"context"
	"fmt"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// importModeCommand shows or changes how a ragged CSV/TSV row (one whose field
// count differs from the header) is imported. With no argument it reports the
// current policy and usage; with one argument (stop|skip|pad) it switches the
// policy used by subsequent .import commands.
func (c CommandList) importModeCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		// A missing policy name is a command error, not a no-op, so a batch script
		// that meant ".import-mode pad" fails visibly instead of exiting 0 in the
		// wrong mode. The current policy and the list ride on the error path, so an
		// interactive user still sees them (on stderr).
		return fmt.Errorf(".import-mode requires a policy name\n"+
			"[Usage]\n"+
			"  .import-mode POLICY   ※ current mode=%s\n"+
			"[Policy list]\n"+
			"  stop ※ abort the import when a row's field count differs from the header (default)\n"+
			"  skip ※ drop such rows and import the rest\n"+
			"  pad ※ pad short rows with empty values; reject long rows",
			s.state.importMode)
	}
	if len(argv) > 1 {
		return fmt.Errorf(".import-mode accepts a single policy name, got %d arguments", len(argv))
	}

	policy, err := model.ParseMalformedRowPolicy(argv[0])
	if err != nil {
		return err
	}
	// Selecting the policy already in effect is a no-op, not a failure: an error
	// here made a script fatal on a line that changed nothing, including the
	// natural combination of `--import-mode skip` with a script that restates it.
	// An unrecognized policy name is still rejected, by ParseMalformedRowPolicy
	// above.
	if policy == s.state.importMode {
		return nil
	}

	fmt.Fprintf(config.Stderr, "Change import mode from %s to %s\n", s.state.importMode, policy)
	s.state.importMode = policy
	s.usecases.importer.SetMalformedRowPolicy(policy)
	return nil
}
