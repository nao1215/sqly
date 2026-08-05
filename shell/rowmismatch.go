package shell

import (
	"context"
	"fmt"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// rowMismatchCommand shows or changes how a CSV/TSV row whose field count
// differs from the header is imported. With no argument it reports the current
// policy and usage; with one argument (error|skip|pad) it switches the policy
// used by subsequent .import commands.
func (c CommandList) rowMismatchCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		// Reporting the policy is the answer to the question ".row-mismatch" asks.
		// See sessionsetting.go for why this is not an error and why it is on
		// stderr.
		printSessionSetting(settingRowMismatch, s.state.rowMismatch.String(),
			strings.Join(model.RowMismatchPolicyNames, ", "))
		return nil
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
