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

// defaultInspectSample is the number of sample rows --inspect includes per
// table unless --inspect-sample overrides it.
const defaultInspectSample = 5

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
	// --sql-file). It lets stdin carry a piped dataset (--stdin) while the query
	// arrives from a file, which a single stdin stream cannot do. It supports
	// multiline statements with the same splitting rules as batch stdin mode and
	// cannot be combined with --sql.
	SQLFilePath string
	// InspectFlag, when true, prints a machine-readable JSON report of the
	// imported tables (names, source mapping, columns, row counts, and sample
	// rows) and exits without starting the shell.
	InspectFlag bool
	// InspectSample caps how many sample rows each --inspect table includes.
	// 0 means schema-only (no sample rows), which keeps the report small for
	// wide or multi-table sources.
	InspectSample int
	// CachePath, when non-empty, is the location of an opt-in import cache: a
	// SQLite snapshot of the imported tables. A warm run whose inputs are unchanged
	// loads from it instead of re-parsing the source files.
	CachePath string
	// SaveInPlace, when true, writes each table back over its source file after
	// the run (for --save). It overwrites source files, so it requires Force.
	SaveInPlace bool
	// SaveDir, when non-empty, writes each table into this directory after the
	// run (for --save-dir), preserving each source's format and compression and
	// leaving the original source files untouched.
	SaveDir string
	// Force allows the destructive in-place overwrite of SaveInPlace.
	Force bool
	// Usage message
	Usage string
	// SheetName is excel sheet name that is imported into the DB.
	SheetName string
	// StdinFormat, when non-empty, makes sqly read stdin as an input dataset of
	// this format (csv|tsv|ltsv|json|jsonl) instead of as SQL/helper commands.
	StdinFormat string
	// StdinTableName is the table name for the --stdin dataset (default: stdin).
	StdinTableName string
	// ImportMode selects how a ragged CSV/TSV row (one whose field count differs
	// from the header) is imported: stop (default), skip, or pad. It sets the
	// initial policy for the session; the .import-mode shell command can change it
	// at runtime.
	ImportMode model.MalformedRowPolicy
	// Encoding selects how a text import without a Unicode BOM is decoded before
	// parsing. It applies to CSV, TSV, LTSV, JSON, and JSONL inputs.
	Encoding model.TextEncoding
	// Dialect is the SQL dialect applied to user queries (loading always uses
	// SQLite). It sets the initial dialect for the session; the .dialect shell
	// command can change it at runtime.
	Dialect dialect.Dialect
	// Version print version message
	Version func()
}

