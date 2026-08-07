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
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/prompt"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/sqltext"
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
	// rowMismatchFlag is the same choice spelled for the command line. Which of
	// the two a message offers depends on whether the process has already
	// started; see rowMismatchAdvice.
	rowMismatchFlag = "--row-mismatch"
	// encodingFlag names the startup flag that decodes a text input, which is the
	// only place the choice can be made: there is no shell command for it.
	encodingFlag    = "--encoding"
	cdCommand       = ".cd"
	clearCommand    = ".clear"
	dumpCommand     = ".dump"
	exitCommand     = ".exit"
	helpCommand     = ".help"
	lsCommand       = ".ls"
	modeCommand     = ".mode"
	tablesCommand   = ".tables"
	pwdCommand      = ".pwd"
	schemaCommand   = ".schema"
	describeCommand = ".describe"
	saveCommand     = ".save"
	dialectCommand  = ".dialect"
	helpFlag        = "--help"
	versionFlag     = "--version"
	helpArgument    = "help"

	msgImportableFile = "Importable file"
	msgImportableDir  = "Directory"
	msgExcelSheet     = "Excel sheet"
)

// historySize is how many past entries the prompt keeps for recall. The history
// file holds more than this, so it is also how many the preload hands over:
// giving the prompt the whole file only to have it keep the tail is work nobody
// sees.
const historySize = 100

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
	// importingStartupInputs is true while the inputs named on the command line
	// are being imported, and false for an .import typed into a running session.
	// It decides which spelling advice offers: a flag can only be passed when the
	// process starts, so a session already running is told about the helper
	// command instead.
	importingStartupInputs bool
	// excelWorkbooks records what each imported workbook held at the moment it
	// was imported: every sheet and whether the workbook showed it. It is kept
	// because --inspect reports the sheets that did not become tables, and by
	// then the file may be gone — a workbook fetched over HTTP is staged into a
	// temp directory the import cleans up.
	excelWorkbooks []excelWorkbookImport

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
	// an EXPLAIN or a zero-row DML leaves source files untouched.
	dataChanged bool
	// importBaseline maps an imported file-backed table name to a fingerprint of
	// its content as loaded, and never moves. It answers "has this session changed
	// this table?", which is what decides whether a `.save DIR` exports it: a
	// session that only touched a TEMP or scratch table, or that made net-zero
	// edits that cancel out, exports nothing. dataChanged is a coarse session-wide
	// gate; this map is the per-table truth that prevents a spurious write.
	importBaseline map[string]string
	// sourceBaseline is a fingerprint of what each table's *source file* holds. It
	// starts equal to importBaseline and moves forward when an in-place save
	// rewrites the source. It answers "does the source still need writing?", which
	// is a different question, and merging the two is what made
	// `UPDATE; .save out; .save --in-place` leave the source with its old rows:
	// the export moved a baseline that describes a file it never touched.
	sourceBaseline map[string]string
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
	// plan is what this invocation is: which mode, and where stdin goes. It is
	// decided once in Run, before anything is read.
	plan runPlan
	// stdinKindCached and stdinKindOnce memoize what standard input is attached
	// to. The answer cannot change during a run, and the probe is a syscall.
	stdinKindCached stdinKind
	stdinKindOnce   bool
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
	// allowRemote is this session's permission to download http(s) inputs, taken
	// from --allow-remote when the shell is built. It is session state rather
	// than a package-level flag so a capability granted to one invocation cannot
	// leak into another, and so an interactive session started with the flag
	// keeps it for the .import commands typed later. See remotepolicy.go.
	allowRemote bool
	// dialectWarned records that this session has already said a non-SQLite
	// dialect is translated rather than emulated. The warning is worth saying
	// once and is noise on every statement, and a session — not the process — is
	// what a user experiences, so it is a field here rather than a package
	// variable. Switching back to SQLite and out again does not re-arm it: the
	// user has been told what the translation means.
	dialectWarned bool
	// httpClient downloads remote datasets for HTTP/HTTPS imports. It is
	// overridable in tests so httptest servers can supply their own transport.
	httpClient *http.Client
	// maxDownloadBytes overrides the download size cap. Zero means the shipped
	// limit; only tests set it, so the limit can be exercised without moving two
	// gigabytes through the disk on every CI run.
	maxDownloadBytes int64
}

