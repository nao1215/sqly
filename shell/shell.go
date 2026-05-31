// Package shell is sqly-shell. shell control user input
// (it's SQL query or helper command) and request the usecase layer to process it.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
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
	importCommand   = ".import"
	cdCommand       = ".cd"
	clearCommand    = ".clear"
	dumpCommand     = ".dump"
	exitCommand     = ".exit"
	headerCommand   = ".header"
	helpCommand     = ".help"
	lsCommand       = ".ls"
	modeCommand     = ".mode"
	tablesCommand   = ".tables"
	pwdCommand      = ".pwd"
	schemaCommand   = ".schema"
	describeCommand = ".describe"
	saveCommand     = ".save"

	msgImportableFile = "Importable file"
)

// Shell is main class of the sqly command.
// Shell is the interface to the user and requests processing from the usecase layer.
type Shell struct {
	argument  *config.Arg
	config    *config.Config
	commands  CommandList
	usecases  Usecases
	state     *state
	newPrompt promptFactory
	// stdin is the source for non-TTY batch mode. It defaults to os.Stdin and
	// is overridable in tests so piped input can be simulated without a terminal.
	stdin io.Reader
	// isTTY reports whether stdin is an interactive terminal. When false, Run
	// reads commands from stdin in batch mode instead of starting the prompt.
	isTTY func() bool
	// historyEnabled is true while command history can be persisted. It is
	// disabled for the session if the history DB cannot be created or written,
	// so automation does not fail on a read-only config location.
	historyEnabled bool
	// tableSources maps an imported table name to the source path it came from.
	// It is populated on every import and used by the --inspect report and by
	// write-back (.save) to map a table back to its source file.
	tableSources map[string]string
}

type promptSession interface {
	AddHistory(string)
	Close() error
	Run() (string, error)
	SetPrefix(string)
}

type promptFactory func(prefix string, completer func(prompt.Document) []prompt.Suggestion) (promptSession, error)

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
		newPrompt: func(prefix string, completer func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
			const historySize = 100

			return prompt.New(
				prefix,
				prompt.WithCompleter(completer),
				prompt.WithMemoryHistory(historySize),
				prompt.WithTheme(prompt.ThemeNightOwl),
				prompt.WithMultiline(true),
			)
		},
		stdin:          os.Stdin,
		isTTY:          config.IsInputFromTTY,
		historyEnabled: true,
		tableSources:   make(map[string]string),
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

	// --sheet only affects Excel imports; reject it up front when no input can
	// be an Excel file so a typo is not silently ignored.
	if err := s.validateSheetFlag(); err != nil {
		return err
	}

	// --inspect is self-contained; reject conflicting action/side-effect flags
	// up front instead of silently discarding them.
	if err := s.validateInspectFlags(); err != nil {
		return err
	}

	// --output is only honored by the --sql path (a single result written to one
	// file). Without --sql (no query, batch stdin, --sql-file, or interactive)
	// the flag was silently ignored, so reject it instead of looking successful.
	// Ref #318, #319.
	if s.argument.Output.FilePath != "" && s.argument.Query == "" {
		return errors.New("--output requires --sql")
	}

	// Reject an --output destination that is an existing directory before import,
	// so it is not silently rewritten to a sibling file.
	if err := ensureNotDirectory(s.argument.Output.FilePath); err != nil {
		return err
	}

	// --sql and --sql-file both supply a non-interactive query; accepting both
	// would be ambiguous. Read and validate the SQL file before importing so a
	// bad path fails fast without spending time on the import.
	if s.argument.Query != "" && s.argument.SQLFilePath != "" {
		return errors.New("--sql and --sql-file cannot be used together")
	}
	var sqlScript string
	if s.argument.SQLFilePath != "" {
		script, err := readSQLFile(s.argument.SQLFilePath)
		if err != nil {
			return err
		}
		sqlScript = script
	}

	if err := s.init(ctx); err != nil {
		// A partial import (some inputs loaded, some failed) keeps the loaded
		// tables usable, so the interactive shell still starts after a warning.
		// Non-interactive modes (--sql, --sql-file, --inspect, batch) treat it as
		// a hard failure so automation sees a non-zero exit. Ref #297, #300, #302.
		if errors.Is(err, errPartialImport) && s.startsInteractiveShell() {
			fmt.Fprintf(config.Stderr, "%v\n", err)
		} else {
			return err
		}
	}

	// --inspect is a non-interactive discovery path: after import it prints a
	// JSON report of the loaded tables and exits, so it takes precedence over
	// --sql and the interactive/batch paths.
	if s.argument.InspectFlag {
		return s.runInspect(ctx)
	}

	if err := s.validateSaveFlags(); err != nil {
		return err
	}

	if s.argument.Query != "" {
		if err := s.execSQL(ctx, s.argument.Query); err != nil {
			return err
		}
		return s.maybeSave(ctx)
	}

	// --sql-file runs the loaded script with the same statement-splitting and
	// error reporting as batch stdin mode, so multiline SQL and multiple
	// statements behave identically whether they arrive from a file or a pipe.
	if s.argument.SQLFilePath != "" {
		ranAny, err := s.runBatchReader(ctx, strings.NewReader(sqlScript))
		if err != nil {
			return err
		}
		if !ranAny {
			return nil
		}
		return s.maybeSave(ctx)
	}

	// Without a terminal (e.g. piped stdin) the interactive prompt cannot
	// initialize, so read SQL and helper commands from stdin in batch mode.
	if !s.isTTY() {
		ranAny, err := s.runBatch(ctx)
		if err != nil {
			return err
		}
		// Skip write-back when nothing ran (e.g. empty stdin), so an empty batch
		// does not trigger --save/--save-dir side effects. Ref #330, #331.
		if !ranAny {
			return nil
		}
		return s.maybeSave(ctx)
	}

	// Start shell
	s.printWelcomeMessage()
	return s.communicate(ctx)
}

