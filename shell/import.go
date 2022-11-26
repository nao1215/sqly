package shell

import (
	"errors"
	"fmt"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// importCommand import csv into DB
func (c CommandList) importCommand(s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .import FILE_PATH(S)")
		return nil
	}

	for _, v := range argv {
		var table *model.Table
		if isCSV(v) {
			csv, err := s.csvInteractor.List(v)
			if err != nil {
				return err
			}
			table = csv.ToTable()
		} else if isTSV(v) {
			tsv, err := s.tsvInteractor.List(v)
			if err != nil {
				return err
			}
			table = tsv.ToTable()
		} else if isLTSV(v) {
			ltsv, err := s.ltsvInteractor.List(v)
			if err != nil {
				return err
			}
			table = ltsv.ToTable()
		} else if isJSON(v) {
			json, err := s.jsonInteractor.List(v)
			if err != nil {
				return err
			}
			table = json.ToTable()
		} else {
			return errors.New("not support file format: " + v)
		}

		if err := s.sqlite3Interactor.CreateTable(s.Ctx, table); err != nil {
			return err
		}
		if err := s.sqlite3Interactor.Insert(s.Ctx, table); err != nil {
			return err
		}
	}
	return nil
}
