package interactor

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.ExcelUsecase = (*excelInteractor)(nil)

// excelInteractor implementation of use cases related to Excel handler.
type excelInteractor struct {
	filesqlAdapter *filesql.FileSQLAdapter // filesql for improved performance and compression support
	r              repository.ExcelRepository
}

// NewExcelInteractor return ExcelInteractor
func NewExcelInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.ExcelRepository,
) usecase.ExcelUsecase {
	return &excelInteractor{
		filesqlAdapter: filesqlAdapter,
		r:              r,
	}
}

// List get Excel data using filesql for improved performance and compression support.
func (ei *excelInteractor) List(excelFilePath, sheetName string) (*model.Table, error) {
	ctx := context.Background()

	// Use filesql for improved performance and compression support
	if ei.filesqlAdapter == nil {
		return nil, errors.New("filesql adapter not initialized")
	}

	if err := ei.filesqlAdapter.LoadFile(ctx, excelFilePath); err != nil {
		return nil, fmt.Errorf("failed to load Excel file: %w", err)
	}

	// For Excel files, filesql creates table names in the format "filename_sheetname"
	// If sheetName is provided, we need to find the actual table name
	var tableName string
	if sheetName != "" {
		// Get all table names and find one that ends with the requested sheet name
		tables, err := ei.filesqlAdapter.GetTableNames(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get table names: %w", err)
		}

		// Look for a table that ends with the requested sheet name
		for _, table := range tables {
			if strings.HasSuffix(table.Name(), "_"+sheetName) || table.Name() == sheetName {
				tableName = table.Name()
				break
			}
		}

		if tableName == "" {
			return nil, fmt.Errorf("sheet '%s' not found in Excel file", sheetName)
		}
	} else {
		// If no sheet name specified, use the first available table
		tables, err := ei.filesqlAdapter.GetTableNames(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get table names: %w", err)
		}

		if len(tables) == 0 {
			return nil, errors.New("no sheets found in Excel file")
		}

		tableName = tables[0].Name()
	}

	query := "SELECT * FROM " + tableName
	table, err := ei.filesqlAdapter.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query Excel data: %w", err)
	}

	return table, nil
}

// Dump write contents of DB table to Excel file
func (ei *excelInteractor) Dump(excelFilePath string, table *model.Table) error {
	return ei.r.Dump(filepath.Clean(excelFilePath), table)
}
