// Package usecase defines interfaces for handling different file formats and operations.
// It follows clean architecture principles to separate business logic from implementation details.
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
