package config

import (
	"os"

	"golang.org/x/term"
)

// IsInputFromTTY reports whether standard input is connected to an interactive
// terminal. The shell uses this to choose non-TTY batch mode: when stdin is
// piped or redirected, commands are read from stdin instead of the prompt.
func IsInputFromTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
