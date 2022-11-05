package shell

import (
	"fmt"

	"github.com/fatih/color"
)

// dumpCommand dump specified table to csv file
func (c CommandList) dumpCommand(s *Shell, argv []string) error {
	if len(argv) != 2 {
		fmt.Fprintln(Stdout, "[Usage]")
		fmt.Fprintln(Stdout, "  .dump TABLE_NAME CSV_FILE_PATH")
		return nil
	}

	table, err := s.sqlite3Interactor.List(s.Ctx, argv[0])
	if err != nil {
		return err
	}

	if err := s.csvInteractor.Dump(argv[1], table); err != nil {
		return err
	}
	fmt.Fprintf(Stdout, "dump `%s` table to %s\n", color.CyanString(argv[0]), color.HiCyanString(argv[1]))

	return nil
}