type promptSession interface {
	AddHistory(string)
	Close() error
	Run() (string, error)
	SetPrefix(string)
	// WatchInterrupt watches the terminal for Ctrl-C while a submission runs and
	// returns a context canceled when it arrives. Between prompts nothing else is
	// reading the terminal, and raw mode makes Ctrl-C a byte rather than a
	// signal, so this is the only way the key can reach a running statement.
	WatchInterrupt(context.Context) (context.Context, context.CancelFunc)
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
		allowRemote:    arg.AllowRemote,
		httpClient:     newRemoteClient(),
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
		return &invocationError{Err: errors.New(hint)}
	}

	// --inspect is self-contained; reject conflicting action/side-effect flags
	// up front instead of silently discarding them.
	if err := s.validateInspectFlags(); err != nil {
		return err
	}

	// --output is honored by --sql (a single result) and --sql-file (the script's
	// one result set). Without either (batch stdin or interactive) the flag was
	// silently ignored, so reject it instead of looking successful.
	if s.argument.Output.FilePath != "" && s.argument.ScriptFilePath != "" {
		// A script already has a way to write a file, and it is per-result rather
		// than per-run: .dump names the table and the destination at the point in
		// the script where it means something. One --output for a whole script
		// would have to pick one of its results, and nothing says which.
		return &invocationError{Err: errors.New("--output does not apply to --script-file; write from inside the script with .dump TABLE FILE")}
	}
	if s.argument.Output.FilePath != "" && s.argument.Query == "" && s.argument.SQLFilePath == "" {
		return &invocationError{Err: errors.New("--output requires --sql or --sql-file")}
	}

	// Check the destination before the import, so a run that cannot write its
	// result never spends time reading files: an existing directory would be
	// silently rewritten to a sibling file, and a missing parent directory would
	// only surface after the query had already run.
	if err := ensureWritableDestination(s.argument.Output.FilePath); err != nil {
		return err
	}

	// An --output-format that contradicts the destination's extension is decided
	// from the command line alone, so it is settled here rather than after the
	// import: a caller whose invocation cannot mean anything should not have to
	// read past import diagnostics for a message about two flags, and a missing
	// input must not change the answer from "fix the command line" to "fix the
	// file".
	if err := s.validateOutputFormatAgainstDestination(); err != nil {
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

	// Decide what this invocation is before reading anything: which mode, and
	// where stdin goes. Every later question about stdin and about what a script
	// may contain is answered from this one decision.
	plan, err := s.planRun()
	if err != nil {
		return err
	}
	s.plan = plan

	// Read and parse the script now, so a script that cannot run — a bad
	// boundary, or a helper command in a SQL file — fails before the import
	// spends time on files. The same parse is what executes below.
	elements, err := s.loadScript(ctx)
	if err != nil {
		return err
	}

	// A URL this session may not download is refused here, before the import and
	// before the first statement. Both halves are checked together — the inputs
	// on the command line and the .import lines of the script — so a run that
	// will be refused makes no HTTP request, imports nothing local, creates no
	// temporary directory, and writes nothing to stdout. See remotepolicy.go.
	if err := s.authorizeRemoteInputs(s.argument.FilePaths); err != nil {
		return err
	}
	if err := s.authorizeScriptRemoteInputs(elements); err != nil {
		return err
	}

	// A failed import is a failed start, interactive or not. An import loads
	// every input it was given or none of them, so there is no half-loaded
	// session left to hand someone: starting a shell here would open a prompt
	// onto an empty database while claiming the files were read.
	if err := s.init(ctx); err != nil {
		return err
	}

	switch s.plan.mode {
	case modeInspect:
		return s.runInspect(ctx)

	case modeInlineSQL, modeSQLFile:
		if err := s.prepareForScript(ctx, elements); err != nil {
			return err
		}
		// With --output, export the run's single result set to the file instead of
		// printing each statement's result.
		if s.argument.Output.FilePath != "" {
			return s.runSQLFileToOutput(ctx, elements)
		}
		ranAny, err := s.runScript(ctx, elements)
		if err != nil {
			return err
		}
		if !ranAny {
			return nil
		}
		return s.finishNonInteractive(ctx)

	case modeStdinScript, modeScriptFile:
		if err := s.prepareForScript(ctx, elements); err != nil {
			return err
		}
		ranAny, err := s.runScript(ctx, elements)
		if err != nil {
			return err
		}
		// A non-interactive run that executed nothing — empty or comment-only
		// stdin — is a silent no-op that still exits 0, so headless wrappers and CI
		// mistake it for a completed query. Surface a hint and fail instead. A
		// --script-file with nothing in it was already rejected when it was read,
		// which is why only the stdin case needs the hint.
		if !ranAny {
			return &invocationError{Err: errNoStatements}
		}
		return s.finishNonInteractive(ctx)

	default:
		// Start shell. The welcome banner is printed inside communicate, only after
		// the prompt session is created, so a terminal-backend failure (no usable
		// /dev/tty) reports a clear error instead of looking like the shell started
		// and then crashed right after the banner.
		return s.communicate(ctx)
	}
}

// loadScript reads and parses this run's statements, from wherever the mode says
// they come. The interactive shell and --inspect have no script; every other
// mode has exactly one, parsed once here and executed later from the same
// result.
func (s *Shell) loadScript(ctx context.Context) ([]scriptElement, error) {
	var (
		script string
		origin string
	)
	switch s.plan.mode {
	case modeInlineSQL:
		script, origin = s.argument.Query, flagSQL
	case modeSQLFile:
		loaded, err := readSQLFile(s.argument.SQLFilePath)
		if err != nil {
			// readSQLFile already classifies: a path it could not read is an input
			// failure, a file holding no statement is a script failure.
			return nil, err
		}
		script, origin = loaded, s.argument.SQLFilePath
	case modeScriptFile:
		loaded, err := readScriptFile(s.argument.ScriptFilePath)
		if err != nil {
			return nil, err
		}
		script, origin = loaded, s.argument.ScriptFilePath
	case modeStdinScript:
		data, err := readAllContext(ctx, s.stdin)
		if err != nil {
			return nil, &scriptSourceError{Err: fmt.Errorf("failed to read the script from stdin: %w", err)}
		}
		script, origin = string(data), stdinTableSource
	default:
		return nil, nil
	}

	elements, err := parseScript(script)
	if err != nil {
		return nil, &scriptError{Err: fmt.Errorf("%s: %w", origin, err)}
	}

	// A SQL file holds SQL. Helper commands are the shell's language, and a
	// script that wants them belongs on stdin, where the name of the thing being
	// read does not promise otherwise — and where a destructive .save cannot
	// arrive inside a file someone believed was a query.
	if !s.plan.mode.allowsHelperCommands() {
		if helper, found := firstHelper(elements); found {
			return nil, &scriptError{Err: fmt.Errorf(
				"%s runs SQL only, but line %d is the helper command %q; run it with --script-file, or pipe it to sqly",
				origin, helper.startLine, helper.commandName())}
		}
	}

	// --sql runs one statement. Two would mean printing one result and dropping
	// the other, and none would mean exiting 0 having done nothing, which is
	// indistinguishable from a run that worked. Both are checked against the same
	// parse that would run it.
	//
	// An empty --sql is already refused by the flag parser, which sees the string.
	// This is what is left after parsing: whitespace, a lone semicolon, or a
	// comment. --sql-file rejects the same content for the same reason.
	if s.plan.mode == modeInlineSQL {
		if len(elements) == 0 {
			return nil, &invocationError{Err: errors.New(
				"--sql contains no executable SQL statement; it is blank, a comment, or a bare semicolon")}
		}
		if len(elements) > 1 {
			return nil, &invocationError{Err: fmt.Errorf(
				"--sql accepts a single SQL statement, but got %d; run one statement per invocation, or use --sql-file for a script",
				len(elements))}
		}
	}
	return elements, nil
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
			// Ctrl-C throws away the line being typed and nothing else, the way
			// every other SQL shell answers it. Reporting it as a failure ended the
			// session instead: a half-typed query took the shell down with it and
			// exited non-zero. The prompt has already echoed "^C".
			if errors.Is(err, prompt.ErrInterrupted) {
				continue
			}
			return err
		}
		if err := s.execWatchingForInterrupt(ctx, p, input); err != nil {
			if errors.Is(err, ErrExitSqly) {
				return nil // user input ".exit"
			}
			fmt.Fprintf(config.Stderr, "%v\n", err)
			continue
		}
	}
}

