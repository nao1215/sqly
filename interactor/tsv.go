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
var _ usecase.TSVUsecase = (*tsvInteractor)(nil)

// tsvInteractor implementation of use cases related to TSV handler.
type tsvInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter // filesql for improved performance and compression support
	r              repository.TSVRepository
	f              repository.FileRepository
}

// NewTSVInteractor return TSVInteractor
func NewTSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.TSVRepository,
	f repository.FileRepository,
) usecase.TSVUsecase {
	return &tsvInteractor{
		filesqlAdapter: filesqlAdapter,
		r:              r,
		f:              f,
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
	f, err := ti.f.Create(filepath.Clean(tsvFilePath))
	if err != nil {
		return err
	}
	defer f.Close()

	return ti.r.Dump(f, table)
}
