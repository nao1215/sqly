// Package repository abstract the infrastructure layer.
package repository

import (
	"github.com/nao1215/sqly/domain/model"
)

// ExcelRepository is a repository that handles XLSX file export.
type ExcelRepository interface {
	// Dump write contents of DB table to XLSX file
	Dump(excelFilePath string, table *model.Table) error
}
