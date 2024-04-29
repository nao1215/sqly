package shell

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// dumpCommand dump specified table to csv file
func (c CommandList) dumpCommand(s *Shell, argv []string) error {
	if len(argv) != 2 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .dump TABLE_NAME FILE_PATH")
		fmt.Fprintln(config.Stdout, "[Note]")
		fmt.Fprintln(config.Stdout, "  Output will be in the format specified in .mode.")
		fmt.Fprintln(config.Stdout, "  table mode is not available in .dump. If mode is table, .dump output CSV file.")
		return nil
	}

	table, err := s.sqlite3Interactor.List(s.Ctx, argv[0])
	if err != nil {
		return err
	}

	if err := dumpToFile(s, argv[1], table); err != nil {
		return err
	}
	fmt.Fprintf(config.Stdout, "dump `%s` table to %s (mode=%s)\n",
		color.CyanString(argv[0]), color.HiCyanString(argv[1]), dumpMode(s.argument.Output.Mode))

	return nil
}

// dumpToFile is dump table data to file.
func dumpToFile(s *Shell, filePath string, table *model.Table) error {
	var err error
	switch s.argument.Output.Mode {
	case model.PrintModeCSV:
		err = s.csvInteractor.Dump(filePath, table)
	case model.PrintModeTSV:
		err = s.tsvInteractor.Dump(filePath, table)
	case model.PrintModeLTSV:
		err = s.ltsvInteractor.Dump(filePath, table)
	case model.PrintModeJSON:
		err = s.jsonInteractor.Dump(filePath, table)
	case model.PrintModeExcel:
		err = s.excelInteractor.Dump(filePath, table)
	default:
		err = s.csvInteractor.Dump(filePath, table)
	}
	return err
}

// dumpMode is dump mode.
func dumpMode(m model.PrintMode) string {
	switch m {
	case model.PrintModeTable:
		return "csv"
	}
	return m.String()
}
