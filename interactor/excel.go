package interactor

import (
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.ExcelUsecase = (*excelInteractor)(nil)

// excelInteractor implementation of use cases related to Excel handler.
type excelInteractor struct {
	r repository.ExcelRepository
}

// NewExcelInteractor return ExcelInteractor
func NewExcelInteractor(r repository.ExcelRepository) usecase.ExcelUsecase {
	return &excelInteractor{r: r}
}

// List get Excel data.
func (e *excelInteractor) List(excelFilePath, sheetName string) (*model.Table, error) {
	excel, err := e.r.List(excelFilePath, sheetName)
	if err != nil {
		return nil, err
	}
	return excel.ToTable(), nil
}

// Dump write contents of DB table to JSON file
func (e *excelInteractor) Dump(excelFilePath string, table *model.Table) error {
	return e.r.Dump(excelFilePath, table)
}
