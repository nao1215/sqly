package shell

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
)

var (
	// ErrExitSqly is not error. developer must not print this error.
	ErrExitSqly = errors.New("this is not error. however, user want to exit sqly command")
)

// command is type of sqly helper command
type command struct {
	execute     func(s *Shell) error
	description string
}

// CommandList is sqly helper command list.
// key is command name, value is command function pointer and command description.
type CommandList map[string]command

// NewCommands return *CommandList that set sqly helper commands.
func NewCommands() CommandList {
	c := CommandList{}
	c[".exit"] = command{execute: c.exitCommand, description: "exit sqly"}
	c[".help"] = command{execute: c.helpCommand, description: "print help message"}
	return c
}

// has return whether command list has command that key(command name)
func (c CommandList) has(key string) bool {
	_, ok := c[key]
	return ok
}

// hasPrefix returns whether s has dot prefix or not
func (c CommandList) hasPrefix(s string) bool {
	return strings.HasPrefix(s, ".")
}

// sortCommandNameKey returns an array of sorted keys (command names)
// to sort the command list map
func (c CommandList) sortCommandNameKey() []string {
	var keys []string
	for key := range c {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// exitCommand return ErrExitSqly. The caller shall terminate the sqly command.
func (c CommandList) exitCommand(s *Shell) error {
	return ErrExitSqly
}

// helpCommand print all sqly command and their description.
func (c CommandList) helpCommand(s *Shell) error {
	for _, cmdName := range c.sortCommandNameKey() {
		fmt.Fprintf(os.Stdout, "      %10s: %s\n", color.CyanString(cmdName), c[cmdName].description)
	}
	return nil
}
