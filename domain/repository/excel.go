// Package repository abstract the infrastructure layer.
package repository

import (
	"github.com/nao1215/sqly/domain/model"
)

// ExcelRepository is a repository that handles XLAM / XLSM / XLSX / XLTM / XLTX file.
// The process of opening and closing XLAM / XLSM / XLSX / XLTM / XLTX files is the responsibility of the upper layer.
type ExcelRepository interface {
	// List get excel all data with header.
	List(excelFilePath string, sheetName string) (*model.Excel, error)
	// Dump write contents of DB table to XLSX file
	Dump(excelFilePath string, table *model.Table) error
}
