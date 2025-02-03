package shell

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// importCommand import csv into DB
func (c CommandList) importCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .import FILE_PATH(S) [--sheet=SHEET_NAME]")
		fmt.Fprintln(config.Stdout, "")
		fmt.Fprintln(config.Stdout, "  - Supported file format: csv, tsv, ltsv, json, xlam, xlsm, xlsx, xltm, xltx")
		fmt.Fprintln(config.Stdout, "  - If import multiple files, separate them with spaces")
		fmt.Fprintln(config.Stdout, "  - Does not support importing multiple excel sheets at once")
		fmt.Fprintln(config.Stdout, "  - If import an Excel file, specify the sheet name with --sheet")

		return nil
	}

	for _, v := range argv {
		var table *model.Table
		var sheetName string

		switch {
		case isCSV(v):
			csv, err := s.csvInteractor.List(v)
			if err != nil {
				return err
			}
			table = csv.ToTable()
		case isTSV(v):
			tsv, err := s.tsvInteractor.List(v)
			if err != nil {
				return err
			}
			table = tsv.ToTable()
		case isLTSV(v):
			ltsv, err := s.ltsvInteractor.List(v)
			if err != nil {
				return err
			}
			table = ltsv.ToTable()
		case isJSON(v):
			json, err := s.jsonInteractor.List(v)
			if err != nil {
				return err
			}
			table = json.ToTable()
		case isXLAM(v) || isXLSM(v) || isXLSX(v) || isXLTM(v) || isXLTX(v):
			sheetName = s.argument.SheetName
			if sheetName == "" {
				for _, s := range argv {
					if strings.HasPrefix(s, "--sheet=") {
						sheetName = strings.TrimPrefix(s, "--sheet=")
					}
				}
				if sheetName == "" {
					return errors.New("sheet name is required")
				}
			}
			excel, err := s.excelInteractor.List(v, sheetName)
			if err != nil {
				return err
			}
			table = excel.ToTable()
		case strings.HasPrefix(v, "--sheet="):
			continue
		default:
			return errors.New("not support file format: " + path.Ext(v))
		}

		if err := s.sqlite3Interactor.CreateTable(ctx, table); err != nil {
			return err
		}
		if err := s.sqlite3Interactor.Insert(ctx, table); err != nil {
			return err
		}
	}
	return nil
}
