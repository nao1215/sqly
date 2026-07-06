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

	if sheetName != "Sheet1" {
		if _, err = f.NewSheet(sheetName); err != nil {
			return err
		}
		// Delete the default sheet only when a distinct sheet replaced it.
		if err := f.DeleteSheet("Sheet1"); err != nil {
			return err
		}
	}

	f.SetActiveSheet(0)
	header := table.Header()
	if err := f.SetSheetRow(sheetName, "A1", &header); err != nil {
		return err
	}

	const excelRowOffset = 2
	for i, record := range table.Records() {
		if err := f.SetSheetRow(sheetName, fmt.Sprintf("A%d", i+excelRowOffset), &record); err != nil {
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
