package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

// FileSQLUsecase handles directory-based file imports and SQL operations
type FileSQLUsecase interface {
	// LoadFiles loads multiple files or directories into the database
	LoadFiles(ctx context.Context, filePaths ...string) error
	// GetTableNames returns the list of tables in the database
	GetTableNames(ctx context.Context) ([]*model.Table, error)
}
