package interactor

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.CSVUsecase = (*csvInteractor)(nil)

// csvInteractor implementation of use cases related to CSV handler.
type csvInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter // filesql for improved performance and compression support
	r              repository.CSVRepository
	f              repository.FileRepository
}

// NewCSVInteractor return CSVInteractor
func NewCSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.CSVRepository,
	f repository.FileRepository,
) usecase.CSVUsecase {
	return &csvInteractor{
		filesqlAdapter: filesqlAdapter,
		r:              r,
		f:              f,
	}
}

// List get CSV data using filesql for improved performance and compression support.
func (ci *csvInteractor) List(csvFilePath string) (*model.Table, error) {
	ctx := context.Background()

	// Use filesql for improved performance and compression support
	if ci.filesqlAdapter == nil {
		return nil, errors.New("filesql adapter not initialized")
	}

	if err := ci.filesqlAdapter.LoadFile(ctx, csvFilePath); err != nil {
		return nil, fmt.Errorf("failed to load CSV file: %w", err)
	}

	tableName := filesql.GetTableNameFromFilePath(csvFilePath)
	query := "SELECT * FROM " + tableName

	table, err := ci.filesqlAdapter.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query CSV data: %w", err)
	}

	return table, nil
}

// Dump write contents of DB table to CSV file
func (ci *csvInteractor) Dump(csvFilePath string, table *model.Table) error {
	f, err := ci.f.Create(filepath.Clean(csvFilePath))
	if err != nil {
		return err
	}
	defer f.Close()

	return ci.r.Dump(f, table)
}