// execWatchingForInterrupt runs one submission with Ctrl-C watched for, so a
// statement that turns out to be slower than expected can be stopped.
//
// Nothing reads the terminal between prompts, and the prompt holds it in raw
// mode, where Ctrl-C is a byte rather than a signal: the key could not reach a
// running statement at all. It waited in the input buffer, the statement ran to
// completion however long it took, and the byte was then read as the next line.
//
// A canceled statement is not a failed one. It is reported and the loop goes on,
// so the session survives it the same way it survives an interrupt at the prompt.
// Whether it was canceled is asked of the watch's context rather than of the
// error, because the error crosses layers that render their cause as text, and a
// string comparison is not a way to know what happened.
func (s *Shell) execWatchingForInterrupt(ctx context.Context, p promptSession, input string) error {
	runCtx, stopWatch := p.WatchInterrupt(ctx)
	defer stopWatch()

	err := s.execInteractive(runCtx, input)
	if err == nil {
		return nil
	}
	// A session context that is already done means the shell itself is stopping
	// (a signal, a closed parent), which is not the user canceling a statement.
	if runCtx.Err() != nil && ctx.Err() == nil {
		fmt.Fprintf(config.Stderr, "^C\ncanceled: %s\n", previewStatement(input))
		return nil
	}
	return err
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
			// Only the newest historySize entries are offered. The file keeps far
			// more than the prompt does, and handing over all of them just to have
			// the prompt drop all but the tail is work with no effect on what a
			// user can recall.
			entries := histories.ToStringList()
			if len(entries) > historySize {
				entries = entries[len(entries)-historySize:]
			}
			for _, h := range entries {
				p.AddHistory(h)
			}
		}
	}

	return p, nil
}

// disableHistory turns off history persistence for the rest of the session and
// warns once. It is called when the history file cannot be created at startup or
// a later read/write fails (e.g. the file became read-only), so history stays
// best-effort and never aborts the requested --sql, --inspect, or batch command.
func (s *Shell) disableHistory(err error) {
	if !s.historyEnabled {
		return
	}
	s.historyEnabled = false
	fmt.Fprintf(config.Stderr, "warning: command history disabled (%v). Set SQLY_HISTORY_PATH to a writable path to enable it.\n", err)
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
		return &invocationError{Err: fmt.Errorf("--output-format %s writes a binary file and cannot be printed; add --output FILE",
			s.argument.Output.Mode)}
	default:
		return nil
	}
}

