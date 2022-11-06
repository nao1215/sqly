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
	argument          *config.Arg
	config            *config.Config
	commands          CommandList
	interactive       *Interactive
	csvInteractor     *usecase.CSVInteractor
	sqlite3Interactor *usecase.SQLite3Interactor
}

// NewShell return *Shell.
func NewShell(arg *config.Arg, cfg *config.Config, cmds CommandList, interactive *Interactive,
	csv *usecase.CSVInteractor, sqlite3 *usecase.SQLite3Interactor) *Shell {
	return &Shell{
		Ctx:               context.Background(),
		argument:          arg,
		config:            cfg,
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

	if s.argument.VersionFlag {
		s.argument.Version()
		return nil
	}

	if err := s.init(); err != nil {
		return err
	}

	if s.argument.Query != "" {
		return s.execSQL(s.argument.Query)
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
			s.interactive.deleteChar()
		case runeEnter:
			fmt.Println("") // Not delete it.
			if err = s.exec(); err != nil {
				if errors.Is(err, ErrExitSqly) {
					return nil // user input ".exit"
				}
				fmt.Fprintf(Stderr, "%v\n", err)
				continue
			}
		case runeTabKey:
			// TODO: add completion
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
					s.interactive.cursorRight()
				case 'D':
					s.interactive.cursorLeft()
				}
			}
		default:
			s.interactive.append(r)
		}
	}
}

// init store CSV data to in-memory DB and create table for sqly history.
func (s *Shell) init() error {
	if err := s.interactive.initialize(s.Ctx); err != nil {
		return err
	}
	if len(s.argument.FilePaths) == 0 {
		return nil
	}
	return s.commands[".import"].execute(s, s.argument.FilePaths)
}

// printWelcomeMessage print version and help information.
func (s *Shell) printWelcomeMessage() {
	fmt.Fprintf(Stdout, "%s %s (work in progress)\n", color.GreenString("sqly"), config.GetVersion())
	fmt.Fprintln(Stdout, "")
	fmt.Fprintln(Stdout, "enter \"SQL query\" or \"sqly command that beginning with a dot\".")
	fmt.Fprintf(Stdout, "%s print usage, %s exit sqly.\n", color.CyanString(".help"), color.CyanString(".exit"))
	fmt.Fprintln(Stdout, "")
}

// exec execute sqly helper command or sql query.
func (s *Shell) exec() error {
	req := s.interactive.request()
	argv := strings.Split(trimWordGaps(req), " ")
	if argv[0] == "" {
		return nil // user only input enter, space tab
	}

	if err := s.interactive.recordUserRequest(s.Ctx); err != nil {
		return err
	}

	if s.commands.hasCmd(argv[0]) {
		return s.commands[argv[0]].execute(s, argv[1:])
	}

	if s.commands.hasCmdPrefix(req) {
		return errors.New("no such sqly command: " + color.CyanString(req))
	}

	if err := s.execSQL(req); err != nil {
		return err
	}
	return nil
}

func (s *Shell) execSQL(req string) error {
	req = strings.TrimRight(req, ";")
	table, affectedRows, err := s.sqlite3Interactor.ExecSQL(s.Ctx, req, s.argument.Output.Mode)
	if err != nil {
		return err
	}
	if table == nil {
		fmt.Printf("affected is %d row(s)\n", affectedRows)
		return nil
	}

	// use --sql option and user want to output table data to file.
	if s.argument.NeedsOutputToFile() {
		// I am having difficulty in designing. Therefore, I do not save files in TABLE format.
		// Also, only Windows users need to save files with the --output option, and Linux users (it's me)
		// can save data in table format with redirection.
		if err := s.csvInteractor.Dump(s.argument.Output.FilePath, table); err != nil {
			return err
		}
		fmt.Fprintf(Stdout, "Output sql result to %s\n", color.HiCyanString(s.argument.Output.FilePath))
		return nil
	}

	table.Print(os.Stdout, s.argument.Output.Mode)
	return nil
}
