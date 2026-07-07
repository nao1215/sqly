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

// IsOutputToTTY reports whether standard output is connected to an interactive
// terminal. The interactive shell holds the terminal in raw mode across prompts,
// which disables the terminal's own LF-to-CRLF mapping; the shell uses this to
// decide whether command output needs explicit CRLF translation so results stay
// aligned. When stdout is piped or redirected it returns false and output is left
// untouched.
func IsOutputToTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
