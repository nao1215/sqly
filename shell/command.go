package shell

import (
	"errors"
	"sort"
	"strings"
)

var (
	// ErrExitSqly is not error. developer must not print this error.
	ErrExitSqly = errors.New("this is not error. however, user want to exit sqly command")
)

// command is type of sqly helper command
type command struct {
	execute     func(s *Shell, argv []string) error
	name        string
	description string
}

// CommandList is sqly helper command list.
// key is command name, value is command function pointer and command description.
type CommandList map[string]command

// NewCommands return *CommandList that set sqly helper commands.
func NewCommands() CommandList {
	c := CommandList{}
	c[".dump"] = command{execute: c.dumpCommand, name: ".dump", description: "dump db table to file in a format according to output mode (default: csv)"}
	c[".exit"] = command{execute: c.exitCommand, name: ".exit", description: "exit sqly"}
	c[".header"] = command{execute: c.headerCommand, name: ".header", description: "print table header"}
	c[".help"] = command{execute: c.helpCommand, name: ".help", description: "print help message"}
	c[".import"] = command{execute: c.importCommand, name: ".import", description: "import file(s)"}
	c[".mode"] = command{execute: c.modeCommand, name: ".mode", description: "change output mode"}
	c[".tables"] = command{execute: c.tablesCommand, name: ".tables", description: "print tables"}
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
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
