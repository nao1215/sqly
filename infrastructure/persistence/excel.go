package persistence

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/xuri/excelize/v2"
)

// excelSheetNameMaxLen is Excel's hard limit on a worksheet name length.
const excelSheetNameMaxLen = 31

// excelForbiddenSheetChars are the characters Excel rejects in a worksheet name.
const excelForbiddenSheetChars = `:\/?*[]`

// excelSheetName adapts a table name to Excel's worksheet-name rules so an
// export never fails on excelize's NewSheet call. It replaces the forbidden
// characters (: \ / ? * [ ]) with '_', caps the length at 31 runes, and trims
// surrounding apostrophes (which Excel disallows at the edges), falling back to
// a default when nothing usable remains. A table name comes from the source
// filename, so a long or punctuated name is ordinary input, not user error.
func excelSheetName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if strings.ContainsRune(excelForbiddenSheetChars, r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	sheet := []rune(b.String())
	if len(sheet) > excelSheetNameMaxLen {
		sheet = sheet[:excelSheetNameMaxLen]
	}
	name = strings.Trim(string(sheet), "'")
	if name == "" {
		return "Sheet1"
	}
	return name
}

// _ interface implementation check
var _ repository.ExcelRepository = (*excelRepository)(nil)

type excelRepository struct{}

// NewExcelRepository return ExcelRepository
func NewExcelRepository() repository.ExcelRepository {
	return &excelRepository{}
}

// Dump write contents of DB table to XLSX file
func (r *excelRepository) Dump(excelFilePath string, table *model.Table) (err error) {
	f := excelize.NewFile()
	defer func() {
		if e := f.Close(); err != nil {
			err = errors.Join(err, e)
		}
	}()

	// Excel worksheet names are limited to 31 characters and cannot contain
	// : \ / ? * [ ], so the table name is adapted before it becomes a sheet.
	sheetName := excelSheetName(table.Name())

	// A new excelize file already has one sheet named "Sheet1"; rename it to the
	// target rather than adding a second sheet and deleting the default. Excelize
	// matches sheet names case-insensitively, so adding "sheet1" would collide
	// with the default and renaming sidesteps that entirely. When the target only
	// differs from "Sheet1" by case, keep the default name so row writes still
	// address the sheet that exists.
	const defaultSheet = "Sheet1"
	if strings.EqualFold(sheetName, defaultSheet) {
		sheetName = defaultSheet
	} else if err = f.SetSheetName(defaultSheet, sheetName); err != nil {
		return err
	}

	f.SetActiveSheet(0)
	header := table.Header()
	for _, label := range header {
		if err := ensureExcelRepresentable(label, label); err != nil {
			return err
		}
	}
	if err := f.SetSheetRow(sheetName, "A1", &header); err != nil {
		return err
	}

	const excelRowOffset = 2
	row := make([]string, 0, table.ColumnCount())
	for i, record := range table.Rows {
		row = record.AppendTo(row[:0])
		for col, value := range row {
			if err := ensureExcelRepresentable(table.ColumnName(col), value); err != nil {
				return err
			}
		}
		if err := f.SetSheetRow(sheetName, fmt.Sprintf("A%d", i+excelRowOffset), &row); err != nil {
			return err
		}
	}
	if err := f.SaveAs(excelFilePath); err != nil {
		return err
	}
	// excelize's SaveAs creates the file with os.ModePerm (0777), which leaves
	// the export executable. Reset to the same non-executable mode as other
	// outputs so .xlsx files are plain data files. Why not pass excelize
	// Options: SaveAs hard-codes the mode and ignores a permissions option.
	return os.Chmod(excelFilePath, defaultFilePerm)
}

// ensureExcelRepresentable reports an error when a value holds a character XLSX
// cannot carry, so the export is refused before a file is written.
//
// XLSX is XML, and XML 1.0 allows only three characters below U+0020: tab, line
// feed, and carriage return. Nothing above it is refused — DEL and U+FFFD itself
// are written unchanged. The writer substitutes U+FFFD for the rest, which
// makes an export silent data loss — the file appears, the process succeeds, and
// the value is gone. LTSV already refuses what it cannot represent rather than
// writing something that cannot be read back; this is the same rule for the same
// reason. The replacement character also has a meaning in sqly's output already:
// it says a file was read with the wrong --encoding, and an export that produces
// it on its own would make that signal a guess.
func ensureExcelRepresentable(label, value string) error {
	for _, r := range value {
		if r == '\t' || r == '\n' || r == '\r' {
			continue // the three XML keeps
		}
		if r < 0x20 {
			return fmt.Errorf("excel: value for column %q contains the control character U+%04X, which XLSX cannot represent; remove it or export to csv/tsv/json", label, r)
		}
	}
	return nil
}
