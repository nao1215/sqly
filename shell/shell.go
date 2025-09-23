// Package shell is sqly-shell. shell control user input
// (it's SQL query or helper command) and request the usecase layer to process it.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/nao1215/prompt"
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

const (
	// importCommand is the command for importing files
	importCommand = ".import"
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

	p, err := prompt.New(
		fmt.Sprintf("sqly:%s(%s)$ ", s.state.shortCWD(), s.state.mode.String()),
		prompt.WithCompleter(func(d prompt.Document) []prompt.Suggestion {
			return s.completerNew(ctx, d.Text)
		}),
		prompt.WithMemoryHistory(100),
		prompt.WithTheme(prompt.ThemeNightOwl),
		prompt.WithMultiline(true),
	)
	if err != nil {
		return "", err
	}
	defer p.Close()

	// Add existing history entries to the prompt
	for _, h := range histories.ToStringList() {
		p.AddHistory(h)
	}

	result, err := p.Run()
	if err != nil {
		return "", err
	}

	// The new prompt library seems to output an extra newline after input.
	// We need to move the cursor up one line to eliminate this extra space.
	fmt.Print("\033[1A") // Move cursor up one line

	// Trim any trailing newlines or whitespace that might be added by the prompt library
	return strings.TrimSpace(result), nil
}

// Suggest is a local struct to maintain compatibility with old code structure
type Suggest struct {
	Text        string
	Description string
}

// completerNew returns completions for the new prompt library
func (s *Shell) completerNew(ctx context.Context, input string) []prompt.Suggestion {
	oldSuggestions := s.getCompletions(ctx, input)
	completions := make([]prompt.Suggestion, 0, len(oldSuggestions))

	// Convert old suggestions to new format
	for _, suggest := range oldSuggestions {
		completions = append(completions, prompt.Suggestion{
			Text:        suggest.Text,
			Description: suggest.Description,
		})
	}

	return completions
}

