package persistence

import (
	"errors"
	"fmt"

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

	for i, record := range table.Records() {
		if err := f.SetSheetRow(table.Name(), fmt.Sprintf("A%d", i+2), &record); err != nil {
			return err
		}
	}
	return f.SaveAs(excelFilePath)
}