// validateOutputFormatAgainstDestination rejects an --output-format that
// contradicts the extension of the --output path.
//
// The check runs before the import because both halves of it are on the command
// line: no file changes the answer, so a run this rejects should read nothing.
// Only the conflict is reported here. What else the resolution can reject —
// compression a format cannot carry, stacked codecs — describes the destination
// rather than the invocation, and is left to the write that would produce it.
func (s *Shell) validateOutputFormatAgainstDestination() error {
	if s.argument.Output.FilePath == "" {
		return nil
	}
	mode := s.state.mode.PrintMode
	_, _, err := resolveOutputTarget(s.argument.Output.FilePath, model.ExportFormatFromPrintMode(mode), !mode.IsDisplayOnly())
	var invocationErr *invocationError
	if errors.As(err, &invocationErr) {
		return err
	}
	return nil
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
	// The Excel sheet policy is set the same way and for the same reason, and it
	// is set once for the session rather than per import: a shell started with
	// --include-hidden-sheets keeps that policy for every later .import, so what
	// the flag means does not change halfway through a session.
	s.usecases.importer.SetIncludeHiddenSheets(s.state.includeHiddenSheets)

	// History is best-effort: a read-only or unwritable history DB (CI,
	// sandboxes, containers) must not block the requested query or command.
	// Disable history for the session and warn instead of failing.
	if err := s.usecases.history.Init(ctx); err != nil {
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
			return &invocationError{Err: errors.New("--stdin-format requires piped or redirected stdin")}
		}
		stdinPath, cleanup, err := s.stageStdinDataset(ctx)
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
	s.importingStartupInputs = true
	importErr := s.commands.importCommand(ctx, s, paths)
	s.importingStartupInputs = false
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

// stageStdinDataset reads all of stdin into a temporary file named after the
// stdin table so filesql imports it like a normal file. Why a temp file:
// filesql loads by path, and staging keeps the import path identical to file
// arguments (including table naming and joins). The returned cleanup removes the
// temp directory; it is safe to call after import because the data is already
// copied into the shared database.
func (s *Shell) stageStdinDataset(ctx context.Context) (string, func(), error) {
	ext, ok := model.StdinFormatExtension(s.argument.StdinFormat)
	if !ok {
		return "", nil, fmt.Errorf("unsupported --stdin-format value %q: want %s", s.argument.StdinFormat, model.StdinFormatNames())
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
	// Cancellation-aware for the same reason the script read is: a piped dataset
	// whose writer never closes would otherwise ignore the interrupt entirely.
	data, err := readAllContext(ctx, s.stdin)
	if err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("read stdin data: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("write stdin staging file: %w", err)
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
	return fmt.Sprintf("sqly:%s(%s)$ ", s.state.shortCWD(), s.state.mode.String())
}

// sqlInputComplete reports whether the interactive buffer holds a statement
// ready to run, so the prompt submits on Enter instead of continuing on a new
// line. Without this, every newline submits, splitting a pasted or typed
// multi-line statement into separate executions.
//
// SQL is complete when a ";" has closed a statement and nothing executable
// follows it. A dot-command (".tables", ".import", ...) and an empty buffer also
// submit. Pressing Enter on a blank continuation line force-submits whatever is
// buffered, so a query typed without a trailing ";" still runs without forcing
// the user to add one.
func sqlInputComplete(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, ".") {
		return true
	}
	if endsAtStatementBoundary(input) {
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

// endsAtStatementBoundary reports whether input ends where a statement ended: a
// ";" has closed at least one statement and only comments and whitespace follow
// it.
//
// Asking whether the text ends with ";" was not the same question. A ";" is a
// terminator only where SQLite treats it as one, so that test submitted a
// fragment for every ";" inside a string literal ("SELECT 'a;") or an
// identifier, where it is ordinary text, and for every ";" inside a trigger
// body, where it separates the body's statements without ending the CREATE
// TRIGGER holding them — and it refused to submit a finished statement carrying
// a trailing comment ("SELECT 1; -- note"), leaving the shell waiting for
// nothing.
// The batch reader has always answered this with the same scanner, which is why
// a script could do what could not be typed.
func endsAtStatementBoundary(input string) bool {
	if sqltext.EndsInsideBlockComment(input) {
		return false
	}
	// The remainder is what follows the last ";" that terminated a statement, so
	// it is shorter than the input exactly when one did. Counting terminators
	// rather than statements is what makes a bare ";" a complete (empty) input
	// instead of a buffer the prompt would wait on forever.
	_, remainder := splitSQLStatements(input)
	return len(remainder) < len(input) && sqltext.StripNoise(remainder) == ""
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
		// The commands whose argument is a value rather than a path: the completion
		// offers that command's values and nothing else. See argumentCompletions.
		if suggestions, ok := s.argumentCompletions(ctx, completed); ok {
			return filterHasPrefix(suggestions, currentWord)
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
			return filterHasPrefix(fileCompletions, slashifyBase(currentWord))
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
					return filterHasPrefix(regularCompletions, slashifyBase(currentWord))
				}
			}
		}
	}

	// Default to regular completions
	return s.getRegularCompletions(ctx, input)
}

// filterHasPrefix filters suggestions that have the given prefix. The match is
// case-insensitive because the SQL keywords among them are offered upper-cased
// and nobody types them that way.
func filterHasPrefix(suggestions []Suggest, prefix string) []Suggest {
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
	}

	// A command's argument values — dialects, output formats, row-mismatch
	// policies — are offered at that command's argument position and nowhere
	// else; see argumentCompletions. Listing them here made ".dialect m" answer
	// "mysql, markdown" and ".mode m" the same pair, each half wrong.
	suggest = append(suggest, s.commandSuggestions()...)

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
	return filterHasPrefix(suggest, currentWord)
}