// getCompletions returns suggestions for auto-completion.
func (s *Shell) getCompletions(ctx context.Context, input string) []Suggest {
	text := input
	// Get current word by finding last space and taking everything after
	lastSpace := strings.LastIndex(text, " ")
	var currentWord string
	if lastSpace >= 0 {
		currentWord = text[lastSpace+1:]
	} else {
		currentWord = text
	}
	// Check if we're dealing with a file path (contains / or \ or starts with common path patterns)
	isFilePath := strings.Contains(currentWord, "/") ||
		strings.Contains(currentWord, `\`) || // Windows path separator support
		strings.HasPrefix(currentWord, "./") ||
		strings.HasPrefix(currentWord, "../") ||
		strings.HasPrefix(currentWord, "~/") ||
		strings.HasPrefix(currentWord, "/") ||
		strings.HasPrefix(currentWord, `.\`) || // Windows relative path
		strings.HasPrefix(currentWord, `..\`) || // Windows relative path
		strings.HasPrefix(currentWord, `C:\`) || // Windows absolute path (common drive)
		// Also check if the word looks like a filename with supported extensions
		(strings.Contains(currentWord, ".") &&
			(strings.Contains(currentWord, ".csv") ||
				strings.Contains(currentWord, ".tsv") ||
				strings.Contains(currentWord, ".ltsv") ||
				strings.Contains(currentWord, ".xlsx") ||
				strings.Contains(currentWord, ".gz") ||
				strings.Contains(currentWord, ".bz2") ||
				strings.Contains(currentWord, ".xz") ||
				strings.Contains(currentWord, ".zst")))
	// Check if we're at the end of a path with / or \
	atEndOfPath := (strings.HasSuffix(text, "/") || strings.HasSuffix(text, `\`)) && len(strings.TrimSpace(text)) > 0
	// If it looks like a file path OR we're at end of path, provide file completions
	if isFilePath || atEndOfPath {
		fileCompletions := s.getFilePathCompletions(currentWord)
		if len(fileCompletions) > 0 {
			// For file path completions, we need to handle filtering differently
			// because GetWordBeforeCursor() returns empty for paths ending with / or \
			if atEndOfPath || strings.HasSuffix(currentWord, "/") || strings.HasSuffix(currentWord, `\`) {
				// When we're at the end of a path, return completions as-is
				return fileCompletions
			}
			// Otherwise, filter based on the current word
			return filterHasPrefix(fileCompletions, currentWord, true)
		}
	}

	// Check if this might be at the end where we expect a file path
	// (after SQL query or after .import command)
	words := strings.Fields(text)
	if len(words) > 0 {
		// If the line starts with .import, always provide file completions for the last argument
		if len(words) >= 1 && words[0] == importCommand {
			fileCompletions := s.getFilePathCompletions(currentWord)

			// For new full-path completion behavior, return all files without filtering
			// This ensures users see all importable files regardless of input
			return fileCompletions
		}

		// If we have a SQL query and the current word might be a filename
		if strings.Contains(strings.ToUpper(text), "FROM") ||
			strings.Contains(strings.ToUpper(text), "SELECT") {
			// Check if current word looks like it could be a file path
			if len(currentWord) > 0 && !strings.ContainsAny(currentWord, " \t") {
				// Try file completion as a fallback
				fileCompletions := s.getFilePathCompletions(currentWord)
				if len(fileCompletions) > 0 {
					// Mix with regular completions
					regularCompletions := s.getRegularCompletions(ctx, input)
					allCompletions := append(regularCompletions, fileCompletions...)
					return filterHasPrefix(allCompletions, currentWord, true)
				}
			}
		}
	}

	// Default to regular completions
	return s.getRegularCompletions(ctx, input)
}

// filterHasPrefix filters suggestions that have the given prefix (case-insensitive)
func filterHasPrefix(suggestions []Suggest, prefix string, _ bool) []Suggest {
	// ignoreCase parameter kept for compatibility but always treated as true
	var filtered []Suggest
	lowerPrefix := strings.ToLower(prefix)
	for _, s := range suggestions {
		if strings.HasPrefix(strings.ToLower(s.Text), lowerPrefix) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// getRegularCompletions returns the original completion logic
func (s *Shell) getRegularCompletions(ctx context.Context, input string) []Suggest {
	suggest := []Suggest{
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
		{Text: "excel", Description: "sqly command argument: excel output format"},
	}

	for _, v := range s.commands {
		suggest = append(suggest, Suggest{
			Text:        v.name,
			Description: "sqly command: " + v.description,
		})
	}

	tables, err := s.usecases.sqlite3.TablesName(ctx)
	if err != nil {
		// Get current word for filtering
		lastSpace := strings.LastIndex(input, " ")
		var currentWord string
		if lastSpace >= 0 {
			currentWord = input[lastSpace+1:]
		} else {
			currentWord = input
		}
		return filterHasPrefix(suggest, currentWord, true)
	}
	for _, v := range tables {
		suggest = append(suggest, Suggest{
			Text:        v.Name(),
			Description: "table: " + v.Name(),
		})

		table, err := s.usecases.sqlite3.Header(ctx, v.Name())
		if err != nil {
			// Get current word for filtering
			lastSpace := strings.LastIndex(input, " ")
			var currentWord string
			if lastSpace >= 0 {
				currentWord = input[lastSpace+1:]
			} else {
				currentWord = input
			}
			return filterHasPrefix(suggest, currentWord, true)
		}
		for _, h := range table.Header() {
			suggest = append(suggest, Suggest{
				Text:        h,
				Description: "header: " + h + " column in " + v.Name() + " table",
			})
		}
	}
	// Get current word for filtering
	lastSpace := strings.LastIndex(input, " ")
	var currentWord string
	if lastSpace >= 0 {
		currentWord = input[lastSpace+1:]
	} else {
		currentWord = input
	}
	return filterHasPrefix(suggest, currentWord, true)
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
	if err := table.Print(config.Stdout, s.state.mode.PrintMode); err != nil {
		return fmt.Errorf("failed to print table: %w", err)
	}
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

// supportedFileExtensions returns list of file extensions that sqly can process
func supportedFileExtensions() []string {
	return []string{".csv", ".tsv", ".ltsv", ".xlsx"}
}

// supportedCompressedExtensions returns list of compression extensions
func supportedCompressedExtensions() []string {
	return []string{".gz", ".bz2", ".xz", ".zst"}
}

// isValidFileForCompletion checks if file has supported extension
func isValidFileForCompletion(filename string) bool {
	// Handle compressed files by removing compression extension first
	name := filename
	for {
		found := false
		for _, compExt := range supportedCompressedExtensions() {
			if strings.HasSuffix(name, compExt) {
				name = strings.TrimSuffix(name, compExt)
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	// Check if the base file has supported extension
	for _, ext := range supportedFileExtensions() {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// getFilePathCompletions returns file path completions for importable files (recursive)
func (s *Shell) getFilePathCompletions(_ string) []Suggest {
	var suggestions []Suggest

	// Walk the current directory tree to find all importable files
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // Return error to stop walking
		}

		// Skip hidden directories and files (unless explicitly requested)
		// But don't skip the current directory "."
		if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// For files, check if they are importable
		if !d.IsDir() && isValidFileForCompletion(d.Name()) {
			suggestions = append(suggestions, Suggest{
				Text:        filepath.ToSlash(path),
				Description: "Importable file",
			})
		}

		return nil
	})

	if err != nil {
		return nil
	}
	return suggestions
}
