package shell

import (
	"fmt"
	"log"

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
	argument          *config.Arg
	csvInteractor     *usecase.CSVInteractor
	sqlite3Interactor *usecase.SQLite3Interactor
}

// NewShell return *Shell.
func NewShell(arg *config.Arg, csv *usecase.CSVInteractor, sqlite3 *usecase.SQLite3Interactor) *Shell {
	return &Shell{
		argument:          arg,
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
	s.interactive()
	return nil
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
	fmt.Fprintf(Stdout, "%s %s\n", color.GreenString("sqly"), Version)
	fmt.Fprintf(Stdout, "enter %s for usage hints.\n", color.CyanString("\".help\""))
}

// interactive is interactive shell for sqly command.
// This function recieve user input (it's SQL query or helper command) and
// request the usecase layer to process it.
func (s *Shell) interactive() {
	tty, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()

	fmt.Fprintf(Stdout, "%s>>", color.GreenString("sqly"))
	input := ""
	for {
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
		}

		// Enter押下したかの判定と補完処理を追加する
		input += string(r)
		fmt.Fprintf(Stdout, "\r%s>>%s", color.GreenString("sqly"), input)
	}
}
