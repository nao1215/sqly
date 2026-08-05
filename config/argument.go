// Package config manage sqly configuration. This file is used to parse command line arguments.
package config

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/model"
	"github.com/spf13/pflag"
)

var (
	// Version is sqly command version. Version value is assigned by LDFLAGS.
	Version string
	// Stdout is new instance of Writer which handles escape sequence for stdout.
	Stdout = colorable.NewColorableStdout()
	// Stderr is new instance of Writer which handles escape sequence for stderr.
	Stderr = colorable.NewColorableStderr()
)

// DefaultInspectSample is the number of sample rows --inspect includes per
// table unless --inspect-sample overrides it.
//
// It is 0: --inspect describes what a file holds, and row data is what the file
// holds. A discovery command run by an agent, a wrapper, or a CI job over a file
// nobody has read yet must not print its contents to stdout as a side effect of
// asking for its schema. Row data is opt-in, one flag away, and the flag says a
// number rather than "yes", so the caller states how much it wants.
const DefaultInspectSample = 0

// defaultStdinTable is the table name a --stdin-format dataset gets when
// --stdin-table does not name one.
const defaultStdinTable = "stdin"

// Output is configuration for output data to file.
type Output struct {
	// FilePath is output destination path
	FilePath string
	// Mode is enum to specify output method
	Mode model.PrintMode
}

// Arg is a structure for managing options and arguments
type Arg struct {
	// FilePath is CSV file paths that are imported into the DB.
	FilePaths []string
	// Output is configuration for output data to file.
	Output *Output
	// HelpFlag is flag whether print usage or not (for --help option)
	HelpFlag bool
	// VersionFlag is flag whether print version or not (for --version option)
	VersionFlag bool
	// Query is SQL query (for --sql option)
	Query string
	// SQLFilePath is the path to a file containing SQL to execute (for
	// --sql-file). It lets stdin carry a piped dataset (--stdin-format) while the query
	// arrives from a file, which a single stdin stream cannot do. It supports
	// multiline statements with the same splitting rules as batch stdin mode and
	// cannot be combined with --sql.
	SQLFilePath string
	// ScriptFilePath is the path to a sqly script to execute (for --script-file).
	// A script is what the shell reads: SQL statements and dot-commands alike.
	// It is a separate flag from --sql-file rather than a relaxation of it,
	// because the two make different promises about what the file may do — a
	// .sql file that silently ran .save would be a shell script wearing a SQL
	// extension.
	ScriptFilePath string
	// AllowRemote lets this session download the http(s) URLs it is given. It is
	// an explicit capability rather than a default: without it sqly makes no HTTP
	// request at all, so a wrapper that never passes the flag has turned sqly's
	// own network access off. It is not a sandbox — see the note on the flag in
	// newArg.
	AllowRemote bool
	// InspectFlag, when true, prints a machine-readable JSON report of the
	// imported tables (names, source mapping, columns, row counts, and, when
	// --inspect-sample asks for them, sample rows) and exits without starting the
	// shell.
	InspectFlag bool
	// InspectSample caps how many sample rows each --inspect table includes.
	// 0 means schema-only (no sample rows), which keeps the report small for
	// wide or multi-table sources.
	InspectSample int
	// Usage message
	Usage string
	// StdinFormat, when non-empty, makes sqly read stdin as an input dataset of
	// this format (csv, tsv, ltsv, json, jsonl) instead of as SQL/helper commands.
	StdinFormat string
	// StdinTableName is the table name for the --stdin-format dataset (default:
	// stdin).
	StdinTableName string
	// RowMismatch selects how a CSV/TSV row whose field count differs from the
	// header is imported: error (default), skip, or pad. It sets the initial
	// policy for the session; the .row-mismatch shell command can change it at
	// runtime.
	RowMismatch model.RowMismatchPolicy
	// Encoding selects how a text import without a Unicode BOM is decoded before
	// parsing. It applies to CSV, TSV, LTSV, JSON, and JSONL inputs.
	Encoding model.TextEncoding
	// IncludeHiddenSheets imports the sheets an Excel workbook does not show
	// alongside the ones it does. It is off by default, because a hidden sheet
	// usually holds the spreadsheet's own working-out rather than data anyone
	// meant to publish. It sets the policy for the whole session, so a later
	// .import applies it too.
	IncludeHiddenSheets bool
	// ExplicitFlags names the flags the user actually typed, as opposed to the
	// ones sitting at their default. A flag that was typed but can never apply to
	// any of this run's inputs is an error, and only the user's intent — not the
	// value, which may equal the default — distinguishes the two cases.
	ExplicitFlags map[string]bool
	// Dialect is the SQL dialect applied to user queries (loading always uses
	// SQLite). It sets the initial dialect for the session; the .dialect shell
	// command can change it at runtime.
	Dialect dialect.Dialect
	// Version print version message
	Version func()
}

