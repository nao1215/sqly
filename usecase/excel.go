// Package usecase provides use cases related to Multiple File Format.
package usecase

import (
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// ExcelInteractor implementation of use cases related to Excel handler.
type ExcelInteractor struct {
	repository.ExcelRepository
}

// NewExcelInteractor return ExcelInteractor
func NewExcelInteractor(r repository.ExcelRepository) *ExcelInteractor {
	return &ExcelInteractor{ExcelRepository: r}
}

// List get Excel data.
func (e *ExcelInteractor) List(excelFilePath, sheetName string) (*model.Excel, error) {
	return e.ExcelRepository.List(excelFilePath, sheetName)
}

// Dump write contents of DB table to JSON file
func (e *ExcelInteractor) Dump(excelFilePath string, table *model.Table) error {
	return e.ExcelRepository.Dump(excelFilePath, table)
}
