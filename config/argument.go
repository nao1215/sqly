// Package config manage sqly configuration
package config

import (
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

// Version is sqly command version. Version value is assigned by LDFLAGS.
var Version string

// Arg is a structure for managing options and arguments
type Arg struct {
	// FilePath is CSV file paths that are imported into the DB.
	FilePath string
	// HelpFlag is HelpFlag flag.
	HelpFlag bool
	// Usage print help message
	Usage func()
}

// NewArg return *Arg that is assigned the result of parsing os.Args.
func NewArg() (*Arg, error) {
	arg := &Arg{}
	pflag.BoolVarP(&arg.HelpFlag, "help", "h", false, "show help message")
	pflag.Parse()

	arg.Usage = usage

	if !arg.HelpFlag && len(pflag.Args()) == 0 {
		return nil, errors.New("need to specify csv file path")
	}
	arg.FilePath = pflag.Arg(0)

	return arg, nil
}

func usage() {
	fmt.Printf("%s - execute SQL against CSV easily (%s)\n", color.GreenString("sqly"), Version)
	fmt.Println("")
	fmt.Println("[Usage]")
	fmt.Printf("  %s [OPTIONS] [FILE_PATH]\n", color.GreenString("sqly"))
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
