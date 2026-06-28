package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// ImportUsecase loads files into the database and exposes the filesql helpers
// the import command needs to name, validate, and quote tables. It is kept
// separate from query and metadata so non-import commands do not depend on
// file loading.
type ImportUsecase interface {
	// LoadFiles loads multiple files or directories into the database
	LoadFiles(ctx context.Context, filePaths ...string) error
	// GetTableNames returns the list of tables in the database
	GetTableNames(ctx context.Context) ([]*model.Table, error)
	// IsSupportedFile checks if the file has a format supported by filesql
	IsSupportedFile(filePath string) bool
	// IsExcelFile checks if the file is an Excel format
	IsExcelFile(filePath string) bool
	// ListExcelSheetNames returns the worksheet names of an Excel workbook in
	// workbook order, used for --sheet completion.
	ListExcelSheetNames(filePath string) ([]string, error)
	// SanitizeForSQL sanitizes a string to be SQL-safe
	SanitizeForSQL(name string) string
	// QuoteIdentifier safely quotes a SQL identifier
	QuoteIdentifier(identifier string) string
	// GetTableNameFromFilePath derives a table name from a file path
	GetTableNameFromFilePath(filePath string) string
}
