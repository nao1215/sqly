package interactor

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// FileSQLInteractor implements FileSQLUsecase and provides file import functionality
// using the filesql library. It supports importing individual files or entire directories
// containing CSV, TSV, LTSV, and Excel files, including compressed versions.
type FileSQLInteractor struct {
	adapter *filesql.FileSQLAdapter
}

// NewFileSQLInteractor creates a new FileSQLInteractor instance with the provided FileSQLAdapter.
// The adapter handles the actual file processing and database operations.
func NewFileSQLInteractor(adapter *filesql.FileSQLAdapter) usecase.FileSQLUsecase {
	return &FileSQLInteractor{
		adapter: adapter,
	}
}

// LoadFiles loads multiple files or directories into the database.
// It accepts file paths and directory paths, automatically detecting the type.
// For directories, all supported files (CSV, TSV, LTSV, Excel) including compressed
// versions (.gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4) are imported. Mixed file and directory arguments
// are supported in a single call.
func (i *FileSQLInteractor) LoadFiles(ctx context.Context, filePaths ...string) error {
	return i.adapter.LoadFiles(ctx, filePaths...)
}

// GetTableNames returns the list of tables currently available in the database.
// This includes all tables that have been imported from files or directories.
func (i *FileSQLInteractor) GetTableNames(ctx context.Context) ([]*model.Table, error) {
	return i.adapter.GetTableNames(ctx)
}

// IsSupportedFile checks if the file has a format supported by filesql.
func (i *FileSQLInteractor) IsSupportedFile(filePath string) bool {
	return filesql.IsSupportedFile(filePath)
}

// IsExcelFile checks if the file is an Excel format (.xlsx).
func (i *FileSQLInteractor) IsExcelFile(filePath string) bool {
	return filesql.IsExcelFile(filePath)
}

// SanitizeForSQL sanitizes a string to be SQL-safe.
func (i *FileSQLInteractor) SanitizeForSQL(name string) string {
	return filesql.SanitizeForSQL(name)
}

// QuoteIdentifier safely quotes a SQL identifier.
func (i *FileSQLInteractor) QuoteIdentifier(identifier string) string {
	return filesql.QuoteIdentifier(identifier)
}
