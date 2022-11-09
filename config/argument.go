// Package config manage sqly configuration
package config

import (
	"fmt"
	"runtime/debug"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/domain/model"
	"github.com/spf13/pflag"
)

var (
	// Version is sqly command version. Version value is assigned by LDFLAGS.
	Version string
	// query is SQL statement (for --sql option)
	query = pflag.StringP("sql", "s", "", "sql query you want to execute")
	// output is output destionation when user use --sql option (for --option option)
	output = pflag.StringP("output", "o", "", "destination path for SQL results specified in --sql option")
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
	// Usage print help message
	Usage func()
	// Version print version message
	Version func()
}

// NewArg return *Arg that is assigned the result of parsing os.Args.
func NewArg() (*Arg, error) {
	csvFlag := false
	jsonFlag := false

	arg := &Arg{}
	pflag.BoolVarP(&csvFlag, "csv", "c", false, "change output format to csv (default: table)")
	pflag.BoolVarP(&jsonFlag, "json", "j", false, "change output format to json (default: table)")
	pflag.BoolVarP(&arg.HelpFlag, "help", "h", false, "print help message")
	pflag.BoolVarP(&arg.VersionFlag, "version", "v", false, "print help message")
	pflag.Parse()

	arg.Usage = usage
	arg.Version = version
	arg.Output = newOutput(*output, csvFlag, jsonFlag)
	arg.FilePaths = pflag.Args()
	arg.Query = *query

	return arg, nil
}

// newOutput retur *Output
func newOutput(filePath string, csvFlag, jsonFlag bool) *Output {
	mode := model.PrintModeTable
	if csvFlag {
		mode = model.PrintModeCSV
	} else if jsonFlag {
		mode = model.PrintModeJSON
	}
	return &Output{
		FilePath: filePath,
		Mode:     mode,
	}
}

// NeedsOutputToFile whether the data needs to be output to the file
func (a *Arg) NeedsOutputToFile() bool {
	return a.Output.FilePath != "" && a.Query != ""
}

func usage() {
	fmt.Printf("%s - execute SQL against CSV easily (%s)\n", color.GreenString("sqly"), GetVersion())
	fmt.Println("")
	fmt.Println("[Usage]")
	fmt.Printf("  %s [OPTIONS] [FILE_PATH]\n", color.GreenString("sqly"))
	fmt.Println("")
	fmt.Println("[Example]")
	fmt.Printf("  - %s\n", color.HiYellowString("run sqly shell"))
	fmt.Printf("    sqly\n")
	fmt.Printf("  - %s\n", color.HiYellowString("Execute query for csv file"))
	fmt.Printf("    sqly --sql 'SELECT * FROM sample' ./path/to/file.csv\n")
	fmt.Println("")
	fmt.Println("[OPTIONS]")
	pflag.PrintDefaults()
	fmt.Println("")
	fmt.Println("[LICENSE]")
	fmt.Printf("  %s - Copyright (c) 2022 CHIKAMATSU Naohiro\n", color.CyanString("MIT LICENSE"))
	fmt.Println("  https://github.com/nao1215/sqly/blob/main/LICENSE")
	fmt.Println("")
	fmt.Println("[CONTACT]")
	fmt.Println("  https://github.com/nao1215/sqly/issues")
	fmt.Println("")
	fmt.Println("sqly runs the DB in SQLite3 in-memory mode.")
	fmt.Println("So, SQL supported by sqly is the same as SQLite3 syntax.")
}

func version() {
	fmt.Printf("%s %s\n", color.GreenString("sqly"), GetVersion())
}

// GetVersion return sqly command version.
// Version global variable is set by ldflags.
func GetVersion() string {
	version := "unknown"
	if Version != "" {
		version = Version
	} else if buildInfo, ok := debug.ReadBuildInfo(); ok {
		version = buildInfo.Main.Version
	}
	return version
}