// outputFormatDescription is what completion says a format does. Most say only
// their own name, so the default is derived from it and a format added to the
// registry needs nothing here; the three whose name does not describe what
// happens say more.
func outputFormatDescription(mode model.PrintMode) string {
	switch mode {
	case model.PrintModeMarkdownTable:
		return "markdown table output format"
	case model.PrintModeJSONL:
		return "jsonl (newline-delimited JSON) output format"
	case model.PrintModeParquet:
		return "parquet export format"
	default:
		return mode.String() + " output format"
	}
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
//
// Only a dot-command line is split by shell quoting rules. SQL is handed to the
// engine as typed: shell quoting is not SQL quoting, and running the two
// together made a comment's apostrophe — "SELECT 1 -- don't panic" — an
// unterminated quote that stopped a valid statement before it reached SQLite.
func (s *Shell) exec(ctx context.Context, request string) error {
	req := strings.TrimSpace(request)
	if req == "" {
		return nil // user only input enter, space tab
	}
	s.recordHistory(ctx, req)
	return s.dispatch(ctx, req)
}

// execInteractive runs one submission from the interactive prompt: everything
// typed or pasted before Enter, which is not always one statement.
//
// The line is parsed the way a script is, so every statement in it runs and
// every result is printed. Handing the whole line to the engine as a single
// query ran the statements but kept one result, so pasting a snippet — the usual
// way more than one statement is entered — printed the last table and silently
// dropped the rest, and a line whose last statement was a SELECT could print no
// rows at all.
//
// History records the line as it was typed rather than the statements it was
// split into, so the up-arrow returns what the user wrote.
func (s *Shell) execInteractive(ctx context.Context, input string) error {
	req := strings.TrimSpace(input)
	if req == "" {
		return nil // user only input enter, space tab
	}
	s.recordHistory(ctx, req)

	// A helper command is one line by definition and is never SQL, so a single
	// line beginning with "." goes straight to the dispatcher rather than through
	// the statement scanner. A submission of several lines is parsed even when it
	// opens with one, because the lines after it are their own elements.
	if looksLikeCommand(req) && !strings.ContainsAny(req, "\r\n") {
		return s.dispatch(ctx, req)
	}

	elements, err := parseScript(input)
	if err != nil {
		return err
	}
	for _, element := range elements {
		if err := s.dispatch(ctx, element.text); err != nil {
			return err
		}
	}
	return nil
}

// recordHistory stores what the user asked for, before the line is parsed, so an
// input rejected for its syntax is still recallable with the up-arrow.
//
// Skip history persistence when it is disabled so a read-only history DB cannot
// fail the requested command. History is best-effort: a runtime failure after
// startup disables history for the rest of the session and warns, instead of
// aborting the command.
func (s *Shell) recordHistory(ctx context.Context, req string) {
	if !s.historyEnabled {
		return
	}
	if err := s.recordUserRequest(ctx, req); err != nil {
		s.disableHistory(err)
	}
}

// dispatch runs one already-recorded helper command or SQL statement.
func (s *Shell) dispatch(ctx context.Context, req string) error {
	if looksLikeCommand(req) {
		argv, err := splitArgs(req)
		if err != nil {
			return err
		}
		if len(argv) == 0 || argv[0] == "" {
			return nil
		}
		if s.commands.hasCmd(argv[0]) {
			return s.commands[argv[0]].execute(ctx, s, argv[1:])
		}
		return errors.New("no such sqly command: " + color.CyanString(req))
	}

	return s.execSQL(ctx, req)
}

// runScript runs a multi-statement script that prints to stdout.
//
// How many result sets the script may produce depends on the output format. A
// format a person reads carries several, separated as they are printed. A format
// a program parses carries exactly one, because there is no way to say where one
// result ends and the next begins — so those results are collected instead of
// printed, and a script that produced more than one is rejected with nothing on
// stdout. A script that produces none (only DDL/DML) is fine either way.
func (s *Shell) runScript(ctx context.Context, elements []scriptElement) (bool, error) {
	if s.state.mode.AllowsMultipleResults() {
		return s.runScriptElements(ctx, elements)
	}

	s.capturedRowsets = nil
	s.collectingOutput = true
	defer func() {
		s.collectingOutput = false
		s.capturedRowsets = nil
	}()

	ranAny, err := s.runScriptElements(ctx, elements)
	if err != nil {
		return ranAny, err
	}
	if len(s.capturedRowsets) > 1 {
		return ranAny, &resultCountError{Produced: len(s.capturedRowsets), Err: fmt.Errorf(
			"--output-format %s carries one result set, but the script produced %d; %s",
			s.state.mode, len(s.capturedRowsets), multiResultAdvice)}
	}
	for _, table := range s.capturedRowsets {
		if err := printResultTable(table, s.state.mode.PrintMode); err != nil {
			return ranAny, err
		}
	}
	return ranAny, nil
}

// multiResultAdvice is the recovery half of the "one result set" error on the
// machine-readable stdout formats, where changing the format is a way out.
const multiResultAdvice = "keep one statement that returns rows, or use --output-format table, vertical, or markdown, which separate several results"

// multiResultToFileAdvice is the same half for --output, where changing the
// format is not a way out: one file holds one result whichever format it is in.
const multiResultToFileAdvice = "keep one statement that returns rows, or drop --output and let the results print, in a format that separates them (table, vertical, markdown)"

// runSQLFileToOutput runs a --sql-file script and exports its single result set
// to --output. The script may run any number of setup statements (DDL/DML), but
// exactly one must produce a result set: zero or more than one is rejected with a
// clear error, matching the one-file/one-result contract of --sql --output.
// Rowset results are captured rather than printed, so a successful run leaves
// stdout clean and writes only to the output file.
func (s *Shell) runSQLFileToOutput(ctx context.Context, elements []scriptElement) error {
	// Start from a clean slate and clear on the way out, so a reused Shell never
	// counts rowsets captured by an earlier run.
	s.capturedRowsets = nil
	s.collectingOutput = true
	defer func() {
		s.collectingOutput = false
		s.capturedRowsets = nil
	}()

	if _, err := s.runScriptElements(ctx, elements); err != nil {
		return err
	}

	switch len(s.capturedRowsets) {
	case 0:
		return &resultCountError{Produced: 0, Err: errors.New(
			"--output requires the --sql-file script to produce one result set, but it produced none; add a statement that returns rows (for example a SELECT)")}
	case 1:
		if err := s.outputToFile(s.capturedRowsets[0]); err != nil {
			return err
		}
		return s.finishNonInteractive(ctx)
	default:
		return &resultCountError{Produced: len(s.capturedRowsets), Err: fmt.Errorf(
			"--output writes one file, but the script produced %d result sets; %s", len(s.capturedRowsets), multiResultToFileAdvice)}
	}
}

// execSQL execute SQL query.
func (s *Shell) execSQL(ctx context.Context, req string) error {
	// A non-SQLite dialect is announced here, at the first statement it applies
	// to, rather than at startup. Announced at startup it reached runs that never
	// translate anything — a script of nothing but dot-commands — and in the
	// interactive shell it printed above sqly's own banner. See
	// warnDialectTranslationOnce.
	s.warnDialectTranslationOnce(s.usecases.query.Dialect())
	req = strings.TrimRight(req, ";")
	table, affectedRows, err := s.usecases.query.ExecSQL(ctx, req)
	if err != nil {
		return s.withMissingNameHint(ctx, err)
	}
	// Track whether this statement actually changed data, so write-back runs only
	// for a run that modified a table (not an EXPLAIN or a zero-row DML).
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
		// While a run's output is being collected — an --output export, or a
		// machine-readable format, which carries one result and nothing else — a
		// no-rowset statement (DDL, DML, PRAGMA) is a legitimate step whose status
		// line must not reach stdout: stdout holds the data a program parses. It
		// goes to stderr instead of being dropped, so "how many rows did that
		// UPDATE change" has an answer in every format rather than only in the
		// ones a person reads.
		if s.collectingOutput {
			fmt.Fprint(config.Stderr, statementResultMessage(req, affectedRows))
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
	if err := printResultTable(table, s.state.mode.PrintMode); err != nil {
		return err
	}
	s.printedResults++
	return nil
}

// stdoutDestination is the Path an outputPathError carries when the destination
// was stdout rather than a file, so a caller reading Path always finds where the
// result was going.
const stdoutDestination = "stdout"

// printResultTable writes table to stdout and reports a rendering failure as an
// output failure.
//
// A format that cannot represent a value (a tab inside an LTSV field) or a
// column set (two columns of the same name in JSON) is not a SQL error: the
// statement ran and produced the rows it was asked for. What has to change is
// the chosen --output-format or where the result goes, which is what the output
// class tells a caller. Reporting it as a failed statement sent them back to the
// query instead.
func printResultTable(table *model.Table, mode model.PrintMode) error {
	if err := table.Print(config.Stdout, mode); err != nil {
		return &outputPathError{Path: stdoutDestination, Err: fmt.Errorf("failed to print table: %w", err)}
	}
	return nil
}

// resolveOutputTarget resolves path against the run's format and gives each of
// ResolveOutputTarget's refusals its class.
//
// A format conflict is a contradiction between two things the user typed — an
// output mode and a destination extension — so nothing about the data can settle
// it and the fix is on the command line or in the script.
//
// Compression the destination cannot carry describes the file rather than the
// command line, so it is a destination failure. It used to fall through
// unclassified and exit 1, which is the code for a statement that ran and
// failed: nothing had run, and a wrapper reading 1 was told its SQL was wrong
// when what it needed to change was the path.
func resolveOutputTarget(path string, explicit model.ExportFormat, explicitSet bool) (model.ExportFormat, model.Compression, error) {
	format, compression, err := model.ResolveOutputTarget(path, explicit, explicitSet)
	switch {
	case errors.Is(err, model.ErrOutputFormatConflict):
		return 0, model.CompressionNone, &invocationError{Err: err}
	case errors.Is(err, model.ErrCompressionUnsupported):
		return 0, model.CompressionNone, &outputPathError{Path: path, Err: err}
	}
	return format, compression, err
}

// outputToFile output table data to file. The export format and compression are
// resolved from both the chosen output mode and the destination path, so a path
// like "result.parquet" or "out.ndjson.gz" is honored even without a mode flag.
func (s *Shell) outputToFile(table *model.Table) error {
	// ACH and Fedwire are input-only formats: sqly can read them but cannot
	// produce them, so reject such a destination instead of silently writing CSV
	// bytes to a misleading .ach/.fed path.
	if model.IsInputOnlyExtension(s.argument.Output.FilePath) {
		return &outputPathError{Path: s.argument.Output.FilePath, Err: fmt.Errorf("--output destination %q uses an input-only format (ACH/Fedwire); export to csv/tsv/json/parquet instead", s.argument.Output.FilePath)}
	}
	mode := s.state.mode.PrintMode
	explicit := model.ExportFormatFromPrintMode(mode)
	exportFmt, compression, err := resolveOutputTarget(s.argument.Output.FilePath, explicit, !mode.IsDisplayOnly())
	if err != nil {
		return err
	}
	filePath := model.BuildOutputPath(s.argument.Output.FilePath, exportFmt, compression)
	// Refuse an --output destination that aliases an imported source file. A
	// destructive source write must go through .save --in-place, not a one-off
	// export, so a stray --output cannot silently destroy the dataset.
	if name, aliased := s.outputAliasesImportedSource(filePath); aliased {
		return &outputPathError{Path: filePath, Err: fmt.Errorf("--output destination %s is the source file for table %q; use .save --in-place to overwrite a source", filePath, name)}
	}
	// The result is serialized beside the destination and moved onto it, so a
	// format that rejects a value part-way — or a full disk — leaves an existing
	// file whole rather than truncated. .save writes through the same steps.
	//
	// Everything this can fail at is about the destination: a value the format
	// cannot hold, a staging file that could not be written, a commit that did
	// not land. The wrapper carries no text of its own, so the message stays the
	// one the failing step produced.
	if err := s.writeFileAtomically(filePath, func(staging string) error {
		return s.usecases.export.DumpTable(staging, table, exportFmt, compression)
	}); err != nil {
		return &outputPathError{Path: filePath, Err: err}
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
func (s *Shell) prepareForScript(_ context.Context, elements []scriptElement) error {
	s.deferAffectedCounts = runsHelper(elements, saveCommand)
	return s.preflightSave(elements)
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
		return &outputPathError{Path: path, Err: fmt.Errorf("output destination %q ends with a path separator; specify a file path, not a directory", path)}
	}
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return &outputPathError{Path: path, Err: fmt.Errorf("output destination %q is a directory; specify a file path", path)}
		}
		// A FIFO, a device, or a socket is not a file that can be replaced. The
		// write stages a scratch file beside the destination and renames it into
		// place, and a rename replaces the name: pointed at a named pipe it
		// unlinked the pipe, left a regular file where it had been, and reported
		// success while the reader waiting on the other end received nothing.
		// Pointed at /dev/null it tried to create a scratch file in /dev and
		// failed with a permission error naming a path the user never wrote.
		//
		// Streams are not what --output is for; stdout is. So say that, and leave
		// what is there alone.
		if !info.Mode().IsRegular() {
			return &outputPathError{Path: path, Err: fmt.Errorf(
				"output destination %q is a %s, not a regular file; --output replaces a file, so write to stdout and redirect instead",
				path, describeFileMode(info.Mode()))}
		}
	}
	// The parent must already exist. sqly does not create directories for an
	// output path: a typo in a directory name would otherwise leave a tree of
	// empty directories behind. Checking here means the run stops before the
	// import instead of after the query has run.
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return &outputPathError{Path: path, Err: fmt.Errorf("output destination %q: directory %q does not exist", path, parent)}
	}
	if !info.IsDir() {
		return &outputPathError{Path: path, Err: fmt.Errorf("output destination %q: %q is not a directory", path, parent)}
	}
	return nil
}

