package shell

import (
	"fmt"

	"github.com/nao1215/sqly/domain/model"
)

// modeCommand change output mode.
func (c CommandList) modeCommand(s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(Stdout, "[Usage]")
		fmt.Fprintf(Stdout, "  .mode OUTPUT_MODE   ※ current mode=%s\n", s.argument.Output.Mode.String())
		fmt.Fprintln(Stdout, "[Output mode list]")
		fmt.Fprintln(Stdout, "  table")
		fmt.Fprintln(Stdout, "  csv")
		fmt.Fprintln(Stdout, "  tsv")
		fmt.Fprintln(Stdout, "  ltsv")
		fmt.Fprintln(Stdout, "  json")
		return nil
	}

	if argv[0] == s.argument.Output.Mode.String() {
		fmt.Printf("already %s mode\n", argv[0])
		return nil
	}

	switch argv[0] {
	case model.PrintModeTable.String():
		fmt.Printf("Change output mode from %s to table\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeTable
	case model.PrintModeCSV.String():
		fmt.Printf("Change output mode from %s to csv\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeCSV
	case model.PrintModeTSV.String():
		fmt.Printf("Change output mode from %s to tsv\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeTSV
	case model.PrintModeLTSV.String():
		fmt.Printf("Change output mode from %s to ltsv\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeLTSV
	case model.PrintModeJSON.String():
		fmt.Printf("Change output mode from %s to json\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeJSON
	default:
		fmt.Println("invalid output mode: " + argv[0])
	}
	return nil
}