// communicate is interactive command prompt for sqly.
// This function receive user input (it's SQL query or helper command) and
// request the usecase layer to process it.
func (s *Shell) communicate(ctx context.Context) error {
	p, err := s.newPromptSession(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := p.Close(); err != nil {
			fmt.Fprintf(config.Stderr, "failed to close prompt session: %v\n", err)
		}
	}()

	for {
		input, err := s.prompt(p)
		if err != nil {
			return err
		}
		if err = s.exec(ctx, input); err != nil {
			if errors.Is(err, ErrExitSqly) {
				return nil // user input ".exit"
			}
			fmt.Fprintf(config.Stderr, "%v\n", err)
			continue
		}
	}
}

func (s *Shell) newPromptSession(ctx context.Context) (promptSession, error) {
	p, err := s.newPrompt(s.promptPrefix(), func(d prompt.Document) []prompt.Suggestion {
		return s.completerNew(ctx, d.Text)
	})
	if err != nil {
		return nil, err
	}

	// Preload persisted history only when it is available; the prompt still
	// keeps in-session history when persistence is disabled. On read failure,
	// stay best-effort: disable history and start the shell anyway instead of
	// refusing to open the prompt.
	if s.historyEnabled {
		histories, err := s.usecases.history.List(ctx)
		if err != nil {
			s.disableHistory(err)
		} else {
			for _, h := range histories.ToStringList() {
				p.AddHistory(h)
			}
		}
	}

	return p, nil
}

// disableHistory turns off history persistence for the rest of the session and
// warns once. It is called when the history DB cannot be created at startup or a
// later read/write fails (e.g. the DB became read-only), so history stays
// best-effort and never aborts the requested --sql, --inspect, or batch command.
func (s *Shell) disableHistory(err error) {
	if !s.historyEnabled {
		return
	}
	s.historyEnabled = false
	fmt.Fprintf(config.Stderr, "warning: command history disabled (%v). Set SQLY_HISTORY_DB_PATH to a writable path to enable it.\n", err)
}

