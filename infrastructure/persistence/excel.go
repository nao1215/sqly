package persistence

import (
	"errors"
	"fmt"
	"os"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/xuri/excelize/v2"
)

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

	_, err = f.NewSheet(table.Name())
	if err != nil {
		return err
	}

	// Delete default sheet
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}

	f.SetActiveSheet(0)
	header := table.Header()
	if err := f.SetSheetRow(table.Name(), "A1", &header); err != nil {
		return err
	}

	const excelRowOffset = 2
	for i, record := range table.Records() {
		if err := f.SetSheetRow(table.Name(), fmt.Sprintf("A%d", i+excelRowOffset), &record); err != nil {
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
