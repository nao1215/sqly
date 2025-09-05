package interactor

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.CSVUsecase = (*csvInteractor)(nil)

// csvInteractor implementation of use cases related to CSV handler.
type csvInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter // filesql for improved performance and compression support
}

// NewCSVInteractor return CSVInteractor
func NewCSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
) usecase.CSVUsecase {
	return &csvInteractor{
		filesqlAdapter: filesqlAdapter,
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
	file, err := os.Create(filepath.Clean(csvFilePath))
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(table.Header()); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write records
	for _, record := range table.Records() {
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}
