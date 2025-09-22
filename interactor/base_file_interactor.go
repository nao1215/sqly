package interactor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
)

// Sentinel errors for common failure cases
var (
	ErrFilesqlAdapterNotInitialized = errors.New("filesql adapter not initialized")
	ErrFileRepositoryNotInitialized = errors.New("file repository not initialized")
)

// baseFileInteractor provides common functionality for file-based interactors
type baseFileInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter
	f              repository.FileRepository
}

// newBaseFileInteractor creates a new base file interactor
func newBaseFileInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	f repository.FileRepository,
) *baseFileInteractor {
	return &baseFileInteractor{
		filesqlAdapter: filesqlAdapter,
		f:              f,
	}
}

// list loads file data using filesql for improved performance and compression support
func (bi *baseFileInteractor) list(filePath string, fileType string) (*model.Table, error) {
	ctx := context.Background()

	if bi.filesqlAdapter == nil {
		return nil, fmt.Errorf("%s file processing: %w", fileType, ErrFilesqlAdapterNotInitialized)
	}

	if err := bi.filesqlAdapter.LoadFile(ctx, filePath); err != nil {
		return nil, fmt.Errorf("failed to load %s file %q: %w", fileType, filePath, err)
	}

	tableName := filesql.GetTableNameFromFilePath(filePath)
	query := "SELECT * FROM " + tableName

	table, err := bi.filesqlAdapter.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s data from table %q: %w", fileType, tableName, err)
	}

	return table, nil
}

// loadFile loads a file using filesql and returns the context for further operations
func (bi *baseFileInteractor) loadFile(filePath string, fileType string) (context.Context, error) {
	ctx := context.Background()

	if bi.filesqlAdapter == nil {
		return nil, fmt.Errorf("%s file processing: %w", fileType, ErrFilesqlAdapterNotInitialized)
	}

	if err := bi.filesqlAdapter.LoadFile(ctx, filePath); err != nil {
		return nil, fmt.Errorf("failed to load %s file %q: %w", fileType, filePath, err)
	}

	return ctx, nil
}

// getTableNames returns all available table names
func (bi *baseFileInteractor) getTableNames(ctx context.Context) ([]*model.Table, error) {
	return bi.filesqlAdapter.GetTableNames(ctx)
}

// queryTable executes a query and returns the result table
func (bi *baseFileInteractor) queryTable(ctx context.Context, query string, fileType string) (*model.Table, error) {
	table, err := bi.filesqlAdapter.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s data: %w", fileType, err)
	}
	return table, nil
}

// dump writes contents of DB table to file using the provided dump function
func (bi *baseFileInteractor) dump(filePath string, table *model.Table, dumpFunc func(*os.File, *model.Table) error) (err error) {
	if bi.f == nil {
		return ErrFileRepositoryNotInitialized
	}
	f, err := bi.f.Create(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("create output file %q: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close output file %q: %w", filePath, cerr)
		}
	}()
	if err = dumpFunc(f, table); err != nil {
		return fmt.Errorf("dump to %q: %w", filePath, err)
	}
	return nil
}
