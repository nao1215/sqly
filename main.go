// Package main is sqly entry point.
package main

import (
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
		fmt.Fprintf(os.Stderr, "%s: %v\n", "failed to initialize sqly shell", err)
		return 1
	}
	defer cleanup()

	if err := shell.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	return 0
}
