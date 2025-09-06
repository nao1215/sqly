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
var _ usecase.LTSVUsecase = (*ltsvInteractor)(nil)

// ltsvInteractor implementation of use cases related to LTSV handler.
type ltsvInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter
	r              repository.LTSVRepository
	f              repository.FileRepository
}

// NewLTSVInteractor return ltsvInteractor
func NewLTSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.LTSVRepository,
	f repository.FileRepository,
) usecase.LTSVUsecase {
	return &ltsvInteractor{
		filesqlAdapter: filesqlAdapter,
		r:              r,
		f:              f,
	}
}

// List get LTSV data using filesql for improved performance and compression support.
func (li *ltsvInteractor) List(ltsvFilePath string) (*model.Table, error) {
	ctx := context.Background()

	// Use filesql for improved performance and compression support
	if li.filesqlAdapter == nil {
		return nil, errors.New("filesql adapter not initialized")
	}

	if err := li.filesqlAdapter.LoadFile(ctx, ltsvFilePath); err != nil {
		return nil, fmt.Errorf("failed to load LTSV file: %w", err)
	}

	tableName := filesql.GetTableNameFromFilePath(ltsvFilePath)
	query := "SELECT * FROM " + tableName

	table, err := li.filesqlAdapter.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query LTSV data: %w", err)
	}

	return table, nil
}

// Dump write contents of DB table to LTSV file
func (li *ltsvInteractor) Dump(ltsvFilePath string, table *model.Table) error {
	f, err := li.f.Create(filepath.Clean(ltsvFilePath))
	if err != nil {
		return err
	}
	defer f.Close()

	return li.r.Dump(f, table)
}
