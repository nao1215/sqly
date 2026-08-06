package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
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
			err:  &importFailedError{failed: 1, summary: "no.csv: path does not exist"},
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
			err:  fmt.Errorf("line 3: %w", &importFailedError{failed: 1, summary: "no.csv"}),
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
		ExitOK:         "ExitOK",
		ExitFailure:    "ExitFailure",
		ExitUsage:      "ExitUsage",
		ExitInput:      "ExitInput",
		ExitOutput:     "ExitOutput",
		ExitInterrupt:  "ExitInterrupt",
		ExitTerminated: "ExitTerminated",
	}
	if len(codes) != 7 {
		t.Errorf("the seven exit codes collapse to %d distinct values: %v", len(codes), codes)
	}
	signalCodes := map[int]bool{ExitInterrupt: true, ExitTerminated: true}
	for code := range codes {
		if code < 0 || code > 125 && !signalCodes[code] {
			t.Errorf("%s = %d is outside the range a shell reports unambiguously", codes[code], code)
		}
	}
}

// TestExitCodeForSignal pins the two signal codes to the convention a shell,
// a CI runner, and a service manager already use: 128 plus the signal number.
// A caller that reads 143 knows something took the run away; one that reads 130
// knows a person pressed Ctrl-C. Collapsing them onto one code, which is what
// sqly used to do, makes that undecidable.
func TestExitCodeForSignal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sig  os.Signal
		want int
	}{
		{
			name: "SIGINT is 128 plus SIGINT",
			sig:  os.Interrupt,
			want: 130,
		},
		{
			name: "SIGTERM is 128 plus SIGTERM",
			sig:  syscall.SIGTERM,
			want: 143,
		},
		{
			name: "a signal sqly does not trap still reports as a stopped run",
			sig:  syscall.Signal(0),
			want: ExitInterrupt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCodeForSignal(tt.sig); got != tt.want {
				t.Errorf("ExitCodeForSignal(%v) = %d, want %d", tt.sig, got, tt.want)
			}
		})
	}

	// The arithmetic itself, stated once, so a code edited to a nearby number
	// fails here rather than in whatever script depended on it.
	if ExitInterrupt != 128+int(syscall.SIGINT) {
		t.Errorf("ExitInterrupt = %d, want 128+SIGINT = %d", ExitInterrupt, 128+int(syscall.SIGINT))
	}
	if ExitTerminated != 128+int(syscall.SIGTERM) {
		t.Errorf("ExitTerminated = %d, want 128+SIGTERM = %d", ExitTerminated, 128+int(syscall.SIGTERM))
	}
}

// TestExitCode_DoesNotInventASignal is the other half of the contract. An error
// that reached the top level without a signal behind it must not be reported as
// one, and a deadline must not be reported as a stop anyone asked for: a
// download that timed out is an input failure, and calling it 130 would send a
// retry loop after the wrong thing.
func TestExitCode_DoesNotInventASignal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "a timed-out download is not a signal",
			err:  context.DeadlineExceeded,
		},
		{
			name: "a timed-out download wrapped by the import that hit it is not a signal",
			err:  fmt.Errorf("download: %w", context.DeadlineExceeded),
		},
		{
			name: "an ordinary query failure is not a signal",
			err:  errors.New("no such table: nope"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExitCode(tt.err)
			if got == ExitInterrupt || got == ExitTerminated {
				t.Errorf("ExitCode(%v) = %d, which is a signal code", tt.err, got)
			}
		})
	}

	// A cancellation with no signal behind it is a stopped run, but it is never
	// reported as SIGTERM: which signal arrived is main's record to keep, and
	// ExitCode has no way to know one did.
	if got := ExitCode(context.Canceled); got == ExitTerminated {
		t.Errorf("ExitCode(context.Canceled) = %d; a bare cancellation must not claim to be SIGTERM", got)
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

// TestDotCommandUsageErrorsAreUsageErrors pins one rule across every dot-command
// that validates its arguments: a command written wrong did not run, so it is a
// `2` and not the `1` that means a statement ran and failed.
//
// Only .import followed the rule, and its own comment claimed it was keeping
// malformed input "in the usage class with every other 'you typed it wrong'" —
// which the other nine were not. A wrapper reading the code to decide whether to
// retry could not tell a typo in a script from a query that failed on the data.
func TestDotCommandUsageErrorsAreUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		run  func(CommandList, *Shell, []string) error
	}{
		{name: ".import with no path", argv: nil,
			run: func(c CommandList, s *Shell, a []string) error { return c.importCommand(context.Background(), s, a) }},
		{name: ".header with no table", argv: nil,
			run: func(c CommandList, s *Shell, a []string) error { return c.headerCommand(context.Background(), s, a) }},
		{name: ".describe with no table", argv: nil,
			run: func(c CommandList, s *Shell, a []string) error { return c.describeCommand(context.Background(), s, a) }},
		{name: ".schema with no table", argv: nil,
			run: func(c CommandList, s *Shell, a []string) error { return c.schemaCommand(context.Background(), s, a) }},
		{name: ".dump with no destination", argv: []string{"t"},
			run: func(c CommandList, s *Shell, a []string) error { return c.dumpCommand(context.Background(), s, a) }},
		{name: ".mode with an unknown mode", argv: []string{"nope"},
			run: func(c CommandList, s *Shell, a []string) error { return c.modeCommand(context.Background(), s, a) }},
		{name: ".row-mismatch with an unknown policy", argv: []string{"nope"},
			run: func(c CommandList, s *Shell, a []string) error {
				return c.rowMismatchCommand(context.Background(), s, a)
			}},
		{name: ".ls with two paths", argv: []string{"a", "b"},
			run: func(c CommandList, s *Shell, a []string) error { return c.lsCommand(context.Background(), s, a) }},
		{name: ".cd with two paths", argv: []string{"a", "b"},
			run: func(c CommandList, s *Shell, a []string) error { return c.cdCommand(context.Background(), s, a) }},
		{name: ".save with no argument", argv: nil,
			run: func(c CommandList, s *Shell, a []string) error { return c.saveCommand(context.Background(), s, a) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell, cleanup, err := newShell(t, []string{"sqly"})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			backup := config.Stderr
			config.Stderr = &strings.Builder{}
			defer func() { config.Stderr = backup }()

			err = tt.run(shell.commands, shell, tt.argv)
			if err == nil {
				t.Fatal("the command accepted its arguments, want a usage error")
			}
			if got := ExitCode(err); got != ExitUsage {
				t.Errorf("ExitCode(%v) = %d, want %d (the invocation was not accepted)", err, got, ExitUsage)
			}
		})
	}
}
