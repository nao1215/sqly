// Package main is sqly entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
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

// run execute sqly command. This function do dependency injection
// and run the interactive shell. The exit code classifies the failure; see
// shell.ExitCode for what each one means.
//
// SIGINT and SIGTERM cancel the run's context rather than killing the process,
// so the deferred cleanup still removes the temp directories a download or a
// staged stdin dataset created. The interactive shell is unaffected: the prompt
// puts the terminal in raw mode, where Ctrl-C arrives as a keystroke and no
// signal is delivered at all.
func run(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Trapping a signal takes away the one guarantee the default handler gave:
	// that it always kills the process. Cancellation reaches every blocking read
	// sqly does itself, but not one inside a dependency or the kernel, and a CLI
	// that can swallow Ctrl-C is worse than one that exits untidily. Restoring the
	// default disposition after the first signal means a second one kills it
	// outright, which is the usual "press it again" contract.
	go func() {
		<-ctx.Done()
		stop()
	}()

	sqlyShell, cleanup, err := di.NewShell(args)
	if err != nil {
		fmt.Fprintln(config.Stderr, startupErrorMessage(err))
		return shell.ExitCode(err)
	}
	defer cleanup()

	if err := sqlyShell.Run(ctx); err != nil {
		fmt.Fprintf(config.Stderr, "%v\n", err)
		// A canceled context is reported as an interrupt whatever the failing
		// statement called it. A query that notices the cancellation first returns
		// a driver error that says nothing about a signal, so the signal's own
		// record of what happened is what decides.
		if ctx.Err() != nil {
			return shell.ExitInterrupt
		}
		return shell.ExitCode(err)
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
