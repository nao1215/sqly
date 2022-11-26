package shell

import (
	"fmt"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// modeCommand change output mode.
func (c CommandList) modeCommand(s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintf(config.Stdout, "  .mode OUTPUT_MODE   ※ current mode=%s\n", s.argument.Output.Mode.String())
		fmt.Fprintln(config.Stdout, "[Output mode list]")
		fmt.Fprintln(config.Stdout, "  table")
		fmt.Fprintln(config.Stdout, "  markdown")
		fmt.Fprintln(config.Stdout, "  csv")
		fmt.Fprintln(config.Stdout, "  tsv")
		fmt.Fprintln(config.Stdout, "  ltsv")
		fmt.Fprintln(config.Stdout, "  json")
		return nil
	}

	if argv[0] == s.argument.Output.Mode.String() {
		fmt.Fprintf(config.Stdout, "already %s mode\n", argv[0])
		return nil
	}

	switch argv[0] {
	case model.PrintModeTable.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to table\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeTable
	case model.PrintModeMarkdownTable.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to markdown table\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeMarkdownTable
	case model.PrintModeCSV.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to csv\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeCSV
	case model.PrintModeTSV.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to tsv\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeTSV
	case model.PrintModeLTSV.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to ltsv\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeLTSV
	case model.PrintModeJSON.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to json\n", s.argument.Output.Mode.String())
		s.argument.Output.Mode = model.PrintModeJSON
	default:
		fmt.Fprintln(config.Stdout, "invalid output mode: "+argv[0])
	}
	return nil
}
