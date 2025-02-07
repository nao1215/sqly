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
		printImportUsage()
		return nil
	}

	for _, v := range argv {
		var table *model.Table
		var err error
		var sheetName string

		switch {
		case isCSV(v):
			table, err = s.usecases.csv.List(v)
			if err != nil {
				return err
			}
		case isTSV(v):
			table, err = s.usecases.tsv.List(v)
			if err != nil {
				return err
			}
		case isLTSV(v):
			table, err = s.usecases.ltsv.List(v)
			if err != nil {
				return err
			}
		case isJSON(v):
			table, err = s.usecases.json.List(v)
			if err != nil {
				return err
			}
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
			table, err = s.usecases.excel.List(v, sheetName)
			if err != nil {
				return err
			}
		case strings.HasPrefix(v, "--sheet="):
			continue
		default:
			return errors.New("not support file format: " + path.Ext(v))
		}

		if err := s.usecases.sqlite3.CreateTable(ctx, table); err != nil {
			return err
		}
		if err := s.usecases.sqlite3.Insert(ctx, table); err != nil {
			return err
		}
	}
	return nil
}

// printImportUsage print import command usage.
func printImportUsage() {
	fmt.Fprintln(config.Stdout, "[Usage]")
	fmt.Fprintln(config.Stdout, "  .import FILE_PATH(S) [--sheet=SHEET_NAME]")
	fmt.Fprintln(config.Stdout, "")
	fmt.Fprintln(config.Stdout, "  - Supported file format: csv, tsv, ltsv, json, xlam, xlsm, xlsx, xltm, xltx")
	fmt.Fprintln(config.Stdout, "  - If import multiple files, separate them with spaces")
	fmt.Fprintln(config.Stdout, "  - Does not support importing multiple excel sheets at once")
	fmt.Fprintln(config.Stdout, "  - If import an Excel file, specify the sheet name with --sheet")
}
