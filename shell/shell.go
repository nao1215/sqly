// Package shell is sqly-shell. shell control user input
// (it's SQL query or helper command) and request the usecase layer to process it.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	importCommand      = ".import"
	rowMismatchCommand = ".row-mismatch"
	cdCommand          = ".cd"
	clearCommand       = ".clear"
	dumpCommand        = ".dump"
	exitCommand        = ".exit"
	headerCommand      = ".header"
	helpCommand        = ".help"
	lsCommand          = ".ls"
	modeCommand        = ".mode"
	tablesCommand      = ".tables"
	pwdCommand         = ".pwd"
	schemaCommand      = ".schema"
	describeCommand    = ".describe"
	saveCommand        = ".save"
	dialectCommand     = ".dialect"
	helpFlag           = "--help"
	versionFlag        = "--version"
	helpArgument       = "help"

	msgImportableFile = "Importable file"
	msgImportableDir  = "Directory"
	msgExcelSheet     = "Excel sheet"
	// formatNameTable is the default output format's name, used where a literal
	// would otherwise repeat across completion, schema output, and help.
	formatNameTable = "table"
)

// errNoStatements is returned by a non-interactive run that reads stdin in batch
// mode but executes nothing (no TTY and empty or comment-only stdin, with no
// --sql/--sql-file). Without it the run would exit 0 silently, so a headless
// wrapper or CI job could mistake a no-op for a completed query.
var errNoStatements = errors.New("no TTY detected and no SQL was received on stdin; pipe a SQL query or sqly command, or pass --sql/--sql-file (run with --help for usage)")

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
	// stdinStagedPath is the temporary staging file a --stdin-format dataset is written
	// to before import. It is recorded so import error reporting can map that
	// random path (and the temp dir filesql embeds in its own error) back to a
	// stable "stdin" reference, instead of leaking the implementation detail.
	stdinStagedPath string
	// tableSources maps an imported table name to the source path it came from.
	// It is populated on every import and used by the --inspect report and by
	// write-back (.save) to map a table back to its source file.
	tableSources map[string]string
	// dirImported marks tables that came from a directory import. Their
	// tableSources entry may point at the per-file source (for --inspect
	// provenance), but write-back still rejects them because a directory import
	// is not a single editable source the session owns.
	dirImported map[string]bool
	// dataChanged is set when an executed statement actually changed table data
	// (a DML that affected at least one row, or a DML RETURNING that returned at
	// least one row). A non-interactive run only writes back when data changed, so
	// an EXPLAIN or a zero-row DML leaves source files untouched.,
	//,.
	dataChanged bool
	// importBaseline maps an imported file-backed table name to a fingerprint of its
	// content as loaded. Write-back compares the current content against this
	// baseline and skips a table whose content is unchanged, so a session that only
	// touched a TEMP or SQL-created scratch table, or that made net-zero edits that
	// cancel out, never rewrites an untouched source file. dataChanged is a coarse
	// session-wide gate; this map is the per-table truth that prevents a spurious
	// write.
	importBaseline map[string]string
	// pendingAffected holds "affected is N row(s)" lines produced during a
	// write-back run. They are buffered rather than printed immediately and flushed
	// to stdout only after write-back succeeds, so a run that fails during
	// write-back leaves stdout free of success counts.
	pendingAffected []string
	// deferAffectedCounts holds back the "affected is N row(s)" lines until the
	// run has succeeded. It is set only for a script that ends in .save, where a
	// failure during write-back would otherwise leave stdout claiming rows were
	// changed on disk. Every other run prints each count in place, so the counts
	// stay interleaved with the results in statement order.
	deferAffectedCounts bool
	// files is every filesystem call the write-back commit path makes. It is a
	// field so a test can fail exactly one of them; production always gets the
	// real filesystem from NewShell.
	files fileOps
	// collectingOutput routes rowset results into capturedRowsets instead of
	// printing them, so --sql-file combined with --output can export the single
	// result set the script produces. No-rowset statements stay silent in this
	// mode, keeping stdout clean for the exported-data run.
	collectingOutput bool
	// printedResults counts the result sets a non-interactive run has already
	// written to stdout, so the second and later ones can be separated from the
	// one before.
	printedResults int
	// capturedRowsets holds the rowset results produced while collectingOutput is
	// set. The one-result-set contract is enforced after the script finishes: zero
	// or more than one captured rowset is an error.
	capturedRowsets []*model.Table
	// completionTableKey fingerprints the table-name set the cached table/column
	// completion suggestions were built from. completionTableCols holds those
	// suggestions. Together they let interactive completion reuse table and column
	// metadata across keystrokes instead of querying every table's header on each
	// one, rebuilding only when the table set changes (or after an import).
	completionTableKey  string
	completionTableCols []Suggest
	// httpClient downloads remote datasets for HTTP/HTTPS imports. It is
	// overridable in tests so httptest servers can supply their own transport.
	httpClient *http.Client
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
	// Apply the initial SQL dialect from --dialect. Loading always uses SQLite;
	// only user queries run through ExecSQL are translated.
	usecases.query.SetDialect(arg.Dialect)
	return &Shell{
		argument: arg,
		config:   cfg,
		commands: cmds,
		usecases: usecases,
		state:    state,
		files:    defaultFileOps(),
		newPrompt: func(prefix string, completer func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
			const historySize = 100

			return prompt.New(
				prefix,
				prompt.WithCompleter(completer),
				prompt.WithMemoryHistory(historySize),
				prompt.WithTheme(prompt.ThemeNightOwl),
				prompt.WithMultiline(true),
				prompt.WithIsComplete(sqlInputComplete),
				prompt.WithContinuationPrefix(continuationPrefix),
				prompt.WithWordEscape(),
				prompt.WithKeyMap(sqlyKeyMap()),
				prompt.WithPersistentRawMode(),
			)
		},
		stdin:          os.Stdin,
		isTTY:          config.IsInputFromTTY,
		historyEnabled: true,
		tableSources:   make(map[string]string),
		httpClient: &http.Client{
			// Bound the full request/response body read so a server that stalls
			// mid-download cannot hang the CLI indefinitely.
			Timeout: 15 * time.Minute,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
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

	// sqly is flag-driven and has no subcommands, so a common first try like
	// "sqly help" or "sqly version" is otherwise parsed as an input path and fails
	// with a confusing "path does not exist" import error. Detect that exact
	// accidental form and point the user at the right flag instead.
	if hint, ok := positionalSubcommandHint(s.argument.FilePaths); ok {
		return errors.New(hint)
	}

	// --inspect is self-contained; reject conflicting action/side-effect flags
	// up front instead of silently discarding them.
	if err := s.validateInspectFlags(); err != nil {
		return err
	}

	// --output is honored by --sql (a single result) and --sql-file (the script's
	// one result set). Without either (batch stdin or interactive) the flag was
	// silently ignored, so reject it instead of looking successful.
	if s.argument.Output.FilePath != "" && s.argument.Query == "" && s.argument.SQLFilePath == "" {
		return errors.New("--output requires --sql or --sql-file")
	}

	// Check the destination before the import, so a run that cannot write its
	// result never spends time reading files: an existing directory would be
	// silently rewritten to a sibling file, and a missing parent directory would
	// only surface after the query had already run.
	if err := ensureWritableDestination(s.argument.Output.FilePath); err != nil {
		return err
	}

	// Excel and Parquet are binary container formats with no on-screen rendering,
	// so a query run that selects one without a destination used to print CSV to
	// stdout instead: the user asked for one format and silently received another.
	// Ask for the destination rather than guessing. The interactive shell is
	// unaffected: there the format is a standing choice that .dump acts on.
	if err := s.validateBinaryOutputFormat(); err != nil {
		return err
	}

	// An import option the user typed that no input of this run can use is a
	// no-op the user did not ask for. Reject it before reading anything.
	if err := s.validateOptionApplicability(); err != nil {
		return err
	}

	// --sql and --sql-file both supply a non-interactive query; accepting both
	// would be ambiguous. Read and validate the SQL file before importing so a
	// bad path fails fast without spending time on the import.
	if s.argument.Query != "" && s.argument.SQLFilePath != "" {
		return errors.New("--sql and --sql-file cannot be used together")
	}
	// --stdin-format stages piped stdin as a dataset, which consumes stdin entirely, so
	// nothing remains to carry a query. Require an explicit query source;
	// otherwise the dataset is imported and immediately discarded with a success
	// exit code.
	if s.argument.StdinFormat != "" && s.argument.Query == "" && s.argument.SQLFilePath == "" && !s.argument.InspectFlag {
		return errors.New("--stdin-format provides a dataset but no query was given; add --sql, --sql-file, or --inspect")
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
		// a hard failure so automation sees a non-zero exit.
		var partial *partialImportError
		if errors.As(err, &partial) && s.startsInteractiveShell() {
			// Explain the shell state instead of the bare partial-import error, so a
			// working session does not look broken. The failing inputs were already
			// listed above by init.
			fmt.Fprintln(config.Stderr, partialImportStartupMessage(partial.succeeded, partial.failed))
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

	if s.argument.Query != "" {
		// Direct --sql runs exactly one statement and prints (or exports) its single
		// result. Multiple statements separated by ";" would silently drop every
		// result but the last, so reject them up front instead of running the input
		// and discarding output.
		if n := countSQLStatements(s.argument.Query); n > 1 {
			return fmt.Errorf("--sql accepts a single SQL statement, but got %d; run one statement per invocation or use --sql-file for a multi-statement script", n)
		}
		if err := s.execSQL(ctx, s.argument.Query); err != nil {
			return err
		}
		return s.finishNonInteractive(ctx)
	}

	// --sql-file runs the loaded script with the same statement-splitting and
	// error reporting as batch stdin mode, so multiline SQL and multiple
	// statements behave identically whether they arrive from a file or a pipe.
	if s.argument.SQLFilePath != "" {
		if err := s.prepareForScript(ctx, sqlScript); err != nil {
			return err
		}
		// With --output, export the script's single result set to the file instead
		// of printing each statement's result, mirroring how --sql exports.
		if s.argument.Output.FilePath != "" {
			return s.runSQLFileToOutput(ctx, sqlScript)
		}
		ranAny, err := s.runScript(ctx, sqlScript)
		if err != nil {
			return err
		}
		if !ranAny {
			return nil
		}
		return s.finishNonInteractive(ctx)
	}

	// Without a terminal (e.g. piped stdin) the interactive prompt cannot
	// initialize, so read SQL and helper commands from stdin in batch mode. The
	// whole script is read up front so write-back can be validated before the
	// first statement runs and skipped for a read-only script.
	if !s.isTTY() {
		data, err := io.ReadAll(s.stdin)
		if err != nil {
			return fmt.Errorf("failed to read batch input: %w", err)
		}
		batchScript := strings.TrimPrefix(string(data), "\ufeff")
		if err := s.prepareForScript(ctx, batchScript); err != nil {
			return err
		}
		ranAny, err := s.runScript(ctx, batchScript)
		if err != nil {
			return err
		}
		// A non-interactive run that executed nothing (no TTY and empty or
		// comment-only stdin, with no --sql/--sql-file) is a silent no-op that
		// still exits 0, so headless wrappers and CI mistake it for a completed
		// query. Surface a hint and fail instead.
		if !ranAny {
			return errNoStatements
		}
		return s.finishNonInteractive(ctx)
	}

	// Start shell. The welcome banner is printed inside communicate, only after
	// the prompt session is created, so a terminal-backend failure (no usable
	// /dev/tty) reports a clear error instead of looking like the shell started
	// and then crashed right after the banner.
	return s.communicate(ctx)
}

// communicate is interactive command prompt for sqly.
// This function receive user input (it's SQL query or helper command) and
// request the usecase layer to process it.
func (s *Shell) communicate(ctx context.Context) error {
	p, err := s.newPromptSession(ctx)
	if err != nil {
		// The interactive prompt needs a usable terminal (it opens /dev/tty).
		// When that backend is unavailable (some PTY wrappers, headless
		// containers, IDE terminals), report the requirement clearly and point at
		// the non-interactive modes, instead of surfacing a raw "open /dev/tty"
		// error after the welcome banner already printed.
		return fmt.Errorf("cannot start the interactive shell: no usable terminal (%w). Run a query non-interactively, e.g. sqly --sql \"SELECT ...\" file.csv, or pipe SQL/commands via stdin", err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			fmt.Fprintf(config.Stderr, "failed to close prompt session: %v\n", err)
		}
	}()

	// The prompt session is ready, so it is now safe to announce the shell.
	s.printWelcomeMessage()

	// Persistent raw mode disables the terminal's LF-to-CRLF mapping, so route
	// command output through CRLF translation for the session. Restored on exit.
	restoreOutput := installCRLFTranslation()
	defer restoreOutput()

	for {
		input, err := s.prompt(p)
		if err != nil {
			// Ctrl-D / EOF ends the session like ".exit": a normal exit, not a
			// user-facing error. The prompt library reports this as io.EOF
			// (Ctrl-D on an empty line) or prompt.ErrEOF (input stream closed);
			// treat both as a clean termination so no raw "EOF" text leaks out.
			if errors.Is(err, io.EOF) || errors.Is(err, prompt.ErrEOF) {
				return nil
			}
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

// sqlyKeyMap returns the prompt key map with the Emacs-style control shortcuts
// the sqly shell documents on top of the prompt library defaults. The library
// already binds Ctrl+A/E/K/U/W/R and the arrow keys; this adds the control-key
// equivalents the docs advertise: Ctrl+P/Ctrl+N for history navigation,
// Ctrl+F/Ctrl+B for character movement, and Ctrl+L to clear the screen.
func sqlyKeyMap() *prompt.KeyMap {
	km := prompt.NewDefaultKeyMap()
	km.Bind('\x10', prompt.ActionHistoryUp)   // Ctrl+P: previous command
	km.Bind('\x0e', prompt.ActionHistoryDown) // Ctrl+N: next command
	km.Bind('\x06', prompt.ActionMoveRight)   // Ctrl+F: forward one character
	km.Bind('\x02', prompt.ActionMoveLeft)    // Ctrl+B: backward one character
	km.Bind('\x0c', prompt.ActionClearScreen) // Ctrl+L: clear the screen
	return km
}

func (s *Shell) newPromptSession(ctx context.Context) (promptSession, error) {
	p, err := s.newPrompt(s.promptPrefix(), func(d prompt.Document) []prompt.Suggestion {
		return s.completeDocument(ctx, d)
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

// validateBinaryOutputFormat rejects an --output-format that writes a binary
// file when the run has nowhere to write it. Excel and Parquet cannot be
// rendered to a terminal, so a query run without --output silently fell back to
// CSV on stdout. It only applies to a run that produces a result on its own
// (--sql or --sql-file); in the shell the format is a standing choice and .dump
// supplies the destination.
func (s *Shell) validateBinaryOutputFormat() error {
	if s.argument.Output.FilePath != "" {
		return nil
	}
	if s.argument.Query == "" && s.argument.SQLFilePath == "" {
		return nil
	}
	switch s.argument.Output.Mode {
	case model.PrintModeExcel, model.PrintModeParquet:
		return fmt.Errorf("--output-format %s writes a binary file and cannot be printed; add --output FILE",
			s.argument.Output.Mode)
	default:
		return nil
	}
}

// partialImportStartupMessage explains the shell state after a partial startup
// import: the shell did start, some inputs loaded and are queryable now, and the
// failing inputs were already listed above. It replaces the bare "one or more
// inputs failed to import", which made a working session look broken.
func partialImportStartupMessage(succeeded, failed int) string {
	total := succeeded + failed
	return fmt.Sprintf("sqly started with partial data: %d of %d inputs imported, %d failed (listed above). The imported tables are ready to query.", succeeded, total, failed)
}

// positionalSubcommandHint reports whether the first positional argument is the
// accidental subcommand form "help" or "version" (the flags are --help and
// --version) and returns a correcting hint. Why check os.Stat: a real file or
// directory actually named "help"/"version" should still import, so the hint
// fires only when no such path exists. The match is case-insensitive to also
// catch "HELP"/"Version".
func positionalSubcommandHint(paths []string) (string, bool) {
	if len(paths) == 0 {
		return "", false
	}
	first := paths[0]
	var flag string
	switch strings.ToLower(first) {
	case helpArgument:
		flag = helpFlag
	case "version":
		flag = versionFlag
	default:
		return "", false
	}
	if _, err := os.Stat(first); err == nil {
		return "", false // a real path with that name wins
	}
	return fmt.Sprintf("sqly is flag-driven and has no subcommands; use %q. Run \"sqly --help\" for usage. Helper commands like .tables and .import run inside the shell or batch stdin.", flag), true
}

// reportOnly reports whether the run is an inspect invocation whose only
// intended output is the structured report. Successful import progress banners
// are suppressed in this mode so a clean run stays quiet on stderr; warnings and
// errors still print.
func (s *Shell) reportOnly() bool {
	return s.argument.InspectFlag
}

// init store CSV data to in-memory DB and create table for sqly history.
func (s *Shell) init(ctx context.Context) error {
	// Apply the malformed-row import policy from the --row-mismatch flag before any
	// file is loaded, so the initial import honors the requested handling.
	s.usecases.importer.SetRowMismatchPolicy(s.state.rowMismatch)

	// History is best-effort: a read-only or unwritable history DB (CI,
	// sandboxes, containers) must not block the requested query or command.
	// Disable history for the session and warn instead of failing.
	if err := s.usecases.history.CreateTable(ctx); err != nil {
		s.disableHistory(err)
	}

	paths := s.argument.FilePaths
	stdinAbsPath := ""
	stagedStdinPath := ""
	// When --stdin-format is set, stage piped stdin as a dataset file and import it
	// alongside the file/directory arguments so it can be queried and joined.
	if s.argument.StdinFormat != "" {
		// stageStdinDataset reads stdin to EOF; on a terminal that would hang
		// waiting for the user. --stdin-format is only meaningful with piped input.
		if s.isTTY() {
			return errors.New("--stdin-format requires piped or redirected stdin")
		}
		stdinPath, cleanup, err := s.stageStdinDataset()
		if err != nil {
			return err
		}
		defer cleanup()
		stagedStdinPath = stdinPath
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
	// Record the staged path so import error reporting can map it (and the random
	// temp dir filesql embeds in its own error) back to a stable "stdin"
	// reference instead of leaking the implementation-detail path.
	s.stdinStagedPath = stagedStdinPath
	importErr := s.commands.importCommand(ctx, s, paths)
	// Re-point any stdin-derived table's source from the ephemeral temp path to
	// a stable "stdin" marker, so --inspect does not leak the temp path
	// and write-back can reject stdin-backed tables instead of writing to a
	// deleted temp file.
	if stdinAbsPath != "" {
		s.remapStdinTableSources(stdinAbsPath)
	}
	return importErr
}

// stdinTableSource is the synthetic source recorded for tables imported from a
// piped --stdin-format dataset, in place of the ephemeral staging temp path.
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

// stdinFormatExtensions maps the --stdin-format format names to file extensions. The
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
		return "", nil, fmt.Errorf("unsupported --stdin-format value %q: want csv, tsv, ltsv, json, or jsonl", s.argument.StdinFormat)
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

// continuationPrefix marks the lines of a statement sqly is still collecting.
// A query typed without a trailing ";" is buffered (see sqlInputComplete), and
// without a marker the cursor simply dropped to a bare line, which looks exactly
// like a hung program: nothing says the shell is waiting for the rest of the
// statement rather than stuck. sqlite3, psql, and mysql all show one.
const continuationPrefix = "   ...> "

func (s *Shell) promptPrefix() string {
	return fmt.Sprintf("sqly:%s(%s)$ ", s.state.shortCWD(), s.state.mode.displayName())
}

// sqlInputComplete reports whether the interactive buffer holds a statement
// ready to run, so the prompt submits on Enter instead of continuing on a new
// line. Without this, every newline submits, splitting a pasted or typed
// multi-line statement into separate executions.
//
// SQL is complete when it ends with ";". A dot-command (".tables", ".import",
// ...) and an empty buffer also submit. Pressing Enter on a blank continuation
// line force-submits whatever is buffered, so a query typed without a trailing
// ";" still runs without forcing the user to add one.
func sqlInputComplete(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, ".") {
		return true
	}
	if strings.HasSuffix(trimmed, ";") {
		return true
	}
	// The current line is the text after the last newline. When it is blank the
	// user pressed Enter on an empty continuation line, which force-submits.
	lastLine := input
	if i := strings.LastIndexByte(input, '\n'); i >= 0 {
		lastLine = input[i+1:]
	}
	return strings.TrimSpace(lastLine) == ""
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

// completeDocument returns completions for the token at the cursor. It uses the
// text before the cursor (not the whole line) so editing an earlier token and
// pressing TAB completes that token instead of the line ending.
func (s *Shell) completeDocument(ctx context.Context, d prompt.Document) []prompt.Suggestion {
	return s.completerNew(ctx, d.TextBeforeCursor())
}

// getCompletions returns suggestions for auto-completion.
func (s *Shell) getCompletions(ctx context.Context, input string) []Suggest {
	text := input
	// Get the current word, treating backslash-escaped whitespace as part of it so
	// a path like "my\ dir/in" stays one word. This matches the prompt library's
	// escaped word boundary (enabled with WithWordEscape) used to accept the
	// completion, so the directory portion is not lost when descending.
	currentWord := currentCompletionWord(text)

	// Split the already-typed part of the line into shell-aware tokens so a
	// quoted or escaped earlier argument (for example a workbook path with
	// spaces) stays one token. completed excludes the in-progress word; its index
	// is therefore len(completed).
	completed := completedCommandWords(text, currentWord)

	// Command-aware path completion: the path-taking helper commands complete
	// filesystem paths at their path argument. .cd and .save target a directory,
	// so only directories are offered; .ls/.dump/.import also offer importable
	// files. This runs before the generic path detection so a directory-only
	// command is never given file suggestions.
	if len(completed) >= 1 {
		if pathArg, multi, dirsOnly, ok := pathCommandSpec(completed[0]); ok {
			argIndex := len(completed) // the in-progress word is the next argument
			if argIndex == pathArg || (multi && argIndex >= pathArg) {
				if quote, rawInner, ok := openQuotePrefix(currentWord); ok {
					return keepDirsOnly(s.getQuotedFilePathCompletions(rawInner, quote), dirsOnly)
				}
				return keepDirsOnly(s.getFilePathCompletions(currentWord), dirsOnly)
			}
		}
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
			// Suggestions use "/" (slashifyBase), so slashify the word too;
			// otherwise a Windows-style prefix such as "C:\dir\fi" never
			// prefix-matches "C:/dir/file.csv".
			return filterHasPrefix(fileCompletions, slashifyBase(currentWord), true)
		}
	}

	// Check if this might be at the end where we expect a file path
	// (after a SQL query). Helper-command path completion is handled above.
	words := strings.Fields(text)
	if len(words) > 0 {
		// If we have a SQL query and the current word might be a filename
		if strings.Contains(strings.ToUpper(text), "FROM") ||
			strings.Contains(strings.ToUpper(text), "SELECT") {
			// Check if current word looks like it could be a file path
			if len(currentWord) > 0 && !strings.ContainsAny(currentWord, " \t") {
				// Try file completion as a fallback
				fileCompletions := s.getFilePathCompletions(currentWord)
				if len(fileCompletions) > 0 {
					// Slashify the word so a Windows-style prefix matches the
					// slash-normalized suggestions; table and keyword prefixes
					// have no backslash, so they are unaffected.
					regularCompletions := s.getRegularCompletions(ctx, input)
					regularCompletions = append(regularCompletions, fileCompletions...)
					return filterHasPrefix(regularCompletions, slashifyBase(currentWord), true)
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
		{Text: kwSelect, Description: "SQL: get records from table"},
		{Text: "INSERT INTO", Description: "SQL: creates one or more new records in an existing table"},
		{Text: kwUpdate, Description: "SQL: update one or more records"},
		{Text: "AS", Description: "SQL: set alias name"},
		{Text: "FROM", Description: "SQL: specify the table"},
		{Text: "WHERE", Description: "SQL: search condition"},
		{Text: "GROUP BY", Description: "SQL: groping records"},
		{Text: "HAVING", Description: "SQL: extraction conditions for records after grouping"},
		{Text: "ORDER BY", Description: "SQL: sort result"},
		{Text: kwValues, Description: "SQL: specify values to be inserted or updated"},
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
		{Text: formatNameTable, Description: "sqly command argument: table output format"},
		{Text: "markdown", Description: "sqly command argument: markdown table output format"},
		{Text: "csv", Description: "sqly command argument: csv output format"},
		{Text: "tsv", Description: "sqly command argument: tsv output format"},
		{Text: "ltsv", Description: "sqly command argument: ltsv output format"},
		{Text: "json", Description: "sqly command argument: json output format"},
		{Text: "jsonl", Description: "sqly command argument: jsonl (newline-delimited JSON) output format"},
		{Text: "excel", Description: "sqly command argument: excel output format"},
		{Text: "parquet", Description: "sqly command argument: parquet export format"},
		{Text: "sqlite", Description: "sqly command argument: SQLite query dialect (default)"},
		{Text: "mysql", Description: "sqly command argument: MySQL query dialect"},
		{Text: "postgresql", Description: "sqly command argument: PostgreSQL query dialect"},
		{Text: "googlesql", Description: "sqly command argument: GoogleSQL query dialect"},
	}

	for _, v := range s.commands {
		suggest = append(suggest, Suggest{
			Text:        v.name,
			Description: "sqly command: " + v.description,
		})
	}

	// A line still typing a dot-command (no argument yet) never references tables
	// or columns, so skip the table/column metadata entirely for it.
	if !isTypingDotCommand(input) {
		suggest = append(suggest, s.tableColumnSuggestions(ctx)...)
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

// isTypingDotCommand reports whether input is a helper dot-command name still
// being typed (starts with "." and has no whitespace yet), such as ".he". Such a
// line cannot reference a table or column, so completion can skip schema lookups.
func isTypingDotCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, ".") && !strings.ContainsAny(trimmed, " \t")
}

// tableColumnSuggestions returns table-name and column-header completion
// suggestions, cached by the current table-name set. The headers are fetched
// only when that set changes, so consecutive keystrokes reuse the cache instead
// of querying every table's header on each one. Returns nil if the table list
// cannot be read.
func (s *Shell) tableColumnSuggestions(ctx context.Context) []Suggest {
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return nil
	}

	key := completionTableKey(tables)
	if key == s.completionTableKey && s.completionTableCols != nil {
		return s.completionTableCols
	}

	var out []Suggest
	for _, v := range tables {
		out = append(out, Suggest{
			Text:        v.Name(),
			Description: "table: " + v.Name(),
		})

		header, err := s.usecases.metadata.Header(ctx, v.Name())
		if err != nil {
			continue
		}
		for _, h := range header.Header() {
			out = append(out, Suggest{
				Text:        h,
				Description: "header: " + h + " column in " + v.Name() + " table",
			})
		}
	}

	s.completionTableKey = key
	s.completionTableCols = out
	return out
}

// completionTableKey builds a fingerprint of the table-name set used to decide
// whether the cached completion suggestions are still valid.
func completionTableKey(tables []*model.Table) string {
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name())
	}
	return strings.Join(names, "\x00")
}

// invalidateCompletionCache drops the cached table/column completion
// suggestions, forcing the next completion to rebuild them. It is called after an
// import, which can change a table's columns without changing the table-name set.
func (s *Shell) invalidateCompletionCache() {
	s.completionTableKey = ""
	s.completionTableCols = nil
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

// runScript runs a multi-statement script that prints to stdout.
//
// How many result sets the script may produce depends on the output format. A
// format a person reads carries several, separated as they are printed. A format
// a program parses carries exactly one, because there is no way to say where one
// result ends and the next begins — so those results are collected instead of
// printed, and a script that produced more than one is rejected with nothing on
// stdout. A script that produces none (only DDL/DML) is fine either way.
func (s *Shell) runScript(ctx context.Context, script string) (bool, error) {
	if s.state.mode.AllowsMultipleResults() {
		return s.runBatchReader(ctx, strings.NewReader(script))
	}

	s.capturedRowsets = nil
	s.collectingOutput = true
	defer func() {
		s.collectingOutput = false
		s.capturedRowsets = nil
	}()

	ranAny, err := s.runBatchReader(ctx, strings.NewReader(script))
	if err != nil {
		return ranAny, err
	}
	if len(s.capturedRowsets) > 1 {
		return ranAny, fmt.Errorf(
			"--output-format %s carries one result set, but the script produced %d; %s",
			s.state.mode, len(s.capturedRowsets), multiResultAdvice)
	}
	for _, table := range s.capturedRowsets {
		if err := table.Print(config.Stdout, s.state.mode.PrintMode); err != nil {
			return ranAny, fmt.Errorf("failed to print table: %w", err)
		}
	}
	return ranAny, nil
}

// multiResultAdvice is the recovery half of every "one result set" error, shared
// so --output and the machine-readable stdout formats say the same thing.
const multiResultAdvice = "keep one statement that returns rows, or use --output-format table, vertical, or markdown, which separate several results"

// runSQLFileToOutput runs a --sql-file script and exports its single result set
// to --output. The script may run any number of setup statements (DDL/DML), but
// exactly one must produce a result set: zero or more than one is rejected with a
// clear error, matching the one-file/one-result contract of --sql --output.
// Rowset results are captured rather than printed, so a successful run leaves
// stdout clean and writes only to the output file.
func (s *Shell) runSQLFileToOutput(ctx context.Context, script string) error {
	// Start from a clean slate and clear on the way out, so a reused Shell never
	// counts rowsets captured by an earlier run.
	s.capturedRowsets = nil
	s.collectingOutput = true
	defer func() {
		s.collectingOutput = false
		s.capturedRowsets = nil
	}()

	if _, err := s.runBatchReader(ctx, strings.NewReader(script)); err != nil {
		return err
	}

	switch len(s.capturedRowsets) {
	case 0:
		return errors.New("--output requires the --sql-file script to produce one result set, but it produced none; add a statement that returns rows (for example a SELECT)")
	case 1:
		if err := s.outputToFile(s.capturedRowsets[0]); err != nil {
			return err
		}
		return s.finishNonInteractive(ctx)
	default:
		return fmt.Errorf("--output writes one file, but the script produced %d result sets; %s", len(s.capturedRowsets), multiResultAdvice)
	}
}

// execSQL execute SQL query.
func (s *Shell) execSQL(ctx context.Context, req string) error {
	req = strings.TrimRight(req, ";")
	table, affectedRows, err := s.usecases.query.ExecSQL(ctx, req)
	if err != nil {
		return err
	}
	// Track whether this statement actually changed data, so write-back runs only
	// for a run that modified a table (not an EXPLAIN or a zero-row DML).,
	//,,.
	if statementModifiesData(req) {
		if table != nil {
			if table.RowCount() > 0 {
				s.dataChanged = true
			}
		} else if affectedRows > 0 {
			s.dataChanged = true
		}
	}
	if table == nil {
		// While collecting a --sql-file script's output, a no-rowset statement
		// (DDL, DML, PRAGMA) is a legitimate setup step. Stay silent so stdout
		// holds nothing but the exported data; the one-result-set contract is
		// checked after the whole script runs.
		if s.collectingOutput {
			return nil
		}
		// --output is only meaningful for a statement that produces a rowset. An
		// INSERT/UPDATE/DELETE without RETURNING produces only an affected-row
		// count, so reject --output instead of silently ignoring it.
		if s.argument.NeedsOutputToFile() {
			return errors.New("--output requires a statement that returns rows; an INSERT/UPDATE/DELETE without RETURNING produces none")
		}
		msg := statementResultMessage(req, affectedRows)
		// In a non-interactive run the count is buffered rather than printed now: a
		// later statement (or a .save) can still fail the run, and stdout must not
		// carry success text from a run that exits non-zero. finishNonInteractive
		// flushes it once the run has succeeded.
		if s.deferAffectedCounts {
			s.pendingAffected = append(s.pendingAffected, msg)
			return nil
		}
		fmt.Fprint(config.Stdout, msg)
		return nil
	}

	// While collecting a --sql-file script's output, capture each rowset instead
	// of printing it. The script's single result set is exported after the run.
	if s.collectingOutput {
		s.capturedRowsets = append(s.capturedRowsets, table)
		return nil
	}

	// use --sql option and user want to output table data to file.
	if s.argument.NeedsOutputToFile() {
		return s.outputToFile(table)
	}
	// Separate this result from the one before it. Two Markdown tables with no
	// blank line between them render as one broken table, and two ASCII tables or
	// vertical blocks read as one run-on block. Only a format that allows several
	// results reaches here more than once.
	if s.printedResults > 0 {
		fmt.Fprintln(config.Stdout)
	}
	if err := table.Print(config.Stdout, s.state.mode.PrintMode); err != nil {
		return fmt.Errorf("failed to print table: %w", err)
	}
	s.printedResults++
	return nil
}

// outputToFile output table data to file. The export format and compression are
// resolved from both the chosen output mode and the destination path, so a path
// like "result.parquet" or "out.ndjson.gz" is honored even without a mode flag.
func (s *Shell) outputToFile(table *model.Table) error {
	// ACH and Fedwire are input-only formats: sqly can read them but cannot
	// produce them, so reject such a destination instead of silently writing CSV
	// bytes to a misleading .ach/.fed path.
	if model.IsInputOnlyExtension(s.argument.Output.FilePath) {
		return fmt.Errorf("--output destination %q uses an input-only format (ACH/Fedwire); export to csv/tsv/json/parquet instead", s.argument.Output.FilePath)
	}
	mode := s.state.mode.PrintMode
	explicit := model.ExportFormatFromPrintMode(mode)
	exportFmt, compression, err := model.ResolveOutputTarget(s.argument.Output.FilePath, explicit, !mode.IsDisplayOnly())
	if err != nil {
		return err
	}
	filePath := model.BuildOutputPath(s.argument.Output.FilePath, exportFmt, compression)
	// Refuse an --output destination that aliases an imported source file. A
	// destructive source write must go through .save --in-place, not a one-off
	// export, so a stray --output cannot silently destroy the dataset.
	if name, aliased := s.outputAliasesImportedSource(filePath); aliased {
		return fmt.Errorf("--output destination %s is the source file for table %q; use .save --in-place to overwrite a source", filePath, name)
	}
	if err := s.usecases.export.DumpTable(filePath, table, exportFmt, compression); err != nil {
		return err
	}
	// Status for a file-output operation is control-plane information; the data
	// went to the file, so keep stdout empty and report progress on stderr.
	fmt.Fprintf(config.Stderr, "Output sql result to %s (output mode=%s)\n",
		color.HiCyanString(filePath), exportFmt.String())
	return nil
}

// outputAliasesImportedSource reports whether path resolves to a file that an
// imported table was loaded from, returning that table name. It lets --output
// reject a destination that would overwrite a source dataset. Tables staged from
// --stdin-format have no real source file and are skipped.
func (s *Shell) outputAliasesImportedSource(path string) (string, bool) {
	for table, src := range s.tableSources {
		if src == stdinTableSource {
			continue
		}
		if sameFilePath(path, src) {
			return table, true
		}
	}
	return "", false
}

// prepareForScript records what the whole script implies before its first
// statement runs: whether a write-back is coming, which decides if the
// affected-row counts can be printed as they happen.
func (s *Shell) prepareForScript(ctx context.Context, script string) error {
	s.deferAffectedCounts = scriptSaves(script)
	return s.preflightSave(ctx, script)
}

// ensureWritableDestination rejects an output destination sqly cannot write a
// single file to. Without this check the path gets a format extension appended,
// silently writing to a sibling file ("out" -> "out.csv") or, for a
// directory-like path ending in a separator, a hidden file ("outdir/" ->
// "outdir/.csv"). A path whose parent directory does not exist is rejected too,
// rather than failing after the query has already run. A plain non-existent file
// under an existing directory is fine; it is created on write.
func ensureWritableDestination(path string) error {
	if path == "" {
		return nil
	}
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(os.PathSeparator)) {
		return fmt.Errorf("output destination %q ends with a path separator; specify a file path, not a directory", path)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("output destination %q is a directory; specify a file path", path)
	}
	// The parent must already exist. sqly does not create directories for an
	// output path: a typo in a directory name would otherwise leave a tree of
	// empty directories behind. Checking here means the run stops before the
	// import instead of after the query has run.
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("output destination %q: directory %q does not exist", path, parent)
	}
	if !info.IsDir() {
		return fmt.Errorf("output destination %q: %q is not a directory", path, parent)
	}
	return nil
}

// recordUserRequest record user request in DB.
//
// The row id is left to SQLite's AUTOINCREMENT instead of being computed from a
// full history scan, so each write costs a single insert rather than reading the
// whole table first.
func (s *Shell) recordUserRequest(ctx context.Context, request string) error {
	if err := s.usecases.history.Create(ctx, model.NewHistory(0, request)); err != nil {
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

// splitCompletionPrefix splits a typed path prefix into the directory to scan,
// the leading text to keep on each suggestion (so completions preserve what the
// user typed), and the partial entry name to match. The prefix may carry
// backslash-escaped whitespace (for example "my\ dir/in"); the escaped base is
// returned verbatim so suggestions keep round-tripping, while the partial is
// matched after decoding.
//
// It accepts both "/" and "\" as separators so completion behaves the same on
// every OS. Examples (POSIX): "" -> ".", "", ""; "testdata/sa" -> "testdata/",
// "testdata/", "sa"; "testdata" -> ".", "", "testdata".
func splitCompletionPrefix(prefix string) (readDir, base, partial string) {
	idx := lastUnescapedSeparator(prefix)
	if idx < 0 {
		return ".", "", prefix
	}
	base = prefix[:idx+1]
	partial = prefix[idx+1:]
	readDir = base
	if readDir == "" {
		// A leading separator ("/foo") leaves base empty; scan the filesystem root.
		readDir = string(os.PathSeparator)
	}
	return readDir, base, partial
}

// currentCompletionWord returns the word at the end of input used to build
// completions. Whitespace that is backslash-escaped (preceded by an odd number of
// backslashes) is part of the word, so "my\ dir/in" is one word while an
// unescaped space ends it. It mirrors the prompt library's escaped word boundary.
func currentCompletionWord(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	last := len(runes) - 1
	if isUnescapedWhitespace(runes, last) {
		return ""
	}
	start := 0
	for i := last; i >= 0; i-- {
		if isUnescapedWhitespace(runes, i) {
			start = i + 1
			break
		}
	}
	return string(runes[start:])
}

// isUnescapedWhitespace reports whether runes[i] is a space, tab, or newline that
// is not backslash-escaped (an even number of backslashes precede it).
func isUnescapedWhitespace(runes []rune, i int) bool {
	if r := runes[i]; r != ' ' && r != '\t' && r != '\n' {
		return false
	}
	backslashes := 0
	for j := i - 1; j >= 0 && runes[j] == '\\'; j-- {
		backslashes++
	}
	return backslashes%2 == 0
}

// lastUnescapedSeparator returns the byte index of the last path separator in an
// escaped completion prefix, or -1. A backslash that escapes the following
// character (whitespace, quote, or backslash) is not a separator; a lone
// backslash is treated as a Windows separator. This keeps "my\ dir/in" splitting
// at the "/" rather than at the escaping backslash.
func lastUnescapedSeparator(prefix string) int {
	last := -1
	for i := 0; i < len(prefix); i++ {
		switch prefix[i] {
		case '\\':
			if i+1 < len(prefix) {
				switch prefix[i+1] {
				case ' ', '\t', '\\', '\'', '"':
					i++ // skip the escaped character
					continue
				}
			}
			last = i
		case '/':
			last = i
		}
	}
	return last
}

// unescapeCompletionPath reverses escapeCompletionPath, decoding a typed or
// accepted prefix back to a real path for filesystem lookups. It mirrors how
// splitArgs consumes a backslash: only before whitespace, a quote, or another
// backslash; a lone backslash stays literal (a Windows separator).
func unescapeCompletionPath(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			switch next := runes[i+1]; next {
			case ' ', '\t', '\\', '\'', '"':
				b.WriteRune(next)
				i++
				continue
			}
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// escapeCompletionPath backslash-escapes the characters that splitArgs treats as
// token boundaries or quoting syntax, so an accepted completion survives the
// re-tokenization that exec performs on the command line. Without this, a path
// such as "my data.csv" is inserted verbatim and splitArgs would split it into
// two arguments, both reported as missing files.
//
// Backslash escaping is used instead of wrapping the path in quotes because the
// prompt library accepts a suggestion by prefix-matching it against the typed
// word (strings.HasPrefix(suggestion, word)). A leading quote would break that
// match; an escape keeps the typed prefix intact.
func escapeCompletionPath(path string) string {
	var b strings.Builder
	for _, r := range path {
		switch r {
		case ' ', '\t', '\\', '\'', '"':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// slashifyBase normalizes the separators in a completion base to "/" without
// disturbing the backslashes that escape whitespace or quotes. filepath.ToSlash
// cannot be used here because on Windows it would rewrite an escape such as
// "my\ dir/" into "my/ dir/", breaking the round-trip; this converts only genuine
// separator backslashes.
func slashifyBase(base string) string {
	var b strings.Builder
	runes := []rune(base)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' {
			if i+1 < len(runes) {
				switch next := runes[i+1]; next {
				case ' ', '\t', '\\', '\'', '"':
					b.WriteRune('\\')
					b.WriteRune(next)
					i++
					continue
				}
			}
			b.WriteRune('/') // a lone backslash is a path separator
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// pathCommandSpec reports how a helper command participates in path completion:
// the 0-based argument index where the path is typed (the command itself is
// index 0), whether multiple trailing paths are accepted, and whether only
// directories are offered. ok is false for commands that take no path.
func pathCommandSpec(cmd string) (pathArg int, multi, dirsOnly, ok bool) {
	switch cmd {
	case importCommand: // .import FILE...
		return 1, true, false, true
	case lsCommand: // .ls PATH
		return 1, false, false, true
	case cdCommand: // .cd DIR
		return 1, false, true, true
	case saveCommand: // .save DIR
		return 1, false, true, true
	case dumpCommand: // .dump TABLE PATH
		return 2, false, false, true
	}
	return 0, false, false, false
}

// keepDirsOnly drops file suggestions when a command targets a directory (.cd,
// .save). Directory suggestions carry a trailing "/", so they are kept while
// files are filtered out. When dirsOnly is false the suggestions pass through.
func keepDirsOnly(suggestions []Suggest, dirsOnly bool) []Suggest {
	if !dirsOnly {
		return suggestions
	}
	filtered := make([]Suggest, 0, len(suggestions))
	for _, sg := range suggestions {
		if strings.HasSuffix(sg.Text, "/") {
			filtered = append(filtered, sg)
		}
	}
	return filtered
}

// completedCommandWords splits the already-typed portion of text (everything
// before the in-progress word) into shell-aware tokens, so a quoted or escaped
// earlier argument stays a single decoded token. It falls back to whitespace
// splitting only if the prefix cannot be tokenized (an unterminated quote in an
// earlier token), keeping completion functional rather than empty.
func completedCommandWords(text, currentWord string) []string {
	prefix := strings.TrimSuffix(text, currentWord)
	args, err := splitArgs(prefix)
	if err != nil {
		return strings.Fields(prefix)
	}
	return args
}

// getFilePathCompletions returns importable-file and directory suggestions
// scoped to the directory named by prefix. It reads only that directory rather
// than walking the whole working tree, so latency tracks the targeted subtree,
// not repository size. Directories are suggested with a trailing slash so the
// user can descend one level at a time, the same way a shell completes paths.
// Hidden entries are skipped unless the user types a leading dot.
func (s *Shell) getFilePathCompletions(prefix string) []Suggest {
	readDir, base, partial := splitCompletionPrefix(prefix)

	// readDir and partial come from the escaped prefix, so decode them before
	// touching the filesystem: an escaped space ("my\ dir/") names the real
	// directory "my dir/". base stays escaped so each suggestion round-trips
	// through splitArgs; a remaining backslash is a Windows separator mapped to "/".
	readDir = filepath.FromSlash(strings.ReplaceAll(unescapeCompletionPath(readDir), `\`, "/"))
	partial = unescapeCompletionPath(partial)

	// Expand a leading "~" so a home-directory prefix enumerates the real home
	// directory for the lookup. base keeps the typed "~/" so suggestions render as
	// "~/file.csv"; .import expands the tilde again at execution time.
	expandedReadDir, err := expandTilde(readDir)
	if err != nil {
		return nil
	}
	readDir = expandedReadDir

	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil
	}

	includeHidden := strings.HasPrefix(partial, ".")
	var suggestions []Suggest
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !includeHidden {
			continue
		}
		if !strings.HasPrefix(name, partial) {
			continue
		}

		// Escape only the entry name; base is the verbatim typed prefix the prompt
		// library prefix-matches, so escaping it would corrupt the match.
		if entry.IsDir() {
			suggestions = append(suggestions, Suggest{
				Text:        slashifyBase(base) + escapeCompletionPath(name) + "/",
				Description: msgImportableDir,
			})
			continue
		}
		if s.isValidFileForCompletion(name) {
			suggestions = append(suggestions, Suggest{
				Text:        slashifyBase(base) + escapeCompletionPath(name),
				Description: msgImportableFile,
			})
		}
	}
	return suggestions
}

// openQuotePrefix reports whether word begins a still-open quoted argument (a
// leading ' or ") with no matching closing quote yet. It returns the opening
// quote rune and the raw text typed after it. Inside a double quote, a \" or \\
// escape does not close the quote. A word whose quote is already closed is not
// an in-progress quoted path, so ok is false.
func openQuotePrefix(word string) (quote rune, rawInner string, ok bool) {
	runes := []rune(word)
	if len(runes) == 0 {
		return 0, "", false
	}
	q := runes[0]
	if q != '\'' && q != '"' {
		return 0, "", false
	}
	for i := 1; i < len(runes); i++ {
		if q == '"' && runes[i] == '\\' && i+1 < len(runes) {
			if next := runes[i+1]; next == '"' || next == '\\' {
				i++ // skip the escaped character
				continue
			}
		}
		if runes[i] == q {
			return 0, "", false // the quote is closed; not an in-progress quoted word
		}
	}
	return q, string(runes[1:]), true
}

// getQuotedFilePathCompletions returns importable-file and directory suggestions
// for a path fragment typed inside an open quote. Each suggestion keeps the same
// quote: a directory keeps the quote open (so the user descends one level at a
// time) while a file closes it, so the accepted command parses back to a single
// argument through splitArgs.
func (s *Shell) getQuotedFilePathCompletions(rawInner string, quote rune) []Suggest {
	inner := decodeQuotedPath(rawInner, quote)
	readDir, base, partial := splitDecodedPrefix(inner)

	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil
	}

	q := string(quote)
	includeHidden := strings.HasPrefix(partial, ".")
	var suggestions []Suggest
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !includeHidden {
			continue
		}
		if !strings.HasPrefix(name, partial) {
			continue
		}
		// Inside quotes the path is literal, so base and name need no escaping
		// (filenames containing the quote character itself are not handled).
		if entry.IsDir() {
			suggestions = append(suggestions, Suggest{
				Text:        q + base + name + "/",
				Description: msgImportableDir,
			})
			continue
		}
		if s.isValidFileForCompletion(name) {
			suggestions = append(suggestions, Suggest{
				Text:        q + base + name + q,
				Description: msgImportableFile,
			})
		}
	}
	return suggestions
}

// decodeQuotedPath decodes the raw text typed inside an open quote into the real
// path. Single-quoted content is literal; double-quoted content unescapes \" and
// \\, matching splitArgs.
func decodeQuotedPath(s string, quote rune) string {
	if quote != '"' {
		return s
	}
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			if next := runes[i+1]; next == '"' || next == '\\' {
				b.WriteRune(next)
				i++
				continue
			}
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// splitDecodedPrefix splits an already-decoded path fragment into the directory
// to scan, the literal prefix to keep on each suggestion, and the partial entry
// name to match. Both "/" and "\" are accepted as separators.
func splitDecodedPrefix(p string) (readDir, base, partial string) {
	idx := strings.LastIndexAny(p, `/\`)
	if idx < 0 {
		return ".", "", p
	}
	base = p[:idx+1]
	partial = p[idx+1:]
	readDir = filepath.FromSlash(strings.ReplaceAll(base, `\`, "/"))
	if readDir == "" {
		// A leading separator ("/foo") leaves base empty; scan the filesystem root.
		readDir = string(os.PathSeparator)
	}
	return readDir, base, partial
}