// describeFileMode names what a non-regular destination is, so the refusal says
// which kind of thing is in the way rather than "not a regular file".
func describeFileMode(mode os.FileMode) string {
	switch {
	case mode&os.ModeNamedPipe != 0:
		return "named pipe"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode&os.ModeCharDevice != 0:
		return "character device"
	case mode&os.ModeDevice != 0:
		return "device"
	default:
		return "special file"
	}
}

// recordUserRequest record user request in DB.
//
// The row id is left to SQLite's AUTOINCREMENT instead of being computed from a
// full history scan, so each write costs a single insert rather than reading the
// whole table first.
func (s *Shell) recordUserRequest(ctx context.Context, request string) error {
	if err := s.usecases.history.Append(ctx, model.NewHistory(request)); err != nil {
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

// splitPathPrefix splits a typed path prefix at its last separator into the
// directory to scan, the leading text to keep on each suggestion (so completions
// preserve what the user typed), and the partial entry name to match. readDir is
// returned as it was typed; the caller decodes it, because what an escape means
// depends on whether the prefix was quoted.
//
// lastSep says where the separator is, which is the one thing the two ways of
// writing a path disagree about: inside quotes every "/" and "\" separates,
// while outside them a backslash before whitespace or a quote is an escape and
// not a separator at all.
//
// Examples (POSIX): "" -> ".", "", ""; "testdata/sa" -> "testdata/",
// "testdata/", "sa"; "testdata" -> ".", "", "testdata".
func splitPathPrefix(prefix string, lastSep func(string) int) (readDir, base, partial string) {
	idx := lastSep(prefix)
	if idx < 0 {
		return ".", "", prefix
	}
	// base always holds at least the separator, so it is never empty here and a
	// leading-separator path ("/foo") scans "/" through the normal route.
	base = prefix[:idx+1]
	return base, base, prefix[idx+1:]
}

// toScanPath turns the directory text of a completion prefix into a path the
// filesystem accepts: a remaining backslash is a separator (a Windows path
// typed on any host), so it is mapped to "/" and then to whatever this OS uses.
func toScanPath(readDir string) string {
	return filepath.FromSlash(strings.ReplaceAll(readDir, `\`, "/"))
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

// escapableChars are the characters splitArgs treats as a token boundary or as
// quoting syntax, and so the ones a backslash can escape outside quotes.
const escapableChars = " \t\\'\""

// unescapeBackslash decodes a backslash before any character in escapable, and
// leaves every other backslash alone.
//
// Which characters those are is the whole difference between the two ways a path
// can be written. Outside quotes a backslash escapes whitespace and quotes, so a
// lone one stays literal and is a Windows separator; inside double quotes only a
// quote and a backslash can be escaped, so anything else after a backslash was
// never an escape to begin with. Sharing the loop keeps that the only difference.
func unescapeBackslash(s, escapable string) string {
	if !strings.Contains(s, `\`) {
		return s // nothing to decode, so nothing to rebuild
	}
	var b strings.Builder
	b.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) && strings.ContainsRune(escapable, runes[i+1]) {
			b.WriteRune(runes[i+1])
			i++
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// unescapeCompletionPath reverses escapeCompletionPath, decoding a typed or
// accepted prefix back to a real path for filesystem lookups. It mirrors how
// splitArgs consumes a backslash: only before whitespace, a quote, or another
// backslash; a lone backslash stays literal (a Windows separator).
func unescapeCompletionPath(s string) string {
	return unescapeBackslash(s, escapableChars)
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

// argumentValues names what a helper command takes as its first argument, for
// the commands whose argument is a value rather than a filesystem path.
type argumentValues int

const (
	// argNone is a command that takes no argument at all.
	argNone argumentValues = iota
	// argDialect is a SQL dialect name.
	argDialect
	// argOutputMode is an output format name.
	argOutputMode
	// argRowMismatch is a row-mismatch policy name.
	argRowMismatch
	// argTable is the name of an imported table.
	argTable
)

// valueCommandSpec reports what a helper command's first argument is. ok is
// false for a command this table says nothing about, which falls back to the
// general completion.
//
// This is pathCommandSpec's sibling: that one knows which commands take a path,
// this one knows which take a value from a fixed set. Without it every command's
// argument was completed against everything sqly knows — ".dialect " offered
// fifty-five candidates, ".row-mismatch s" offered SELECT and SET but never
// "skip" — because the argument position was not part of the question.
func valueCommandSpec(cmd string) (argumentValues, bool) {
	switch cmd {
	case dialectCommand:
		return argDialect, true
	case modeCommand:
		return argOutputMode, true
	case rowMismatchCommand:
		return argRowMismatch, true
	// .dump names a table first and a path second; the path half is
	// pathCommandSpec's, and only the table half arrives here.
	case schemaCommand, describeCommand, dumpCommand:
		return argTable, true
	case pwdCommand, tablesCommand, clearCommand, exitCommand, helpCommand:
		return argNone, true
	}
	return argNone, false
}

// argumentCompletions returns the suggestions for the argument being typed after
// a helper command, and whether the command is one this knows. A command that
// takes no argument, and an argument position past the one the command accepts,
// return no suggestions rather than falling back to the general list: there is
// nothing valid to type there, and offering SQL keywords for it is noise.
func (s *Shell) argumentCompletions(ctx context.Context, completed []string) ([]Suggest, bool) {
	kind, ok := valueCommandSpec(completed[0])
	if !ok {
		return nil, false
	}
	if len(completed) != 1 { // the in-progress word is the command's first argument
		return nil, true
	}
	switch kind {
	case argDialect:
		return dialectSuggestions(), true
	case argOutputMode:
		return outputModeSuggestions(), true
	case argRowMismatch:
		return rowMismatchSuggestions(), true
	case argTable:
		return s.tableNameSuggestions(ctx), true
	case argNone:
		return nil, true
	}
	return nil, true
}

// dialectSuggestions offers every dialect filesql knows, spelled in the message
// the way its own project spells it.
func dialectSuggestions() []Suggest {
	dialects := dialect.Dialects()
	out := make([]Suggest, 0, len(dialects))
	for _, d := range dialects {
		out = append(out, Suggest{
			Text:        string(d),
			Description: d.DisplayName() + " query dialect",
		})
	}
	return out
}

// outputModeSuggestions offers the formats .mode can select, read from the same
// registry .mode resolves against, so completion cannot offer a name .mode would
// then reject. Excel and Parquet are absent for that reason: they name a file
// for --output or .dump, not something a screen can show.
func outputModeSuggestions() []Suggest {
	modes := model.SelectableModes()
	out := make([]Suggest, 0, len(modes))
	for _, m := range modes {
		out = append(out, Suggest{Text: m.String(), Description: outputFormatDescription(m)})
	}
	return out
}

// rowMismatchSuggestions offers the policies .row-mismatch accepts, each saying
// what it does to a row whose field count differs from the header.
func rowMismatchSuggestions() []Suggest {
	out := make([]Suggest, 0, len(model.RowMismatchPolicyNames))
	for _, name := range model.RowMismatchPolicyNames {
		out = append(out, Suggest{Text: name, Description: rowMismatchDescription(name)})
	}
	return out
}

// rowMismatchDescription is what completion says a row-mismatch policy does.
func rowMismatchDescription(name string) string {
	switch name {
	case model.RowMismatchSkip.String():
		return "drop such rows and import the rest"
	case model.RowMismatchPad.String():
		return "pad short rows with empty values; fail on long rows"
	default:
		return "fail the import when a row's field count differs from the header (default)"
	}
}

// tableNameSuggestions offers the imported table names, and only those: a
// command that names a table (.header, .schema, .describe, .dump) cannot take a
// column or a SQL keyword there.
func (s *Shell) tableNameSuggestions(ctx context.Context) []Suggest {
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return nil
	}
	out := make([]Suggest, 0, len(tables))
	for _, t := range tables {
		out = append(out, Suggest{Text: t.Name(), Description: "table: " + t.Name()})
	}
	return out
}

// commandSuggestions lists the helper commands in name order.
//
// The order is fixed here because CommandList is a map: ranging over it directly
// reordered the candidates on every keystroke, so the list under the cursor
// reshuffled while the user was reading it.
func (s *Shell) commandSuggestions() []Suggest {
	names := make([]string, 0, len(s.commands))
	for name := range s.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Suggest, 0, len(names))
	for _, name := range names {
		out = append(out, Suggest{
			Text:        s.commands[name].name,
			Description: "sqly command: " + s.commands[name].description,
		})
	}
	return out
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
	readDir, base, partial := splitPathPrefix(prefix, lastUnescapedSeparator)

	// readDir and partial come from the escaped prefix, so decode them before
	// touching the filesystem: an escaped space ("my\ dir/") names the real
	// directory "my dir/". base stays escaped so each suggestion round-trips
	// through splitArgs.
	readDir = toScanPath(unescapeCompletionPath(readDir))
	partial = unescapeCompletionPath(partial)

	// Expand a leading "~" so a home-directory prefix enumerates the real home
	// directory for the lookup. base keeps the typed "~/" so suggestions render as
	// "~/file.csv"; .import expands the tilde again at execution time.
	readDir, err := expandTilde(readDir)
	if err != nil {
		return nil
	}

	// Escape only the entry name; base is the verbatim typed prefix the prompt
	// library prefix-matches, so escaping it would corrupt the match. It is
	// slashified so a Windows-style prefix still matches the suggestion.
	return s.scanPathCompletions(readDir, partial, func(name string, isDir bool) string {
		text := slashifyBase(base) + escapeCompletionPath(name)
		if isDir {
			text += "/"
		}
		return text
	})
}

// scanPathCompletions lists the importable files and the directories of readDir
// that continue partial, rendering each with render.
//
// The two ways a path can be typed — backslash-escaped, or inside quotes —
// differ only in how the prefix is decoded on the way in and how a suggestion is
// written on the way out. What happens in between is the same directory read and
// the same filtering, so it happens once here: with a copy each, a change to
// what completion offers would have to be made twice, and completing "a b/" and
// completing "\"a b/" would silently stop agreeing.
//
// A directory is suggested with a trailing "/" so the user descends one level at
// a time, the way a shell completes paths. Hidden entries are skipped unless the
// user types a leading dot. Only readDir is read, not the tree below it, so
// latency tracks the directory rather than the repository.
func (s *Shell) scanPathCompletions(readDir, partial string, render func(name string, isDir bool) string) []Suggest {
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
		if entry.IsDir() {
			suggestions = append(suggestions, Suggest{
				Text:        render(name, true),
				Description: msgImportableDir,
			})
			continue
		}
		if s.isValidFileForCompletion(name) {
			suggestions = append(suggestions, Suggest{
				Text:        render(name, false),
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
	readDir, base, partial := splitPathPrefix(inner, lastAnySeparator)

	q := string(quote)
	// Inside quotes the path is literal, so base and name need no escaping
	// (filenames containing the quote character itself are not handled). A
	// directory keeps the quote open so the user descends one level at a time; a
	// file closes it, so the accepted line parses back to one argument.
	return s.scanPathCompletions(toScanPath(readDir), partial, func(name string, isDir bool) string {
		if isDir {
			return q + base + name + "/"
		}
		return q + base + name + q
	})
}

// lastAnySeparator returns the byte index of the last "/" or "\" in an
// already-decoded path fragment, or -1. Inside quotes a backslash escapes only a
// quote or another backslash, and decodeQuotedPath has already consumed those,
// so every backslash left is a separator.
func lastAnySeparator(p string) int {
	return strings.LastIndexAny(p, `/\`)
}

// decodeQuotedPath decodes the raw text typed inside an open quote into the real
// path. Single-quoted content is literal; double-quoted content unescapes \" and
// \\, matching splitArgs.
func decodeQuotedPath(s string, quote rune) string {
	if quote != '"' {
		return s
	}
	return unescapeBackslash(s, `"\`)
}
