package config

import (
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

// Arg is a structure for managing options and arguments
type Arg struct {
	// FilePath is CSV file path. This CSV file is imported into the DB.
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

	arg.Usage = func() {
		fmt.Println("sqly - execute SQL against CSV easily")
		fmt.Println("")
		fmt.Println("[Usage]")
		fmt.Println("  sqly [OPTIONS] CSV_FILE_PATH")
		fmt.Println("")
		fmt.Println("[OPTIONS]")
		pflag.PrintDefaults()
	}

	if !arg.HelpFlag && len(pflag.Args()) == 0 {
		return nil, errors.New("need to specify csv file path")
	}
	arg.FilePath = pflag.Arg(0)

	return arg, nil
}
