// Package main is sqly entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/di"
	"github.com/nao1215/sqly/shell"
)

// osExit is a variable that holds the os.Exit function.
// This variable is used to mock the os.Exit function in tests.
var osExit = os.Exit

// main is entry point for sqly command.
func main() {
	config.InitSQLite3()
	osExit(run(os.Args))
}

// signalTrap is the record of which signal stopped a run.
//
// The signal has to be kept rather than inferred, because by the time the run
// unwinds nothing left carries it: the context reports only that it was
// canceled, and a query that noticed first returns a driver error that never
// mentions a signal. Ctrl-C and a service manager's SIGTERM would then be
// indistinguishable, and they exit with different codes.
type signalTrap struct {
	mu       sync.Mutex
	received os.Signal
}

// record stores the first signal to arrive. Later ones change nothing: the
// second signal's job is to kill the process outright, not to renumber the exit.
func (t *signalTrap) record(sig os.Signal) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.received == nil {
		t.received = sig
	}
}

// signal returns the signal that stopped the run, or nil if none did.
func (t *signalTrap) signal() os.Signal {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.received
}

// run execute sqly command. This function do dependency injection
// and run the interactive shell. The exit code classifies the failure; see
// shell.ExitCode for what each one means.
//
// SIGINT and SIGTERM cancel the run's context rather than killing the process,
// so the deferred cleanup still removes the temp directories a download or a
// staged stdin dataset created. Which of the two arrived decides the exit code:
// 130 for SIGINT, 143 for SIGTERM. The interactive shell is unaffected by
// either: the prompt puts the terminal in raw mode, where Ctrl-C arrives as a
// keystroke and no signal is delivered at all.
func run(args []string) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trap := &signalTrap{}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	// Trapping a signal takes away the one guarantee the default handler gave:
	// that it always kills the process. Cancellation reaches every blocking read
	// sqly does itself, but not one inside a dependency or the kernel, and a CLI
	// that can swallow Ctrl-C is worse than one that exits untidily. Restoring the
	// default disposition after the first signal means a second one kills it
	// outright, which is the usual "press it again" contract.
	//
	// The signal is recorded before the cancellation is published, so a run that
	// sees a canceled context can always read back which signal canceled it.
	go func() {
		sig, ok := <-signals
		if !ok {
			return
		}
		trap.record(sig)
		signal.Stop(signals)
		cancel()
	}()

	sqlyShell, cleanup, err := di.NewShell(args)
	if err != nil {
		fmt.Fprintln(config.Stderr, startupErrorMessage(err))
		return shell.ExitCode(err)
	}
	defer cleanup()

	runErr := sqlyShell.Run(ctx)
	if runErr != nil {
		fmt.Fprintf(config.Stderr, "%v\n", runErr)
	}
	// A run a signal stopped exits with that signal's code whatever the failing
	// statement called the error — including when the statement managed to finish
	// and returned nothing at all, which is still a run that was cut short.
	if sig := trap.signal(); sig != nil {
		return shell.ExitCodeForSignal(sig)
	}
	if runErr != nil {
		return shell.ExitCode(runErr)
	}
	return shell.ExitOK
}

// startupErrorMessage renders the stderr line for an error returned by
// di.NewShell. A config.ArgError is a bad CLI invocation (unknown flag,
// conflicting flags, malformed value), so it is shown as-is; the "failed to
// initialize sqly shell" prefix is reserved for genuine startup failures
// (database, history file, working directory) so it does not misdirect a user
// whose command line was simply wrong.
func startupErrorMessage(err error) string {
	var argErr *config.ArgError
	if errors.As(err, &argErr) {
		return err.Error()
	}
	return "failed to initialize sqly shell: " + err.Error()
}
