// Package cmd define sqly helper commands
package cmd

import (
	"errors"
	"sort"
	"strings"

	"github.com/nao1215/sqly/shell"
)

var (
	// ErrExitSqly is not error. developer must not print this error.
	ErrExitSqly = errors.New("this is not error. however, user want to exit sqly command")
)

// command is type of sqly helper command
type command struct {
	execute     func(s *shell.Shell) error
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

// hasCmd return whether command list hasCmd command that key(command name)
func (c CommandList) hasCmd(key string) bool {
	_, ok := c[key]
	return ok
}

// hasCmdPrefix returns whether s has dot prefix or not
func (c CommandList) hasCmdPrefix(s string) bool {
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
