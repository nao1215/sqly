// Package repository abstract the infrastructure layer.
package repository

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../../infrastructure/mock/$GOFILE -package mock

// ExcelRepository is a repository that handles XLAM / XLSM / XLSX / XLTM / XLTX file.
type ExcelRepository interface {
	// Dump write contents of DB table to XLSX file
	Dump(excelFilePath string, table *model.Table) error
}
