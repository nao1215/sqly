// Package shell is sqly-shell. shell control user input
// (it's SQL query or helper command) and request the usecase layer to process it.
package shell

import (
	"errors"
	"fmt"
	"os"

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
	currentInput      string
	commands          CommandList
	interactive       *Interactive
	argument          *config.Arg
	csvInteractor     *usecase.CSVInteractor
	sqlite3Interactor *usecase.SQLite3Interactor
}

// NewShell return *Shell.
func NewShell(arg *config.Arg, cmds CommandList, interactive *Interactive,
	csv *usecase.CSVInteractor, sqlite3 *usecase.SQLite3Interactor) *Shell {
	return &Shell{
		argument:          arg,
		commands:          cmds,
		interactive:       interactive,
		csvInteractor:     csv,
		sqlite3Interactor: sqlite3,
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
// This function recieve user input (it's SQL query or helper command) and
// request the usecase layer to process it.
func (s *Shell) communicate() error {
	tty, err := tty.Open()
	if err != nil {
		return err
	}
	defer tty.Close()

	for {
		fmt.Fprintf(Stdout, "\r%s>>%s", color.GreenString("sqly"), s.interactive.currentInput)
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
				// TODO: add execute history
				case 'A':
					fmt.Println("ALLOW-UP")
				case 'B':
					fmt.Println("ALLOW-DOWN")
				// TODO: add completion
				case 'C':
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
	csv, err := s.csvInteractor.List(s.argument.FilePath)
	if err != nil {
		return err
	}

	table := csv.ToTable()
	if err := s.sqlite3Interactor.CreateTable(table); err != nil {
		return err
	}

	if err := s.sqlite3Interactor.Insert(table); err != nil {
		return err
	}
	return nil
}

// printWelcomeMessage print version and help information.
func (s *Shell) printWelcomeMessage() {
	fmt.Fprintf(Stdout, "%s %s (work in progress)\n", color.GreenString("sqly"), Version)
	fmt.Println("")
	fmt.Println("enter \"SQL query\" or \"sqly command that beginning with a dot\".")
	fmt.Fprintf(Stdout, "%s print usage, %s exit sqly.\n", color.CyanString(".help"), color.CyanString(".exit"))
	fmt.Println("")
}

// exec execute sqly helper command or sql query.
func (s *Shell) exec() error {
	defer s.interactive.resetUserInput()

	req := s.interactive.request()
	if s.commands.has(req) {
		return s.commands[req].execute()
	}

	if s.commands.hasPrefix(req) {
		return errors.New("no such sqly command: " + color.CyanString(req))
	}

	// Exec query here
	// Check if it is the correct query

	return nil
}
