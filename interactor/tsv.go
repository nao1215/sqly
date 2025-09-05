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
var _ usecase.TSVUsecase = (*tsvInteractor)(nil)

// tsvInteractor implementation of use cases related to TSV handler.
type tsvInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter // filesql for improved performance and compression support
}

// NewTSVInteractor return TSVInteractor
func NewTSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
) usecase.TSVUsecase {
	return &tsvInteractor{
		filesqlAdapter: filesqlAdapter,
	}
}

// List get TSV data using filesql for improved performance and compression support.
func (ti *tsvInteractor) List(tsvFilePath string) (*model.Table, error) {
	ctx := context.Background()

	// Use filesql for improved performance and compression support
	if ti.filesqlAdapter == nil {
		return nil, errors.New("filesql adapter not initialized")
	}

	if err := ti.filesqlAdapter.LoadFile(ctx, tsvFilePath); err != nil {
		return nil, fmt.Errorf("failed to load TSV file: %w", err)
	}

	tableName := filesql.GetTableNameFromFilePath(tsvFilePath)
	query := "SELECT * FROM " + tableName

	table, err := ti.filesqlAdapter.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query TSV data: %w", err)
	}

	return table, nil
}

// Dump write contents of DB table to TSV file
func (ti *tsvInteractor) Dump(tsvFilePath string, table *model.Table) error {
	file, err := os.Create(filepath.Clean(tsvFilePath))
	if err != nil {
		return fmt.Errorf("failed to create TSV file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close TSV file: %w", cerr)
		}
	}()

	writer := csv.NewWriter(file)
	writer.Comma = '\t' // Use tab separator for TSV
	defer writer.Flush()

	// Write header
	if err := writer.Write(table.Header()); err != nil {
		return fmt.Errorf("failed to write TSV header: %w", err)
	}

	// Write records
	for _, record := range table.Records() {
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write TSV record: %w", err)
		}
	}

	return nil
}
