package interactor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.LTSVUsecase = (*ltsvInteractor)(nil)

// ltsvInteractor implementation of use cases related to LTSV handler.
type ltsvInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter
}

// NewLTSVInteractor return ltsvInteractor
func NewLTSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
) usecase.LTSVUsecase {
	return &ltsvInteractor{
		filesqlAdapter: filesqlAdapter,
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
	file, err := os.Create(filepath.Clean(ltsvFilePath))
	if err != nil {
		return fmt.Errorf("failed to create LTSV file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close LTSV file: %w", cerr)
		}
	}()

	// Write LTSV format: key1:value1<tab>key2:value2<newline>
	headers := table.Header()

	for _, record := range table.Records() {
		// Pre-allocate slice with appropriate capacity for better memory usage
		ltsvLine := make([]string, 0, len(headers))
		for i, value := range record {
			if i < len(headers) {
				ltsvLine = append(ltsvLine, headers[i]+":"+value)
			}
		}
		_, err := file.WriteString(strings.Join(ltsvLine, "\t") + "\n")
		if err != nil {
			return fmt.Errorf("failed to write LTSV record: %w", err)
		}
	}

	return nil
}
