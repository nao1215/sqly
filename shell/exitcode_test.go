package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/nao1215/sqly/config"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "no error is success",
			err:  nil,
			want: ExitOK,
		},
		{
			name: "a bad command line is a usage error",
			err:  mustArgError(t),
			want: ExitUsage,
		},
		{
			name: "flags that cannot mean anything together are a usage error",
			err:  &invocationError{Err: errors.New("--output requires --sql or --sql-file")},
			want: ExitUsage,
		},
		{
			name: "a script sqly cannot parse is a usage error",
			err:  &scriptError{Err: errors.New("a helper command must start its own line")},
			want: ExitUsage,
		},
		{
			name: "a format that cannot carry the results is a usage error",
			err:  &resultCountError{Produced: 2, Err: errors.New("csv cannot separate two results")},
			want: ExitUsage,
		},
		{
			name: "an input that failed to import is an input error",
			err:  &partialImportError{succeeded: 0, failed: 1, summary: "no.csv: path does not exist"},
			want: ExitInput,
		},
		{
			name: "a destination sqly will not write to is an output error",
			err:  &outputPathError{Path: "out.csv", Err: errors.New("parent directory does not exist")},
			want: ExitOutput,
		},
		{
			name: "a save that cannot be planned is an output error",
			err:  &writeBackError{Err: errors.New("came from a remote URL")},
			want: ExitOutput,
		},
		{
			name: "a filesystem step of a write is an output error",
			err:  &fileOpError{Op: opCommit, Path: "out.csv", Err: errors.New("disk full")},
			want: ExitOutput,
		},
		{
			name: "an interrupted run is the shell interrupt code",
			err:  context.Canceled,
			want: ExitInterrupt,
		},
		{
			name: "a failed query is the general failure code",
			err:  errors.New("no such table: nope"),
			want: ExitFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// TestExitCode_ClassifiesThroughWrapping checks the classification survives the
// wrapping every error picks up on its way out of a run. A code decided by the
// outermost layer would report every failure inside a script as the same thing.
func TestExitCode_ClassifiesThroughWrapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "an import error wrapped by the statement that hit it",
			err:  fmt.Errorf("line 3: %w", &partialImportError{succeeded: 0, failed: 1, summary: "no.csv"}),
			want: ExitInput,
		},
		{
			name: "a write-back error wrapped by the save that planned it",
			err:  fmt.Errorf(".save: %w", &writeBackError{Err: errors.New("no source file")}),
			want: ExitOutput,
		},
		{
			name: "a cancellation wrapped by the query that was running",
			err:  fmt.Errorf("run query: %w", context.Canceled),
			want: ExitInterrupt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// TestExitCode_UsageBeatsTheRest pins the precedence that matters: an error
// carrying more than one classification is reported as the earliest stage it
// failed at, because that is the one the user has to fix first.
func TestExitCode_UsageBeatsTheRest(t *testing.T) {
	t.Parallel()

	err := &invocationError{Err: &writeBackError{Err: errors.New("no source file")}}
	if got := ExitCode(err); got != ExitUsage {
		t.Errorf("ExitCode(usage wrapping output) = %d, want %d", got, ExitUsage)
	}
}

// TestExitCodes_AreDistinct guards the contract itself: the codes exist to be
// told apart by a shell script, so two of them meaning the same number would
// silently merge two failure classes.
func TestExitCodes_AreDistinct(t *testing.T) {
	t.Parallel()

	codes := map[int]string{
		ExitOK:        "ExitOK",
		ExitFailure:   "ExitFailure",
		ExitUsage:     "ExitUsage",
		ExitInput:     "ExitInput",
		ExitOutput:    "ExitOutput",
		ExitInterrupt: "ExitInterrupt",
	}
	if len(codes) != 6 {
		t.Errorf("the six exit codes collapse to %d distinct values: %v", len(codes), codes)
	}
	for code := range codes {
		if code < 0 || code > 125 && code != ExitInterrupt {
			t.Errorf("%s = %d is outside the range a shell reports unambiguously", codes[code], code)
		}
	}
}

// mustArgError returns a real config.ArgError by parsing an invalid command
// line, so the test binds to the type the parser actually produces rather than
// to a hand-built stand-in.
func mustArgError(t *testing.T) error {
	t.Helper()
	_, err := config.NewArg([]string{"sqly", "--no-such-flag"})
	if err == nil {
		t.Fatal("parsing an unknown flag returned no error")
	}
	var argErr *config.ArgError
	if !errors.As(err, &argErr) {
		t.Fatalf("parsing an unknown flag returned %T, want *config.ArgError", err)
	}
	return err
}

// TestRun_StdinScriptReadObservesCancellation is the half of interrupt handling
// that a signal handler alone does not give you. main.go traps SIGINT and
// cancels the run's context instead of letting the default handler kill the
// process — which means every blocking read in the run has to notice, or
// trapping the signal turns "Ctrl-C quits" into "Ctrl-C does nothing".
//
// A stdin script read is the blocking one that matters: a pipe whose writer
// never closes (a harness, a FIFO with an idle writer) leaves io.ReadAll parked
// forever. It is driven here through an io.Pipe left open on purpose.
func TestRun_StdinScriptReadObservesCancellation(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// A pipe with a writer that is never closed and never writes: the reader
	// blocks until something else intervenes.
	reader, writer := io.Pipe()
	defer func() { _ = writer.Close() }()
	s.stdin = reader
	s.isTTY = func() bool { return false }

	backupOut, backupErr := config.Stdout, config.Stderr
	var out, errOut strings.Builder
	config.Stdout, config.Stderr = &out, &errOut
	defer func() { config.Stdout, config.Stderr = backupOut, backupErr }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	// Give Run time to reach the read, then interrupt it the way a signal would.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case runErr := <-done:
		if runErr == nil {
			t.Fatal("an interrupted run returned no error")
		}
		if got := ExitCode(runErr); got != ExitInterrupt {
			t.Errorf("ExitCode = %d, want %d (%v)", got, ExitInterrupt, runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was canceled; the stdin read ignores cancellation")
	}
}
