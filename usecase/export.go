package usecase

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// ExportUsecase handles exporting table data to files in various formats.
type ExportUsecase interface {
	// DumpTable exports a table to a file in the specified format.
	DumpTable(filePath string, table *model.Table, format model.ExportFormat) error
}
