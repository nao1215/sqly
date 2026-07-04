package shell

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// ErrExitSqly is not error. developer must not print this error.
var ErrExitSqly = errors.New("this is not error. however, user want to exit sqly command")

// command is type of sqly helper command
type command struct {
	execute     func(ctx context.Context, s *Shell, argv []string) error
	name        string
	description string
}

// CommandList is sqly helper command list.
// key is command name, value is command function pointer and command description.
type CommandList map[string]command

// NewCommands return *CommandList that set sqly helper commands.
func NewCommands() CommandList {
	c := CommandList{}
	c[cdCommand] = command{execute: c.cdCommand, name: cdCommand, description: "change directory"}
	c[clearCommand] = command{execute: c.clearCommand, name: clearCommand, description: "clear terminal screen"}
	c[dumpCommand] = command{execute: c.dumpCommand, name: dumpCommand, description: "dump db table to file in a format according to output mode (default: csv)"}
	c[exitCommand] = command{execute: c.exitCommand, name: exitCommand, description: "exit sqly"}
	c[headerCommand] = command{execute: c.headerCommand, name: headerCommand, description: "print table header"}
	c[helpCommand] = command{execute: c.helpCommand, name: helpCommand, description: "print help message"}
	c[importCommand] = command{execute: c.importCommand, name: importCommand, description: "import file(s) and/or directory(ies)"}
	c[importModeCommand] = command{execute: c.importModeCommand, name: importModeCommand, description: "show or set how a ragged CSV/TSV row is imported (stop|skip|fill)"}
	c[lsCommand] = command{execute: c.lsCommand, name: lsCommand, description: "print directory contents"}
	c[modeCommand] = command{execute: c.modeCommand, name: modeCommand, description: "change output mode"}
	c[tablesCommand] = command{execute: c.tablesCommand, name: tablesCommand, description: "print tables"}
	c[pwdCommand] = command{execute: c.pwdCommand, name: pwdCommand, description: "print current working directory"}
	c[schemaCommand] = command{execute: c.schemaCommand, name: schemaCommand, description: "print CREATE TABLE statement of a table"}
	c[describeCommand] = command{execute: c.describeCommand, name: describeCommand, description: "print column information of a table"}
	c[saveCommand] = command{execute: c.saveCommand, name: saveCommand, description: "write tables back to files: .save DIR (to a directory) or .save --force (overwrite sources)"}
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
