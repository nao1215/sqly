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
// versions (.gz, .bz2, .xz, .zst) are imported. Mixed file and directory arguments
// are supported in a single call.
func (i *FileSQLInteractor) LoadFiles(ctx context.Context, filePaths ...string) error {
	return i.adapter.LoadFiles(ctx, filePaths...)
}

// GetTableNames returns the list of tables currently available in the database.
// This includes all tables that have been imported from files or directories.
func (i *FileSQLInteractor) GetTableNames(ctx context.Context) ([]*model.Table, error) {
	return i.adapter.GetTableNames(ctx)
}