const (
	outputFormatTable    = "table"
	outputFormatCSV      = "csv"
	outputFormatTSV      = "tsv"
	outputFormatLTSV     = "ltsv"
	outputFormatExcel    = "excel"
	outputFormatMarkdown = "markdown"
	outputFormatJSON     = "json"
	outputFormatNDJSON   = "ndjson"
	outputFormatParquet  = "parquet"
	outputFormatVertical = "vertical"
	outputFormatHelp     = outputFormatTable + "|" + outputFormatCSV + "|" + outputFormatTSV + "|" + outputFormatLTSV + "|" + outputFormatExcel + "|" + outputFormatMarkdown + "|" + outputFormatJSON + "|" + outputFormatNDJSON + "|" + outputFormatParquet + "|" + outputFormatVertical
)

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
	outputFormat := flag.String("output-format", outputFormatTable, "output format: "+outputFormatHelp)
	sheetName := flag.StringP("sheet", "S", "", "excel sheet name you want to import")
	stdinFormat := flag.String("stdin", "", "treat stdin as an input dataset of this format ("+outputFormatCSV+"|"+outputFormatTSV+"|"+outputFormatLTSV+"|"+outputFormatJSON+"|jsonl)")
	stdinName := flag.String("stdin-name", "stdin", "table name for the --stdin dataset")
	importMode := flag.String("import-mode", "stop", "how to import a CSV/TSV row whose field count differs from the header: stop (abort)|skip (drop)|pad (pad short rows with empty fields; reject long rows)")
	importEncoding := flag.String("encoding", model.TextEncodingUTF8.String(), "text input encoding for CSV/TSV/LTSV/JSON/JSONL import: "+model.TextEncodingHelp())
	sqlDialect := flag.String("dialect", string(dialect.SQLite), "SQL dialect for queries (loading always uses sqlite): sqlite|mysql|postgresql|googlesql")
	query := flag.StringP("sql", "s", "", "sql query you want to execute")
	sqlFile := flag.StringP("sql-file", "f", "", "path to a file with SQL to execute (multiline; cannot be used with --sql)")
	output := flag.StringP("output", "o", "", "destination path for the result of --sql or a single-result --sql-file script")
	flag.BoolVarP(&arg.InspectFlag, "inspect", "i", false, "print a JSON report of imported tables (schema, row counts, sample rows) and exit")
	inspectSample := flag.Int("inspect-sample", defaultInspectSample, "rows to include per table in --inspect (0 for schema only)")
	cachePath := flag.String("cache", "", "opt-in import cache: reuse a SQLite snapshot of the imported tables when inputs are unchanged (keyed by path+size+SHA-256 content hash)")
	flag.BoolVar(&arg.SaveInPlace, "save", false, "after the run, write each table back over its source file (requires --force)")
	saveDir := flag.String("save-dir", "", "after the run, write each table into this directory (originals untouched)")
	flag.BoolVar(&arg.Force, "force", false, "allow --save to overwrite source files in place")
	flag.BoolVarP(&arg.HelpFlag, "help", "h", false, "print help message")
	flag.BoolVarP(&arg.VersionFlag, "version", "v", false, "print sqly version")
	if err := flag.Parse(args[1:]); err != nil {
		return nil, err
	}

	// An explicit empty --sheet ("--sheet \"\"") is a mistake: the empty string
	// is the "no sheet selected" sentinel, so accepting it would silently behave
	// like the flag was never passed. Reject it so the error is visible.
	if flag.Changed("sheet") && *sheetName == "" {
		return nil, errEmptySheet
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
	if flag.Changed("save-dir") && *saveDir == "" {
		return nil, errEmptySaveDir
	}
	if flag.Changed("stdin") && *stdinFormat == "" {
		return nil, errEmptyStdin
	}

	// --stdin-name only names the --stdin dataset, so it has no effect without
	// --stdin. Reject it when set alone instead of silently ignoring it.
	if flag.Changed("stdin-name") && *stdinFormat == "" {
		return nil, errStdinNameWithoutStdin
	}

	// --inspect-sample only caps the rows --inspect samples, so it has no effect
	// without --inspect. Reject it (including invalid values like -1) when set
	// without --inspect instead of silently ignoring it.
	if flag.Changed("inspect-sample") && !arg.InspectFlag {
		return nil, errInspectSampleWithoutInspect
	}

	// --force only confirms the destructive --save write-back, so it has no effect
	// without --save/--save-dir. Reject it when set alone.
	if arg.Force && !arg.SaveInPlace && *saveDir == "" {
		return nil, errForceWithoutSave
	}

	// --cache must be non-empty when given (its empty string is the "absent"
	// sentinel).
	if flag.Changed("cache") && *cachePath == "" {
		return nil, errEmptyCache
	}

	// Validate --stdin-name so it cannot be empty or contain path separators.
	// The name becomes a staging filename; a value like "" or "../escaped" would
	// otherwise create odd hidden files or write outside the temp directory. Ref
	//. Only meaningful with --stdin, so validate only when staging applies.
	if *stdinFormat != "" {
		if err := validateStdinName(*stdinName); err != nil {
			return nil, err
		}
	}

	// Parse --import-mode into a policy, rejecting any value other than
	// stop|skip|pad so a typo fails fast instead of silently defaulting.
	importPolicy, err := model.ParseMalformedRowPolicy(*importMode)
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
		return nil, fmt.Errorf("unknown SQL dialect %q (supported: sqlite, mysql, postgresql, googlesql)", *sqlDialect)
	}
	outputMode, err := parseOutputFormat(*outputFormat)
	if err != nil {
		return nil, err
	}

	arg.Usage = usage(flag)
	arg.Version = version
	arg.Output = newOutput(*output, outputMode)
	arg.FilePaths = flag.Args()
	arg.SheetName = *sheetName
	arg.StdinFormat = *stdinFormat
	arg.StdinTableName = *stdinName
	arg.ImportMode = importPolicy
	arg.Encoding = importTextEncoding
	arg.Dialect = sqlDialectValue
	arg.Query = *query
	arg.SQLFilePath = *sqlFile
	arg.SaveDir = *saveDir
	arg.InspectSample = *inspectSample
	arg.CachePath = *cachePath

	return arg, nil
}