// NewArg return *Arg that is assigned the result of parsing os.Args.
// NOTE: Adding options directly to the pflag package results in a double
// option definition error when NewArg() is called multiple times.
// Therefore, create a new FlagSet() and add it to pflags.
// Ref. https://stackoverflow.com/questions/61216174/how-to-test-cli-flags-currently-failing-with-flag-redefined
func NewArg(args []string) (*Arg, error) {
	// Tag every failure as an ArgError in one place so the top-level command can
	// distinguish a bad invocation (which the user fixes on the command line) from
	// a genuine shell-start failure, without each return site repeating the wrap.
	arg, err := newArg(args)
	if err != nil {
		return nil, newArgError(err)
	}
	return arg, nil
}

// newArg parses args into an *Arg, returning the raw parse and validation errors.
// NewArg wraps those errors as ArgError; this function stays unwrapped so the
// individual sentinel errors remain easy to read and compare here.
func newArg(args []string) (*Arg, error) {
	if len(args) == 0 {
		return nil, ErrEmptyArg
	}
	arg := &Arg{}

	flag := pflag.FlagSet{}
	// Parse flags even when they appear after file/directory arguments. A
	// zero-value pflag.FlagSet disables this, which silently turns a misplaced
	// flag (e.g. "sqly data.csv --output out") and its value into import paths
	// that fail with "path does not exist". Interspersed parsing instead applies
	// the flag, and an unknown flag fails fast with a clear parse error.
	flag.SetInterspersed(true)
	// Input.
	stdinFormat := flag.String("stdin-format", "", "read stdin as a dataset instead of as SQL; one of: "+model.StdinFormatNames())
	stdinTable := flag.String("stdin-table", defaultStdinTable, "table name for the --stdin-format dataset")
	importEncoding := flag.String("encoding", model.TextEncodingUTF8.String(), "decode every csv, tsv, ltsv, json, and jsonl input that has no BOM as one of: "+strings.ReplaceAll(model.TextEncodingHelp(), "|", ", "))
	rowMismatch := flag.String("row-mismatch", model.RowMismatchError.String(), "for csv and tsv, what to do with a row whose field count differs from the header: error (fail the import), skip (drop the row), pad (fill a short row, fail on a long one)")
	flag.BoolVar(&arg.IncludeHiddenSheets, "include-hidden-sheets", false, "import the sheets an excel workbook hides as well as the ones it shows")
	// --allow-remote is a capability, not a security boundary. It decides whether
	// sqly performs an HTTP request at all; it decides nothing about where that
	// request may go. A caller that can add flags can add this one, so what it
	// protects is the case where the caller cannot: a wrapper, a sandbox policy,
	// or a CI job that fixes sqly's argument list. Said plainly in the help so
	// nobody reads it as an SSRF defense.
	flag.BoolVar(&arg.AllowRemote, "allow-remote", false, "allow sqly to download http(s) input explicitly named by this session; without it a url is refused before any request. this is a capability, not a sandbox or an ssrf defense")
	// Query.
	query := flag.StringP("sql", "s", "", "run one SQL statement, then exit")
	sqlFile := flag.StringP("sql-file", "f", "", "run every SQL statement in this file, then exit; a dot-command is rejected, so use --script-file for those; printing several results needs --output-format table, vertical, or markdown")
	scriptFile := flag.String("script-file", "", "run this sqly script, then exit: SQL statements and dot-commands, exactly as when piped in; use it to script .save and .import from a file")
	sqlDialect := flag.String("dialect", string(dialect.SQLite), "write the query in one of: sqlite, mysql, postgresql, googlesql; sqly translates it to SQLite")
	// Output.
	output := flag.StringP("output", "o", "", "write the one query result to this file instead of stdout")
	outputFormat := flag.String("output-format", model.PrintModeTable.String(), "print the query result as one of: "+model.PrintModeNames()+"; excel and parquet need --output")
	// Inspection.
	flag.BoolVar(&arg.InspectFlag, "inspect", false, "print one JSON report of the imported tables (schema, row counts, source) and exit; no row data unless --inspect-sample asks for it")
	inspectSample := flag.Int("inspect-sample", DefaultInspectSample, "sample rows per table in the --inspect report; 0 keeps the report schema-only")
	// General.
	flag.BoolVarP(&arg.HelpFlag, "help", "h", false, "print this help and exit")
	flag.BoolVarP(&arg.VersionFlag, "version", "v", false, "print the sqly version and exit")
	if err := flag.Parse(args[1:]); err != nil {
		return nil, err
	}

	// Reject other flags given an explicit empty value for the same reason: each
	// flag's empty string is the "flag absent" sentinel, so an explicit "" would
	// otherwise be silently ignored.
	if flag.Changed("sql") && *query == "" {
		return nil, errEmptyQuery
	}
	if flag.Changed("output") && *output == "" {
		return nil, errEmptyOutput
	}
	if flag.Changed("sql-file") && *sqlFile == "" {
		return nil, errEmptySQLFile
	}
	if flag.Changed("script-file") && *scriptFile == "" {
		return nil, errEmptyScriptFile
	}
	if flag.Changed("stdin-format") && *stdinFormat == "" {
		return nil, errEmptyStdinFormat
	}

	// Reject an unknown --stdin-format here rather than when stdin is staged. It
	// is a flag value like --row-mismatch or --dialect, so a typo should fail the
	// same way they do: before anything is read, as a usage error naming the
	// values that exist.
	if *stdinFormat != "" {
		normalized := strings.ToLower(strings.TrimSpace(*stdinFormat))
		if _, ok := model.StdinFormatExtension(normalized); !ok {
			return nil, fmt.Errorf("unsupported --stdin-format value %q: want %s", *stdinFormat, model.StdinFormatNames())
		}
		// Store what was validated, not what was typed. Accepting " CSV " here and
		// passing the raw string on left the staging step looking it up in a map
		// keyed by the canonical names and failing there instead — a value that
		// passed validation and then failed anyway.
		*stdinFormat = normalized
	}

	// --stdin-table only names the --stdin-format dataset, so it has no effect
	// without it. Reject it when set alone instead of silently ignoring it.
	if flag.Changed("stdin-table") && *stdinFormat == "" {
		return nil, errStdinTableWithoutFormat
	}

	// --inspect-sample only caps the rows --inspect samples, so it has no effect
	// without --inspect. Reject it (including invalid values like -1) when set
	// without --inspect instead of silently ignoring it.
	if flag.Changed("inspect-sample") && !arg.InspectFlag {
		return nil, errInspectSampleWithoutInspect
	}

	// A negative count is a malformed value, not a run that fails: it is caught
	// here so the exit code says "fix the command line" and nothing has been read
	// by the time it is reported. There is no upper bound — a caller that asks for
	// more rows than the table holds gets the table, which is what it asked for.
	if *inspectSample < 0 {
		return nil, fmt.Errorf("%w, got %d", errNegativeInspectSample, *inspectSample)
	}

	// Validate --stdin-table so it cannot be empty or contain path separators.
	// The name becomes a staging filename; a value like "" or "../escaped" would
	// otherwise create odd hidden files or write outside the temp directory. It is
	// only meaningful with --stdin-format, so validate only when staging applies.
	if *stdinFormat != "" {
		if err := validateStdinTable(*stdinTable); err != nil {
			return nil, err
		}
	}

	// Parse --row-mismatch into a policy, rejecting any value other than
	// error|skip|pad so a typo fails fast instead of silently defaulting.
	rowMismatchPolicy, err := model.ParseRowMismatchPolicy(*rowMismatch)
	if err != nil {
		return nil, err
	}
	importTextEncoding, err := model.ParseTextEncoding(*importEncoding)
	if err != nil {
		return nil, err
	}
	// Parse --dialect, rejecting any value the dialect package does not recognize
	// so a typo fails fast with the list of supported dialects.
	sqlDialectValue, err := dialect.Parse(*sqlDialect)
	if err != nil {
		return nil, fmt.Errorf("unknown SQL dialect %q: want sqlite, mysql, postgresql, or googlesql", *sqlDialect)
	}
	outputMode, err := parseOutputFormat(*outputFormat)
	if err != nil {
		return nil, err
	}

	arg.Usage = usage(flag)
	arg.Version = version
	arg.Output = newOutput(*output, outputMode)
	arg.FilePaths = flag.Args()
	arg.StdinFormat = *stdinFormat
	arg.StdinTableName = *stdinTable
	arg.RowMismatch = rowMismatchPolicy
	arg.Encoding = importTextEncoding
	arg.ExplicitFlags = explicitFlags(&flag)
	arg.Dialect = sqlDialectValue
	arg.Query = *query
	arg.SQLFilePath = *sqlFile
	arg.ScriptFilePath = *scriptFile
	arg.InspectSample = *inspectSample

	return arg, nil
}

