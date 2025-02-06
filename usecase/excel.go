// Package usecase defines interfaces for handling different file formats and operations.
// It follows clean architecture principles to separate business logic from implementation details.
package usecase

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// ExcelUsecase handle Excel file.
type ExcelUsecase interface {
	// List get Excel data.
	List(excelFilePath, sheetName string) (*model.Table, error)
	// Dump write contents of DB table to Excel file.
	Dump(excelFilePath string, table *model.Table) error
}
