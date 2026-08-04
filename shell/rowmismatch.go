package shell

import (
	"context"
	"fmt"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// rowMismatchCommand shows or changes how a CSV/TSV row whose field count
// differs from the header is imported. With no argument it reports the current
// policy and usage; with one argument (error|skip|pad) it switches the policy
// used by subsequent .import commands.
func (c CommandList) rowMismatchCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		// A missing policy name is a command error, not a no-op, so a batch script
		// that meant ".row-mismatch pad" fails visibly instead of exiting 0 under the
		// wrong policy. The current policy and the list ride on the error path, so an
		// interactive user still sees them (on stderr).
		return fmt.Errorf(".row-mismatch requires a policy name\n"+
			"[Usage]\n"+
			"  .row-mismatch POLICY   ※ applies to CSV/TSV only; current policy=%s\n"+
			"[Policy list]\n"+
			"  error ※ fail the import when a row's field count differs from the header (default)\n"+
			"  skip ※ drop such rows and import the rest\n"+
			"  pad ※ pad short rows with empty values; fail on long rows",
			s.state.rowMismatch)
	}
	if len(argv) > 1 {
		return fmt.Errorf(".row-mismatch accepts a single policy name, got %d arguments", len(argv))
	}

	policy, err := model.ParseRowMismatchPolicy(argv[0])
	if err != nil {
		return err
	}
	// Selecting the policy already in effect is a no-op, not a failure: an error
	// here made a script fatal on a line that changed nothing, including the
	// natural combination of `--row-mismatch skip` with a script that restates it.
	// An unrecognized policy name is still rejected, by ParseRowMismatchPolicy
	// above.
	if policy == s.state.rowMismatch {
		return nil
	}

	fmt.Fprintf(config.Stderr, "Change row-mismatch policy from %s to %s\n", s.state.rowMismatch, policy)
	s.state.rowMismatch = policy
	s.usecases.importer.SetRowMismatchPolicy(policy)
	return nil
}
