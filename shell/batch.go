package shell

import (
	"bufio"
	"context"
	"errors"
	"fmt"

	"github.com/nao1215/sqly/config"
)

// maxBatchLineBytes caps a single batch input line, preventing unbounded memory
// growth on input without newlines.
const maxBatchLineBytes = 10 * 1024 * 1024

// runBatch executes SQL queries and helper commands read from stdin, one per
// line, until EOF. It is used when sqly runs without a TTY (piped stdin), where
// the interactive prompt cannot start.
//
// Each non-empty line is executed via exec, so quoting rules match the
// interactive shell. Errors are reported to stderr but do not stop processing;
// if any line failed, runBatch returns an error so the process exits non-zero.
// A line of ".exit" stops processing early with a success status, mirroring the
// interactive shell.
func (s *Shell) runBatch(ctx context.Context) error {
	scanner := bufio.NewScanner(s.stdin)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxBatchLineBytes)

	failed := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if err := s.exec(ctx, line); err != nil {
			if errors.Is(err, ErrExitSqly) {
				return nil // user input ".exit"
			}
			// Report the line number and content so a failing command is
			// identifiable when many lines are piped in.
			fmt.Fprintf(config.Stderr, "batch line %d failed: %q: %v\n", lineNo, line, err)
			failed = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read batch input: %w", err)
	}
	if failed {
		return errors.New("one or more batch commands failed")
	}
	return nil
}
