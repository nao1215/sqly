// Package shell is sqly-shell. shell control user input
// (it's SQL query or helper command) and request the usecase layer to process it.
package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-tty"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/usecase"
)

var (
	// Version is sqly command version. Version value is assigned by LDFLAGS.
	Version string
	// Stdout is new instance of Writer which handles escape sequence for stdout.
	Stdout = colorable.NewColorableStdout()
	// Stderr is new instance of Writer which handles escape sequence for stderr.
	Stderr = colorable.NewColorableStderr()
)

// Shell is main class of the sqly command.
// Shell is the interface to the user and requests processing from the usecase layer.
type Shell struct {
	Ctx               context.Context
	commands          CommandList
	interactive       *Interactive
	argument          *config.Arg
	CsvInteractor     *usecase.CSVInteractor
	Sqlite3Interactor *usecase.SQLite3Interactor
}

// NewShell return *Shell.
func NewShell(arg *config.Arg, cmds CommandList, interactive *Interactive,
	csv *usecase.CSVInteractor, sqlite3 *usecase.SQLite3Interactor) *Shell {
	return &Shell{
		Ctx:               context.Background(),
		argument:          arg,
		commands:          cmds,
		interactive:       interactive,
		CsvInteractor:     csv,
		Sqlite3Interactor: sqlite3,
	}
}

// Run start sqly shell.
// After successful initialization, start the interactive shell.
func (s *Shell) Run() error {
	if s.argument.HelpFlag {
		s.argument.Usage()
		return nil
	}

	if err := s.init(); err != nil {
		return err
	}
	s.printWelcomeMessage()
	return s.communicate()
}

// communicate is interactive command prompt for sqly.
// This function receive user input (it's SQL query or helper command) and
// request the usecase layer to process it.
func (s *Shell) communicate() error {
	tty, err := tty.Open()
	if err != nil {
		return err
	}
	defer tty.Close()

	for {
		s.interactive.clearLine()
		s.interactive.printPrompt()

		r, err := tty.ReadRune()
		if err != nil {
			return err
		}

		switch r {
		case runeBackSpace, runeDelete:
			s.interactive.deleteLastInput()
		case runeEnter:
			fmt.Println("")
			if err := s.exec(); err != nil {
				if errors.Is(err, ErrExitSqly) {
					return nil // user input ".exit"
				}
				fmt.Fprintln(os.Stderr, err)
			}
		case runeTabKey:
			// TODO: completion
			fmt.Println("Tab")
			continue
		case runeEscapeKey:
			r, err = tty.ReadRune()
			if err == nil && r == 0x5b {
				r, err = tty.ReadRune()
				if err != nil {
					return err
				}
				switch r {
				case 'A':
					s.interactive.olderInput()
				case 'B':
					s.interactive.newerInput()
				case 'C':
					// TODO: add completion
					fmt.Println("ALLOW-RIGHT")
				case 'D':
					fmt.Println("ALLOW-LEFT")
				}
			}
		default:
			s.interactive.append(r)
		}
	}
}

// init store CSV data to DB.
func (s *Shell) init() error {
	if len(s.argument.FilePaths) == 0 {
		return nil
	}
	if err := s.commands[".import"].execute(s, s.argument.FilePaths); err != nil {
		return err
	}
	return nil
}

// printWelcomeMessage print version and help information.
func (s *Shell) printWelcomeMessage() {
	fmt.Fprintf(Stdout, "%s %s (work in progress)\n", color.GreenString("sqly"), Version)
	fmt.Fprintln(Stdout, "")
	fmt.Fprintln(Stdout, "enter \"SQL query\" or \"sqly command that beginning with a dot\".")
	fmt.Fprintf(Stdout, "%s print usage, %s exit sqly.\n", color.CyanString(".help"), color.CyanString(".exit"))
	fmt.Fprintln(Stdout, "")
}

// exec execute sqly helper command or sql query.
func (s *Shell) exec() error {
	defer s.interactive.history.alloc()

	req := s.interactive.request()
	argv := strings.Split(trimWordGaps(req), " ")
	if s.commands.hasCmd(argv[0]) {
		return s.commands[argv[0]].execute(s, argv[1:])
	}

	if s.commands.hasCmdPrefix(req) {
		return errors.New("no such sqly command: " + color.CyanString(req))
	}

	// Exec query here
	// Check if it is the correct query

	return nil
}
