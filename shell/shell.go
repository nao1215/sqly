// Package shell is sqly-shell. shell control user input
// (it's SQL query or helper command) and request the usecase layer to process it.
package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/nao1215/gorky/str"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
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
	promptPrefix      string
	argument          *config.Arg
	config            *config.Config
	commands          CommandList
	csvInteractor     *usecase.CSVInteractor
	tsvInteractor     *usecase.TSVInteractor
	ltsvInteractor    *usecase.LTSVInteractor
	jsonInteractor    *usecase.JSONInteractor
	sqlite3Interactor *usecase.SQLite3Interactor
	historyInteractor *usecase.HistoryInteractor
}

// NewShell return *Shell.
func NewShell(arg *config.Arg, cfg *config.Config, cmds CommandList,
	csv *usecase.CSVInteractor, tsv *usecase.TSVInteractor, ltsv *usecase.LTSVInteractor, json *usecase.JSONInteractor,
	sqlite3 *usecase.SQLite3Interactor, history *usecase.HistoryInteractor) *Shell {
	return &Shell{
		Ctx:               context.Background(),
		promptPrefix:      "sqly> ",
		argument:          arg,
		config:            cfg,
		commands:          cmds,
		csvInteractor:     csv,
		tsvInteractor:     tsv,
		ltsvInteractor:    ltsv,
		jsonInteractor:    json,
		sqlite3Interactor: sqlite3,
		historyInteractor: history,
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
	// workaround
	// bug :https://github.com/c-bata/go-prompt/issues/228
	defer func() {
		rawModeOff := exec.Command("/bin/stty", "-raw", "echo")
		rawModeOff.Stdin = os.Stdin
		_ = rawModeOff.Run()
		rawModeOff.Wait()
	}()

	for {
		input, err := s.prompt(s.Ctx)
		if err != nil {
			return err
		}
		if err = s.exec(input); err != nil {
			if errors.Is(err, ErrExitSqly) {
				return nil // user input ".exit"
			}
			fmt.Fprintf(Stderr, "%v\n", err)
			continue
		}
	}
}

// init store CSV data to in-memory DB and create table for sqly history.
func (s *Shell) init() error {
	if err := s.historyInteractor.CreateTable(s.Ctx); err != nil {
		return fmt.Errorf("failed to create table for sqly history: %v", err)
	}
	if len(s.argument.FilePaths) == 0 {
		return nil
	}
	return s.commands.importCommand(s, s.argument.FilePaths)
}

// printWelcomeMessage print version and help information.
func (s *Shell) printWelcomeMessage() {
	fmt.Fprintf(Stdout, "%s %s (work in progress)\n", color.GreenString("sqly"), config.GetVersion())
	fmt.Fprintln(Stdout, "")
	fmt.Fprintln(Stdout, "enter \"SQL query\" or \"sqly command that beginning with a dot\".")
	fmt.Fprintf(Stdout, "%s print usage, %s exit sqly.\n", color.CyanString(".help"), color.CyanString(".exit"))
	fmt.Fprintln(Stdout, "")
}

// printPrompt print "sqly>" prompt and getting user input
func (s *Shell) prompt(ctx context.Context) (string, error) {
	histories, err := s.historyInteractor.List(ctx)
	if err != nil {
		return "", err
	}

	return prompt.Input(s.promptPrefix,
		s.completer,
		prompt.OptionTitle("sqly"),
		prompt.OptionPrefixTextColor(prompt.Cyan),
		prompt.OptionSuggestionBGColor(prompt.DarkBlue),
		prompt.OptionSuggestionTextColor(prompt.Black),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionSelectedSuggestionBGColor(prompt.Blue),
		prompt.OptionSelectedDescriptionBGColor(prompt.Cyan),
		prompt.OptionSelectedSuggestionTextColor(prompt.DarkRed),
		prompt.OptionSelectedDescriptionTextColor(prompt.DarkRed),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
		prompt.OptionHistory(histories.ToStringList())), nil
}

func (s *Shell) completer(d prompt.Document) []prompt.Suggest {
	suggest := []prompt.Suggest{
		{Text: "SELECT", Description: "SQL: get records from table"},
		{Text: "INSERT INTO", Description: "SQL: creates one or more new records in an existing table"},
		{Text: "UPDATE", Description: "SQL: update one or more records"},
		{Text: "AS", Description: "SQL: set alias name"},
		{Text: "FROM", Description: "SQL: specify the table"},
		{Text: "WHERE", Description: "SQL: search condition"},
		{Text: "GROUP BY", Description: "SQL: groping records"},
		{Text: "HAVING", Description: "SQL: extraction conditions for records after grouping"},
		{Text: "ORDER BY", Description: "SQL: sort result"},
		{Text: "VALUES", Description: "SQL: specify values to be inserted or updated"},
		{Text: "SET", Description: "SQL: specify values to be updated"},
		{Text: "DELETE FROM", Description: "SQL: specify tables to be deleted"},
		{Text: "IN", Description: "SQL: condition grouping"},
		{Text: "INNER JOIN", Description: "SQL: inner join tables"},
		{Text: "LIMIT", Description: "SQL: upper Limit of records"},
		{Text: "table", Description: "sqly command argument: table output format"},
		{Text: "markdown", Description: "sqly command argument: markdown table output format"},
		{Text: "csv", Description: "sqly command argument: csv output format"},
		{Text: "tsv", Description: "sqly command argument: tsv output format"},
		{Text: "ltsv", Description: "sqly command argument: ltsv output format"},
		{Text: "json", Description: "sqly command argument: json output format"},
	}

	for _, v := range s.commands {
		suggest = append(suggest, prompt.Suggest{
			Text:        v.name,
			Description: "sqly command: " + v.description,
		})
	}

	tableNames, err := s.sqlite3Interactor.TablesName(s.Ctx)
	if err != nil {
		return prompt.FilterHasPrefix(suggest, d.GetWordBeforeCursor(), true)
	}
	for _, v := range tableNames {
		suggest = append(suggest, prompt.Suggest{
			Text:        v.Name,
			Description: "table: " + v.Name,
		})

		table, err := s.sqlite3Interactor.Header(s.Ctx, v.Name)
		if err != nil {
			return prompt.FilterHasPrefix(suggest, d.GetWordBeforeCursor(), true)
		}
		for _, h := range table.Header {
			suggest = append(suggest, prompt.Suggest{
				Text:        h,
				Description: "header: " + h + " column in " + v.Name + " table",
			})
		}
	}

	return prompt.FilterHasPrefix(suggest, d.GetWordBeforeCursor(), true)
}

// exec execute sqly helper command or sql query.
func (s *Shell) exec(request string) error {
	req := strings.TrimSpace(request)
	argv := strings.Split(str.TrimGaps(req), " ")
	if argv[0] == "" {
		return nil // user only input enter, space tab
	}

	if err := s.recordUserRequest(s.Ctx, req); err != nil {
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
	table, affectedRows, err := s.sqlite3Interactor.ExecSQL(s.Ctx, req)
	if err != nil {
		return err
	}
	if table == nil {
		fmt.Printf("affected is %d row(s)\n", affectedRows)
		return nil
	}

	// use --sql option and user want to output table data to file.
	if s.argument.NeedsOutputToFile() {
		if err := dumpToFile(s, s.argument.Output.FilePath, table); err != nil {
			return err
		}
		fmt.Fprintf(Stdout, "Output sql result to %s (output mode=%s)\n",
			color.HiCyanString(s.argument.Output.FilePath), dumpMode(s.argument.Output.Mode))
		return nil
	}

	table.Print(os.Stdout, s.argument.Output.Mode)
	return nil
}

// recordUserRequest record user request in DB.
func (s *Shell) recordUserRequest(ctx context.Context, request string) error {
	histories, err := s.historyInteractor.List(ctx)
	if err != nil {
		return err
	}

	history := model.History{
		ID:      len(histories) + 1,
		Request: request,
	}

	if err := s.historyInteractor.Create(ctx, history); err != nil {
		return fmt.Errorf("failed to store user input history: %v", err)
	}
	return nil
}
