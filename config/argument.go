// Package config manage sqly configuration. This file is used to parse command line arguments.
package config

import (
	"fmt"
	"runtime/debug"

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
	// Usage message
	Usage string
	// SheetName is excel sheet name that is imported into the DB.
	SheetName string
	// Version print version message
	Version func()
}

// outputFlag is a structure for managing output format options.
type outputFlag struct {
	csv      bool
	tsv      bool
	ltsv     bool
	json     bool
	excel    bool
	markdown bool
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
	flag.BoolVarP(&oFlag.csv, "csv", "c", false, "change output format to csv (default: table)")
	flag.BoolVarP(&oFlag.excel, "excel", "e", false, "change output format to excel (default: table)")
	flag.BoolVarP(&oFlag.json, "json", "j", false, "change output format to json (default: table)")
	flag.BoolVarP(&oFlag.ltsv, "ltsv", "l", false, "change output format to ltsv (default: table)")
	flag.BoolVarP(&oFlag.markdown, "markdown", "m", false, "change output format to markdown table (default: table)")
	flag.BoolVarP(&oFlag.tsv, "tsv", "t", false, "change output format to tsv (default: table)")
	sheetName := flag.StringP("sheet", "S", "", "excel sheet name you want to import")
	query := flag.StringP("sql", "s", "", "sql query you want to execute")
	output := flag.StringP("output", "o", "", "destination path for SQL results specified in --sql option")
	flag.BoolVarP(&arg.HelpFlag, "help", "h", false, "print help message")
	flag.BoolVarP(&arg.VersionFlag, "version", "v", false, "print sqly version")
	if err := flag.Parse(args[1:]); err != nil {
		return nil, err
	}

	arg.Usage = usage(flag)
	arg.Version = version
	arg.Output = newOutput(*output, oFlag)
	arg.FilePaths = flag.Args()
	arg.SheetName = *sheetName
	arg.Query = *query

	return arg, nil
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
	case of.json:
		output.Mode = model.PrintModeJSON
	case of.markdown:
		output.Mode = model.PrintModeMarkdownTable
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
	s := fmt.Sprintf("%s - execute SQL against CSV/TSV/LTSV/JSON with shell (%s)\n", color.GreenString("sqly"), GetVersion())
	s += "\n"
	s += "[Usage]\n"
	s += fmt.Sprintf("  %s [OPTIONS] [FILE_PATH]\n", color.GreenString("sqly"))
	s += "\n"
	s += "[Example]\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("run sqly shell"))
	s += "    sqly\n"
	s += fmt.Sprintf("  - %s\n", color.HiYellowString("Execute query for csv file"))
	s += "    sqly --sql 'SELECT * FROM sample' ./path/to/sample.csv\n"
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
