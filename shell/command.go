package shell

import (
	"context"
	"errors"
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
	c[helpCommand] = command{execute: c.helpCommand, name: helpCommand, description: "print help message"}
	c[importCommand] = command{execute: c.importCommand, name: importCommand, description: "import file(s) and/or directory(ies)"}
	c[rowMismatchCommand] = command{execute: c.rowMismatchCommand, name: rowMismatchCommand, description: "handle a CSV/TSV row whose field count differs from the header (error|skip|pad)"}
	c[lsCommand] = command{execute: c.lsCommand, name: lsCommand, description: "print directory contents"}
	c[modeCommand] = command{execute: c.modeCommand, name: modeCommand, description: "change output mode"}
	c[tablesCommand] = command{execute: c.tablesCommand, name: tablesCommand, description: "print tables"}
	c[pwdCommand] = command{execute: c.pwdCommand, name: pwdCommand, description: "print current working directory"}
	c[schemaCommand] = command{execute: c.schemaCommand, name: schemaCommand, description: "print CREATE TABLE statement of a table"}
	c[describeCommand] = command{execute: c.describeCommand, name: describeCommand, description: "print column information of a table"}
	c[saveCommand] = command{execute: c.saveCommand, name: saveCommand, description: "write tables back to files: .save DIR (to a directory) or .save --in-place (overwrite sources)"}
	c[dialectCommand] = command{execute: c.dialectCommand, name: dialectCommand, description: "show or set the SQL dialect for queries (sqlite, mysql, postgresql, googlesql)"}
	c[editCommand] = command{execute: c.editCommand, name: editCommand, description: "edit the last statement in $VISUAL or $EDITOR and run what is saved"}
	return c
}

// hasCmd return whether command list hasCmd command that key(command name)
func (c CommandList) hasCmd(key string) bool {
	_, ok := c[key]
	return ok
}

// looksLikeCommand reports whether s was meant as a helper command: it starts
// with a dot. It is what tells "no such sqly command" apart from a SQL statement
// the engine should be given a chance to reject on its own terms.
func looksLikeCommand(s string) bool {
	return strings.HasPrefix(s, ".")
}
