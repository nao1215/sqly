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
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
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
	argument *config.Arg
	config   *config.Config
	commands CommandList
	usecases Usecases
	state    *state
}

// NewShell return *Shell.
func NewShell(
	arg *config.Arg,
	cfg *config.Config,
	cmds CommandList,
	usecases Usecases,
) (*Shell, error) {
	state, err := newState(arg)
	if err != nil {
		return nil, err
	}
	return &Shell{
		argument: arg,
		config:   cfg,
		commands: cmds,
		usecases: usecases,
		state:    state,
	}, nil
}

// Run start sqly shell.
// After successful initialization, start the interactive shell.
func (s *Shell) Run(ctx context.Context) error {
	if s.argument.HelpFlag {
		fmt.Fprintf(config.Stdout, "%s", s.argument.Usage)
		return nil
	}

	if s.argument.VersionFlag {
		s.argument.Version()
		return nil
	}

	if err := s.init(ctx); err != nil {
		return err
	}

	if s.argument.Query != "" {
		return s.execSQL(ctx, s.argument.Query)
	}

	// Start shell
	s.printWelcomeMessage()
	return s.communicate(ctx)
}

// communicate is interactive command prompt for sqly.
// This function receive user input (it's SQL query or helper command) and
// request the usecase layer to process it.
func (s *Shell) communicate(ctx context.Context) error {
	// workaround
	// bug :https://github.com/c-bata/go-prompt/issues/228
	defer func() {
		rawModeOff := exec.Command("/bin/stty", "-raw", "echo")
		rawModeOff.Stdin = os.Stdin
		if err := rawModeOff.Run(); err != nil {
			fmt.Fprintf(Stderr, "failed to turn off raw mode: %v\n", err)
		}
		if err := rawModeOff.Wait(); err != nil {
			fmt.Fprintf(Stderr, "failed to wait raw mode off: %v\n", err)
		}
	}()

	for {
		input, err := s.prompt(ctx)
		if err != nil {
			return err
		}
		if err = s.exec(ctx, input); err != nil {
			if errors.Is(err, ErrExitSqly) {
				return nil // user input ".exit"
			}
			fmt.Fprintf(Stderr, "%v\n", err)
			continue
		}
	}
}

// init store CSV data to in-memory DB and create table for sqly history.
func (s *Shell) init(ctx context.Context) error {
	if err := s.usecases.history.CreateTable(ctx); err != nil {
		return fmt.Errorf("failed to create table for sqly history: %w", err)
	}
	if len(s.argument.FilePaths) == 0 {
		return nil
	}
	return s.commands.importCommand(ctx, s, s.argument.FilePaths)
}

// printWelcomeMessage print version and help information.
func (s *Shell) printWelcomeMessage() {
	fmt.Fprintf(config.Stdout, "%s %s\n", color.GreenString("sqly"), config.GetVersion())
	fmt.Fprintln(config.Stdout, "")
	fmt.Fprintln(config.Stdout, "enter \"SQL query\" or \"sqly command that begins with a dot\".")
	fmt.Fprintf(config.Stdout, "%s print usage, %s exit sqly.\n", color.CyanString(".help"), color.CyanString(".exit"))
	fmt.Fprintln(config.Stdout, "")
}