// explicitFlags records which flags the user typed. pflag knows this per flag;
// collecting it once keeps the rest of the program from needing the FlagSet.
func explicitFlags(flags *pflag.FlagSet) map[string]bool {
	typed := make(map[string]bool)
	flags.Visit(func(f *pflag.Flag) { typed[f.Name] = true })
	return typed
}

// IsExplicit reports whether the user typed the named flag on the command line.
func (a *Arg) IsExplicit(name string) bool {
	return a != nil && a.ExplicitFlags[name]
}

// validateStdinTable rejects a --stdin-table that is empty or path-like. The name
// is used as a staging file name, so path separators or "."/".." could escape
// the temp directory or create surprising files.
func validateStdinTable(name string) error {
	if name == "" {
		return errInvalidStdinTable
	}
	if name == "." || name == ".." {
		return errInvalidStdinTable
	}
	if strings.ContainsAny(name, `/\`) {
		return errInvalidStdinTable
	}
	// Require a bare-identifier name so the advertised --stdin-table is the exact
	// queryable table name. Otherwise filesql sanitizes spaces and dashes (e.g.
	// "my data" -> "my_data"), leaving the name the user gave unusable.
	if !isValidTableIdentifier(name) {
		return errInvalidStdinTable
	}
	// A SQLite keyword has a valid identifier shape but is not queryable as a bare
	// table name (e.g. "SELECT * FROM select" is a syntax error), so reject it
	// instead of advertising an unusable name.
	if model.IsReservedSQLiteKeyword(name) {
		return errStdinTableReserved
	}
	return nil
}

// isValidTableIdentifier reports whether name is a bare SQL identifier: ASCII
// letters, digits, and underscores, not starting with a digit. Such a name is
// imported and queryable verbatim, with no sanitization.
func isValidTableIdentifier(name string) bool {
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return name != ""
}

func parseOutputFormat(name string) (model.PrintMode, error) {
	mode, ok := model.ParsePrintMode(name)
	if !ok {
		return model.PrintModeTable, fmt.Errorf("invalid output format %q: want %s", name, model.PrintModeNames())
	}
	return mode, nil
}

// newOutput returns the output destination and its selected format.
func newOutput(filePath string, mode model.PrintMode) *Output {
	return &Output{FilePath: filePath, Mode: mode}
}

// NeedsOutputToFile whether the data needs to be output to the file
func (a *Arg) NeedsOutputToFile() bool {
	return a != nil && a.Output != nil && a.Output.FilePath != "" && a.Query != ""
}

// optionGroup is one titled block of the option list. Options are grouped by the
// stage of a run they belong to, so a reader looking for "how do I write the
// result somewhere" reads one short block instead of scanning a flat list of
// eighteen flags.
type optionGroup struct {
	title string
	// options are long flag names, in the order the group shows them.
	options []string
}

// optionGroups is the layout of the [Options] section. Every flag defined in
// newArg appears in exactly one group; helpUsage fails loudly if that stops
// being true, so a flag added later cannot silently vanish from --help.
var optionGroups = []optionGroup{
	{title: "Input", options: []string{"stdin-format", "stdin-table", "encoding", "row-mismatch", "include-hidden-sheets", "allow-remote"}},
	{title: "Query", options: []string{"sql", "sql-file", "script-file", "dialect"}},
	{title: "Output", options: []string{"output", "output-format"}},
	{title: "Inspection", options: []string{"inspect", "inspect-sample"}},
	{title: "General", options: []string{"help", "version"}},
}

// Placeholder names --help shows after a value-taking flag.
const (
	argFile     = "FILE"
	argName     = "NAME"
	argFormat   = "FORMAT"
	argDir      = "DIR"
	argEncoding = "ENCODING"
	argPolicy   = "POLICY"
	argSQL      = "SQL"
	argCount    = "N"
)

// optionArgNames gives each value-taking flag the placeholder --help shows after
// it, so the kind of value a flag wants (a file, a directory, a format name) is
// visible in the list itself instead of only in the prose.
var optionArgNames = map[string]string{
	"stdin-format":   argFormat,
	"stdin-table":    argName,
	"encoding":       argEncoding,
	"row-mismatch":   argPolicy,
	"sql":            argSQL,
	"sql-file":       argFile,
	"script-file":    argFile,
	"dialect":        argName,
	"output":         argFile,
	"output-format":  argFormat,
	"inspect-sample": argCount,
}

// helpWidth is the column the option descriptions wrap at. 80 keeps --help
// readable in a default terminal without a pager.
const helpWidth = 80

// usage return usage message.
func usage(flag pflag.FlagSet) string {
	s := color.GreenString("sqly") + " - run SQL against CSV, TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH,\n"
	s += fmt.Sprintf("and Fedwire files (%s)\n", GetVersion())
	s += "\n"
	s += "[Usage]\n"
	s += fmt.Sprintf("  %s [OPTIONS] [FILE|DIRECTORY|URL ...]\n", color.GreenString("sqly"))
	s += "\n"
	s += "  Each input is loaded into an in-memory SQLite database as a table named\n"
	s += "  after the file. With --sql, --sql-file, or --script-file, sqly runs the\n"
	s += "  work and exits; with none of them, it opens an interactive shell, or runs\n"
	s += "  a script piped into it. sqly has no subcommands. Dot-commands run inside\n"
	s += "  the shell, in a piped script, and in a --script-file — including .save,\n"
	s += "  which writes changed tables back to their files.\n"
	s += "\n"
	s += "[Examples]\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("open the shell with a file loaded"))
	s += "    sqly sample.csv\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("run one query over a file or a directory"))
	s += "    sqly --sql 'SELECT * FROM sample' sample.csv\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("read a URL, which needs the capability to be given explicitly"))
	s += "    sqly --allow-remote --sql 'SELECT * FROM sample' https://example.com/sample.csv\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("join files of different formats"))
	s += "    sqly --sql 'SELECT * FROM a JOIN b USING (id)' a.csv b.parquet\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("write the result somewhere else, in another format"))
	s += "    sqly --output-format json --output out.json --sql 'SELECT 1' a.csv\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("read the data from a pipe and the query from a file"))
	s += "    cat data.csv | sqly --stdin-format csv --sql-file query.sql\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("look at a file you have never seen, as JSON (schema only)"))
	s += "    sqly --inspect sample.csv\n"
	s += "\n"
	s += helpOptions(&flag)
	s += "\n"
	s += "Queries are SQLite by default. --dialect translates MySQL, PostgreSQL, or\n"
	s += "GoogleSQL syntax into SQLite syntax; it does not emulate that database.\n"
	s += "Every statement runs on SQLite, with SQLite semantics, types, collation,\n"
	s += "NULL handling, and functions, so a result can differ from the source\n"
	s += "database. Choosing a non-SQLite dialect says so once, on stderr.\n"
	s += "\n"
	s += "A URL is downloaded only with --allow-remote. That flag is an explicit\n"
	s += "network capability, not a sandbox or an SSRF defense.\n"
	s += "\n"
	s += "Documentation:   https://nao1215.github.io/sqly/\n"
	s += "Report an issue: https://github.com/nao1215/sqly/issues\n"
	s += "GitHub Sponsors: https://github.com/sponsors/nao1215\n"
	return s
}

// helpOptions renders the option list as titled groups. The flag column is one
// width across every group, so the descriptions line up down the whole section
// even though each group is laid out separately.
func helpOptions(flags *pflag.FlagSet) string {
	labels := make(map[string]string)
	width := 0
	seen := make(map[string]bool)
	for _, group := range optionGroups {
		for _, name := range group.options {
			f := flags.Lookup(name)
			if f == nil {
				// A group naming a flag that no longer exists is a bug in this file,
				// not something a user can cause; say so where it is noticed.
				return fmt.Sprintf("[Options]\n  (internal error: --%s is grouped but not defined)\n", name)
			}
			seen[name] = true
			label := optionLabel(f)
			labels[name] = label
			if len(label) > width {
				width = len(label)
			}
		}
	}
	// Any flag missing from optionGroups would be invisible in --help, so list it
	// under a fallback heading rather than hide it.
	var ungrouped []string
	flags.VisitAll(func(f *pflag.Flag) {
		if !seen[f.Name] {
			ungrouped = append(ungrouped, f.Name)
			labels[f.Name] = optionLabel(f)
			if len(labels[f.Name]) > width {
				width = len(labels[f.Name])
			}
		}
	})

	groups := optionGroups
	if len(ungrouped) > 0 {
		groups = append(append([]optionGroup{}, groups...), optionGroup{title: "Other", options: ungrouped})
	}

	var b strings.Builder
	b.WriteString("[Options]\n")
	for i, group := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  %s\n", color.HiYellowString(group.title+":"))
		for _, name := range group.options {
			f := flags.Lookup(name)
			b.WriteString(optionEntry(labels[name], f.Usage+optionDefault(f), width))
		}
	}
	return b.String()
}

// optionLabel renders a flag's shorthand and long form with the placeholder for
// its value, e.g. "-o, --output FILE" or "    --inspect".
func optionLabel(f *pflag.Flag) string {
	label := "    "
	if f.Shorthand != "" {
		label = "-" + f.Shorthand + ", "
	}
	label += "--" + f.Name
	if arg, ok := optionArgNames[f.Name]; ok {
		label += " " + arg
	}
	return label
}

// optionDefault renders the trailing "(default: x)" note for a flag that has a
// meaningful one. A boolean's "false" and an empty string are what "not passed"
// already means, so noting them would only add noise.
func optionDefault(f *pflag.Flag) string {
	if f.DefValue == "" || f.DefValue == "false" {
		return ""
	}
	return " (default: " + f.DefValue + ")"
}

// optionEntry renders one option line, wrapping its description into the column
// after the flag so a narrow terminal never has to scroll sideways.
func optionEntry(label, description string, width int) string {
	indent := "    "
	gap := strings.Repeat(" ", width-len(label)+2)
	continuation := indent + strings.Repeat(" ", width+2)
	limit := helpWidth - len(continuation)
	if limit < 20 {
		limit = 20
	}

	var b strings.Builder
	line := indent + label + gap
	column := 0
	for _, word := range strings.Fields(description) {
		switch {
		case column == 0:
			line += word
			column = len(word)
		case column+1+len(word) <= limit:
			line += " " + word
			column += 1 + len(word)
		default:
			b.WriteString(line + "\n")
			line = continuation + word
			column = len(word)
		}
	}
	b.WriteString(line + "\n")
	return b.String()
}

// version print version message.
func version() {
	fmt.Fprintf(Stdout, "sqly %s\n", GetVersion())
}

// GetVersion return sqly command version.
// Version global variable is set by ldflags.
func GetVersion() string {
	if Version != "" {
		return Version
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if buildInfo.Main.Version != "" {
			return buildInfo.Main.Version
		}
	}
	return "(devel)"
}