// validateStdinName rejects a --stdin-name that is empty or path-like. The name
// is used as a staging file name, so path separators or "."/".." could escape
// the temp directory or create surprising files.
func validateStdinName(name string) error {
	if name == "" {
		return errInvalidStdinName
	}
	if name == "." || name == ".." {
		return errInvalidStdinName
	}
	if strings.ContainsAny(name, `/\`) {
		return errInvalidStdinName
	}
	// Require a bare-identifier name so the advertised --stdin-name is the exact
	// queryable table name. Otherwise filesql sanitizes spaces and dashes (e.g.
	// "my data" -> "my_data"), leaving the name the user gave unusable.
	if !isValidTableIdentifier(name) {
		return errInvalidStdinName
	}
	// A SQLite keyword has a valid identifier shape but is not queryable as a bare
	// table name (e.g. "SELECT * FROM select" is a syntax error), so reject it
	// instead of advertising an unusable name.
	if model.IsReservedSQLiteKeyword(name) {
		return errStdinNameReserved
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
	switch strings.ToLower(strings.TrimSpace(name)) {
	case outputFormatTable:
		return model.PrintModeTable, nil
	case outputFormatCSV:
		return model.PrintModeCSV, nil
	case outputFormatTSV:
		return model.PrintModeTSV, nil
	case outputFormatLTSV:
		return model.PrintModeLTSV, nil
	case outputFormatExcel:
		return model.PrintModeExcel, nil
	case outputFormatMarkdown:
		return model.PrintModeMarkdownTable, nil
	case outputFormatJSON:
		return model.PrintModeJSON, nil
	case outputFormatNDJSON:
		return model.PrintModeNDJSON, nil
	case outputFormatParquet:
		return model.PrintModeParquet, nil
	case outputFormatVertical:
		return model.PrintModeVertical, nil
	default:
		return model.PrintModeTable, fmt.Errorf("invalid output format %q: want %s", name, outputFormatHelp)
	}
}

// newOutput returns the output destination and its selected format.
func newOutput(filePath string, mode model.PrintMode) *Output {
	return &Output{FilePath: filePath, Mode: mode}
}

// NeedsOutputToFile whether the data needs to be output to the file
func (a *Arg) NeedsOutputToFile() bool {
	return a != nil && a.Output != nil && a.Output.FilePath != "" && a.Query != ""
}

// usage return usage message.
func usage(flag pflag.FlagSet) string {
	s := fmt.Sprintf("%s - execute SQL against CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel/ACH/Fedwire with shell (%s)\n", color.GreenString("sqly"), GetVersion())
	s += "\n"
	s += "[Usage]\n"
	s += fmt.Sprintf("  %s [OPTIONS] [FILE_PATH(S)|DIRECTORY_PATH(S)]\n", color.GreenString("sqly"))
	s += "\n"
	s += "  sqly is flag-driven and has no subcommands: use --help and --version,\n"
	s += "  not \"sqly help\" or \"sqly version\". Helper commands like .tables and\n"
	s += "  .import run inside the shell or batch stdin mode, not as arguments.\n"
	s += "\n"
	s += "[Example]\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("run sqly shell"))
	s += "    sqly\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("Execute query for csv file"))
	s += "    sqly --sql 'SELECT * FROM sample' ./path/to/sample.csv\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("Import directory with all supported files"))
	s += "    sqly ./path/to/data/directory\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("Mix files and directories"))
	s += "    sqly file1.csv ./data_dir file2.tsv\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("Batch mode: pipe SQL/commands via stdin (no TTY)"))
	s += "    echo 'SELECT * FROM sample' | sqly ./path/to/sample.csv\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("Join a piped dataset (--stdin) with a query loaded from a file"))
	s += "    cat data.csv | sqly --stdin csv --sql-file query.sql\n"
	s += "\n"
	s += "[OPTIONS]\n"
	s += flag.FlagUsages()
	s += "\n"
	s += "[LICENSE]\n"
	s += fmt.Sprintf("  %s - Copyright (c) 2022 CHIKAMATSU Naohiro\n", color.CyanString("MIT LICENSE"))
	s += "  https://github.com/nao1215/sqly/blob/main/LICENSE\n"
	s += "\n"
	s += "[CONTACT]\n"
	s += "  https://github.com/nao1215/sqly/issues\n"
	s += "\n"
	s += "Documentation: https://nao1215.github.io/sqly/\n"
	s += "GitHub Sponsors: https://github.com/sponsors/nao1215\n"
	s += "\n"
	s += "sqly runs the DB in SQLite3 in-memory mode, so queries use SQLite3 syntax by default.\n"
	s += "Use --dialect (or .dialect in the shell) to write MySQL, PostgreSQL, or GoogleSQL\n"
	s += "queries instead; sqly translates them to SQLite3 before running them.\n"
	return s
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
