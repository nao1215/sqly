package persistence

import (
	"errors"
	"fmt"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/xuri/excelize/v2"
)

type excelRepository struct{}

// NewExcelRepository return ExcelRepository
func NewExcelRepository() repository.ExcelRepository {
	return &excelRepository{}
}

// List get excel all data with header.
func (r *excelRepository) List(excelFilePath string, sheetName string) (excel *model.Excel, err error) {
	f, err := excelize.OpenFile(excelFilePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := f.Close(); err != nil {
			e = errors.Join(err, e)
		}
	}()

	excel = &model.Excel{
		Name:    sheetName,
		Header:  model.Header{},
		Records: []model.Record{},
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	for i, row := range rows {
		if i == 0 {
			excel.Header = row
			continue
		}
		excel.Records = append(excel.Records, row)
	}
	return excel, nil
}

// Dump write contents of DB table to XLSX file
func (r *excelRepository) Dump(excelFilePath string, table *model.Table) (err error) {
	f := excelize.NewFile()
	defer func() {
		if e := f.Close(); err != nil {
			e = errors.Join(err, e)
		}
	}()

	_, err = f.NewSheet(table.Name)
	if err != nil {
		return err
	}

	// Delete default sheet
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}

	f.SetActiveSheet(0)
	f.SetSheetRow(table.Name, "A1", &table.Header)
	for i, record := range table.Records {
		f.SetSheetRow(table.Name, fmt.Sprintf("A%d", i+2), &record)
	}
	return f.SaveAs(excelFilePath)
}
