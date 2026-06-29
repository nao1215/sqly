// Package main is sqly entry point.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/di"
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
// and run the interactive shell.
// Returns 0 for success case, 1 for error case.
func run(args []string) int {
	shell, cleanup, err := di.NewShell(args)
	if err != nil {
		fmt.Fprintln(config.Stderr, startupErrorMessage(err))
		return 1
	}
	defer cleanup()

	if err := shell.Run(context.Background()); err != nil {
		fmt.Fprintf(config.Stderr, "%v\n", err)
		return 1
	}
	return 0
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