// printPrompt print "sqly>" prompt and getting user input
func (s *Shell) prompt(ctx context.Context) (string, error) {
	histories, err := s.usecases.history.List(ctx)
	if err != nil {
		return "", err
	}

	return prompt.Input(
		func() string {
			return fmt.Sprintf("sqly:%s(%s)$ ", s.state.shortCWD(), s.state.mode.String())
		}(),
		func(d prompt.Document) []prompt.Suggest {
			return s.completer(ctx, d)
		},
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

// completer return prompt.Suggest for auto-completion.
func (s *Shell) completer(ctx context.Context, d prompt.Document) []prompt.Suggest {
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
		{Text: "LIKE", Description: "SQL: use wildcards to perform pattern matching"},
		{Text: "GLOB", Description: "SQL: match only text values against a pattern using wildcard"},
		{Text: "BETWEEN", Description: "SQL: selects values within a given range"},
		{Text: "IS NULL", Description: "SQL: selects null values"},
		{Text: "DISTINCT", Description: "SQL: exclude duplicate values"},
		{Text: "INNER JOIN", Description: "SQL: inner join tables"},
		{Text: "OUTER JOIN", Description: "SQL: outer join tables"},
		{Text: "CROSS JOIN", Description: "SQL: cross join tables"},
		{Text: "NATURAL", Description: "SQL: natural join tables"},
		{Text: "LIMIT", Description: "SQL: upper Limit of records"},
		{Text: "OFFSET", Description: "SQL: identify the starting point to return result rows"},
		{Text: "CASE", Description: "SQL: branching by conditions"},
		{Text: "table", Description: "sqly command argument: table output format"},
		{Text: "markdown", Description: "sqly command argument: markdown table output format"},
		{Text: "csv", Description: "sqly command argument: csv output format"},
		{Text: "tsv", Description: "sqly command argument: tsv output format"},
		{Text: "ltsv", Description: "sqly command argument: ltsv output format"},
		{Text: "json", Description: "sqly command argument: json output format"},
		{Text: "excel", Description: "sqly command argument: excel output format"},
	}

	for _, v := range s.commands {
		suggest = append(suggest, prompt.Suggest{
			Text:        v.name,
			Description: "sqly command: " + v.description,
		})
	}

	tables, err := s.usecases.sqlite3.TablesName(ctx)
	if err != nil {
		return prompt.FilterHasPrefix(suggest, d.GetWordBeforeCursor(), true)
	}
	for _, v := range tables {
		suggest = append(suggest, prompt.Suggest{
			Text:        v.Name(),
			Description: "table: " + v.Name(),
		})

		table, err := s.usecases.sqlite3.Header(ctx, v.Name())
		if err != nil {
			return prompt.FilterHasPrefix(suggest, d.GetWordBeforeCursor(), true)
		}
		for _, h := range table.Header() {
			suggest = append(suggest, prompt.Suggest{
				Text:        h,
				Description: "header: " + h + " column in " + v.Name() + " table",
			})
		}
	}
	return prompt.FilterHasPrefix(suggest, d.GetWordBeforeCursor(), true)
}

// exec execute sqly helper command or sql query.
func (s *Shell) exec(ctx context.Context, request string) error {
	req := strings.TrimSpace(request)
	argv := strings.Split(trimGaps(req), " ")
	if argv[0] == "" {
		return nil // user only input enter, space tab
	}

	if err := s.recordUserRequest(ctx, req); err != nil {
		return err
	}

	if s.commands.hasCmd(argv[0]) {
		return s.commands[argv[0]].execute(ctx, s, argv[1:])
	}

	if s.commands.hasCmdPrefix(req) {
		return errors.New("no such sqly command: " + color.CyanString(req))
	}

	if err := s.execSQL(ctx, req); err != nil {
		return err
	}
	return nil
}

// execSQL execute SQL query.
func (s *Shell) execSQL(ctx context.Context, req string) error {
	req = strings.TrimRight(req, ";")
	table, affectedRows, err := s.usecases.sqlite3.ExecSQL(ctx, req)
	if err != nil {
		return err
	}
	if table == nil {
		fmt.Printf("affected is %d row(s)\n", affectedRows)
		return nil
	}

	// use --sql option and user want to output table data to file.
	if s.argument.NeedsOutputToFile() {
		return s.outputToFile(table)
	}
	table.Print(config.Stdout, s.state.mode.PrintMode)
	return nil
}

// outputToFile output table data to file.
func (s *Shell) outputToFile(table *model.Table) error {
	if err := dumpToFile(s, s.argument.Output.FilePath, table); err != nil {
		return err
	}
	fmt.Fprintf(config.Stdout, "Output sql result to %s (output mode=%s)\n",
		color.HiCyanString(s.argument.Output.FilePath), dumpMode(s.state.mode.PrintMode))
	return nil
}

// recordUserRequest record user request in DB.
func (s *Shell) recordUserRequest(ctx context.Context, request string) error {
	histories, err := s.usecases.history.List(ctx)
	if err != nil {
		return err
	}
	if err := s.usecases.history.Create(ctx, model.NewHistory(len(histories)+1, request)); err != nil {
		return fmt.Errorf("failed to store user input history: %w", err)
	}
	return nil
}

// trimGaps Remove white space at the beginning/end of a
// string and single out multiple white spaces between characters.
// Whitespace includes tabs and line feed.
// " Hello,    World  ! "         --> "Hello, World !"
// "Hello,\tWorld ! "             --> "Hello, World !"
// " \t\n\t Hello, \n\t World \n ! \n\t " --> "Hello, World !"
func trimGaps(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