// startsInteractiveShell reports whether this run will open the interactive
// prompt: a terminal with no non-interactive action requested (--inspect,
// --sql, --sql-file). Batch mode (non-TTY) and those flags are non-interactive.
func (s *Shell) startsInteractiveShell() bool {
	return s.isTTY() && !s.argument.InspectFlag && s.argument.Query == "" && s.argument.SQLFilePath == ""
}

// init store CSV data to in-memory DB and create table for sqly history.
func (s *Shell) init(ctx context.Context) error {
	// History is best-effort: a read-only or unwritable history DB (CI,
	// sandboxes, containers) must not block the requested query or command.
	// Disable history for the session and warn instead of failing.
	if err := s.usecases.history.CreateTable(ctx); err != nil {
		s.disableHistory(err)
	}

	paths := s.argument.FilePaths
	stdinAbsPath := ""
	// When --stdin is set, stage piped stdin as a dataset file and import it
	// alongside the file/directory arguments so it can be queried and joined.
	if s.argument.StdinFormat != "" {
		// stageStdinDataset reads stdin to EOF; on a terminal that would hang
		// waiting for the user. --stdin is only meaningful with piped input.
		if s.isTTY() {
			return errors.New("--stdin requires piped or redirected stdin")
		}
		stdinPath, cleanup, err := s.stageStdinDataset()
		if err != nil {
			return err
		}
		defer cleanup()
		if abs, err := filepath.Abs(stdinPath); err == nil {
			stdinAbsPath = abs
		} else {
			stdinAbsPath = stdinPath
		}
		paths = append([]string{stdinPath}, paths...)
	}

	if len(paths) == 0 {
		return nil
	}
	importErr := s.commands.importCommand(ctx, s, paths)
	// Re-point any stdin-derived table's source from the ephemeral temp path to
	// a stable "stdin" marker, so --inspect does not leak the temp path (#290)
	// and write-back can reject stdin-backed tables instead of writing to a
	// deleted temp file (#291).
	if stdinAbsPath != "" {
		s.remapStdinTableSources(stdinAbsPath)
	}
	return importErr
}

// stdinTableSource is the synthetic source recorded for tables imported from a
// piped --stdin dataset, in place of the ephemeral staging temp path.
const stdinTableSource = "stdin"

// remapStdinTableSources replaces the recorded source of any table staged from
// stdin (its temp path) with the stable stdinTableSource marker.
func (s *Shell) remapStdinTableSources(stdinAbsPath string) {
	for name, src := range s.tableSources {
		if src == stdinAbsPath {
			s.tableSources[name] = stdinTableSource
		}
	}
}

// stdinFormatExtensions maps the --stdin format names to file extensions. The
// format-name keys intentionally repeat strings used by unrelated features
// (completion, mode names), so goconst is suppressed here.
//
//nolint:goconst // format-name registry keys
var stdinFormatExtensions = map[string]string{
	"csv":   model.ExtCSV,
	"tsv":   model.ExtTSV,
	"ltsv":  model.ExtLTSV,
	"json":  model.ExtJSON,
	"jsonl": model.ExtJSONL,
}

// stageStdinDataset reads all of stdin into a temporary file named after the
// stdin table so filesql imports it like a normal file. Why a temp file:
// filesql loads by path, and staging keeps the import path identical to file
// arguments (including table naming and joins). The returned cleanup removes the
// temp directory; it is safe to call after import because the data is already
// copied into the shared database.
func (s *Shell) stageStdinDataset() (string, func(), error) {
	ext, ok := stdinFormatExtensions[s.argument.StdinFormat]
	if !ok {
		return "", nil, fmt.Errorf("unsupported --stdin format %q (supported: csv, tsv, ltsv, json, jsonl)", s.argument.StdinFormat)
	}

	dir, err := os.MkdirTemp("", "sqly-stdin-")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir for stdin data: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	path := filepath.Join(dir, s.argument.StdinTableName+ext)
	f, err := os.Create(path) //nolint:gosec // path is a sqly-generated temp path
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create stdin staging file: %w", err)
	}
	if _, err := io.Copy(f, s.stdin); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("read stdin data: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close stdin staging file: %w", err)
	}
	return path, cleanup, nil
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
func (s *Shell) prompt(p promptSession) (string, error) {
	p.SetPrefix(s.promptPrefix())
	return p.Run()
}

