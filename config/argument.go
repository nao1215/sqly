// Package config manage sqly configuration. This file is used to parse command line arguments.
package config

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
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
	// Version print version message
	Version func()
}

// outputFlag is a structure for managing output format options.
type outputFlag struct {
	csv      bool
	tsv      bool
	ltsv     bool
	excel    bool
	markdown bool
	json     bool
	ndjson   bool
	parquet  bool
}

// NewArg return *Arg that is assigned the result of parsing os.Args.
// NOTE: Adding options directly to the pflag package results in a double
// option definition error when NewArg() is called multiple times.
// Therefore, create a new FlagSet() and add it to pflags.
// Ref. https://stackoverflow.com/questions/61216174/how-to-test-cli-flags-currently-failing-with-flag-redefined
func NewArg(args []string) (*Arg, error) {
	if len(args) == 0 {
		return nil, ErrEmptyArg
	}
	oFlag := outputFlag{}
	arg := &Arg{}

	flag := pflag.FlagSet{}
	// Parse flags even when they appear after file/directory arguments. A
	// zero-value pflag.FlagSet disables this, which silently turns a misplaced
	// flag (e.g. "sqly data.csv --output out") and its value into import paths
	// that fail with "path does not exist". Interspersed parsing instead applies
	// the flag, and an unknown flag fails fast with a clear parse error. Ref #264.
	flag.SetInterspersed(true)
	flag.BoolVarP(&oFlag.csv, "csv", "c", false, "change output format to csv (default: table)")
	flag.BoolVarP(&oFlag.excel, "excel", "e", false, "change output format to excel (default: table)")
	flag.BoolVarP(&oFlag.ltsv, "ltsv", "l", false, "change output format to ltsv (default: table)")
	flag.BoolVarP(&oFlag.markdown, "markdown", "m", false, "change output format to markdown table (default: table)")
	flag.BoolVarP(&oFlag.tsv, "tsv", "t", false, "change output format to tsv (default: table)")
	flag.BoolVarP(&oFlag.json, "json", "j", false, "change output format to json (default: table)")
	flag.BoolVarP(&oFlag.ndjson, "ndjson", "n", false, "change output format to ndjson (default: table)")
	flag.BoolVarP(&oFlag.parquet, "parquet", "p", false, "export results as parquet (export-only; use with --output or .dump)")
	sheetName := flag.StringP("sheet", "S", "", "excel sheet name you want to import")
	stdinFormat := flag.String("stdin", "", "treat stdin as an input dataset of this format (csv|tsv|ltsv|json|jsonl)")
	stdinName := flag.String("stdin-name", "stdin", "table name for the --stdin dataset")
	query := flag.StringP("sql", "s", "", "sql query you want to execute")
	sqlFile := flag.StringP("sql-file", "f", "", "path to a file with SQL to execute (multiline; cannot be used with --sql)")
	output := flag.StringP("output", "o", "", "destination path for SQL results specified in --sql option")
	flag.BoolVarP(&arg.InspectFlag, "inspect", "i", false, "print a JSON report of imported tables (schema, row counts, sample rows) and exit")
	inspectSample := flag.Int("inspect-sample", defaultInspectSample, "rows to include per table in --inspect (0 for schema only)")
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
	// like the flag was never passed. Reject it so the error is visible. Ref #313.
	if flag.Changed("sheet") && *sheetName == "" {
		return nil, errEmptySheet
	}

	// Validate --stdin-name so it cannot be empty or contain path separators.
	// The name becomes a staging filename; a value like "" or "../escaped" would
	// otherwise create odd hidden files or write outside the temp directory. Ref
	// #305. Only meaningful with --stdin, so validate only when staging applies.
	if *stdinFormat != "" {
		if err := validateStdinName(*stdinName); err != nil {
			return nil, err
		}
	}

	arg.Usage = usage(flag)
	arg.Version = version
	arg.Output = newOutput(*output, oFlag)
	arg.FilePaths = flag.Args()
	arg.SheetName = *sheetName
	arg.StdinFormat = *stdinFormat
	arg.StdinTableName = *stdinName
	arg.Query = *query
	arg.SQLFilePath = *sqlFile
	arg.SaveDir = *saveDir
	arg.InspectSample = *inspectSample

	return arg, nil
}

// validateStdinName rejects a --stdin-name that is empty or path-like. The name
// is used as a staging file name, so path separators or "."/".." could escape
// the temp directory or create surprising files. Ref #305.
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
	// "my data" -> "my_data"), leaving the name the user gave unusable. Ref #289.
	if !isValidTableIdentifier(name) {
		return errInvalidStdinName
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

// newOutput retur *Output
func newOutput(filePath string, of outputFlag) *Output {
	output := &Output{
		FilePath: filePath,
	}
	switch {
	case of.excel:
		output.Mode = model.PrintModeExcel
	case of.csv:
		output.Mode = model.PrintModeCSV
	case of.tsv:
		output.Mode = model.PrintModeTSV
	case of.ltsv:
		output.Mode = model.PrintModeLTSV
	case of.markdown:
		output.Mode = model.PrintModeMarkdownTable
	case of.json:
		output.Mode = model.PrintModeJSON
	case of.ndjson:
		output.Mode = model.PrintModeNDJSON
	case of.parquet:
		output.Mode = model.PrintModeParquet
	default:
		output.Mode = model.PrintModeTable
	}
	return output
}

// NeedsOutputToFile whether the data needs to be output to the file
func (a *Arg) NeedsOutputToFile() bool {
	return a.Output.FilePath != "" && a.Query != ""
}

// usage return usage message.
func usage(flag pflag.FlagSet) string {
	s := fmt.Sprintf("%s - execute SQL against CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel/ACH/Fedwire with shell (%s)\n", color.GreenString("sqly"), GetVersion())
	s += "\n"
	s += "[Usage]\n"
	s += fmt.Sprintf("  %s [OPTIONS] [FILE_PATH(S)|DIRECTORY_PATH(S)]\n", color.GreenString("sqly"))
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
	s += "sqly runs the DB in SQLite3 in-memory mode.\n"
	s += "So, SQL supported by sqly is the same as SQLite3 syntax.\n"
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
