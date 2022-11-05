package shell

import "fmt"

// importCommand import csv into DB
func (c CommandList) importCommand(s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(Stdout, "[Usage]")
		fmt.Fprintln(Stdout, "  .import CSV_FILE_PATH(S)")
	}

	for _, v := range argv {
		csv, err := s.csvInteractor.List(v)
		if err != nil {
			return err
		}

		table := csv.ToTable()
		if err := s.sqlite3Interactor.CreateTable(s.Ctx, table); err != nil {
			return err
		}

		if err := s.sqlite3Interactor.Insert(s.Ctx, table); err != nil {
			return err
		}
	}
	return nil
}