func (s *Shell) promptPrefix() string {
	return fmt.Sprintf("sqly:%s(%s)$ ", s.state.shortCWD(), s.state.mode.String())
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
		(strings.Contains(currentWord, ".") && s.usecases.importer.IsSupportedFile(currentWord))
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
					regularCompletions = append(regularCompletions, fileCompletions...)
					return filterHasPrefix(regularCompletions, currentWord, true)
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
		{Text: "json", Description: "sqly command argument: json output format"},
		{Text: "ndjson", Description: "sqly command argument: ndjson output format"},
		{Text: "excel", Description: "sqly command argument: excel output format"},
		{Text: "parquet", Description: "sqly command argument: parquet export format"},
	}

	for _, v := range s.commands {
		suggest = append(suggest, Suggest{
			Text:        v.name,
			Description: "sqly command: " + v.description,
		})
	}

	tables, err := s.usecases.metadata.TablesName(ctx)
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

		table, err := s.usecases.metadata.Header(ctx, v.Name())
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
	argv, err := splitArgs(req)
	if err != nil {
		return err
	}
	if len(argv) == 0 || argv[0] == "" {
		return nil // user only input enter, space tab
	}

	// Skip history persistence when it is disabled so a read-only history DB
	// cannot fail the requested command. History is best-effort: a runtime
	// failure after startup disables history for the rest of the session and
	// warns, instead of aborting the command.
	if s.historyEnabled {
		if err := s.recordUserRequest(ctx, req); err != nil {
			s.disableHistory(err)
		}
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
	table, affectedRows, err := s.usecases.query.ExecSQL(ctx, req)
	if err != nil {
		return err
	}
	if table == nil {
		fmt.Fprintf(config.Stdout, "affected is %d row(s)\n", affectedRows)
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

// outputToFile output table data to file. The export format and compression are
// resolved from both the chosen output mode and the destination path, so a path
// like "result.parquet" or "out.ndjson.gz" is honored even without a mode flag.
func (s *Shell) outputToFile(table *model.Table) error {
	mode := s.state.mode.PrintMode
	explicit := model.ExportFormatFromPrintMode(mode)
	exportFmt, compression, err := model.ResolveOutputTarget(s.argument.Output.FilePath, explicit, mode != model.PrintModeTable)
	if err != nil {
		return err
	}
	filePath := model.BuildOutputPath(s.argument.Output.FilePath, exportFmt, compression)
	if err := s.usecases.export.DumpTable(filePath, table, exportFmt, compression); err != nil {
		return err
	}
	// Status for a file-output operation is control-plane information; the data
	// went to the file, so keep stdout empty and report progress on stderr.
	fmt.Fprintf(config.Stderr, "Output sql result to %s (output mode=%s)\n",
		color.HiCyanString(filePath), exportFmt.String())
	return nil
}

// ensureNotDirectory rejects an output destination that already exists as a
// directory. Without this check the path gets a format extension appended,
// silently writing to a sibling file (e.g. "out" -> "out.csv") instead of the
// directory the user named. A non-existent path is fine; it is created on write.
func ensureNotDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil //nolint:nilerr // a missing path is created at write time; other errors surface there
	}
	if info.IsDir() {
		return fmt.Errorf("output destination %q is a directory; specify a file path", path)
	}
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

// isValidFileForCompletion checks if file has a supported extension.
func (s *Shell) isValidFileForCompletion(filename string) bool {
	return s.usecases.importer.IsSupportedFile(filename)
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
		if !d.IsDir() && s.isValidFileForCompletion(d.Name()) {
			suggestions = append(suggestions, Suggest{
				Text:        filepath.ToSlash(path),
				Description: msgImportableFile,
			})
		}

		return nil
	})
	if err != nil {
		return nil
	}
	return suggestions
}
