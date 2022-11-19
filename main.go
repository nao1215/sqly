package main

import (
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/sqly/di"
)

// main is entry point for sqly command.
func main() {
	os.Exit(run(os.Args))
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
