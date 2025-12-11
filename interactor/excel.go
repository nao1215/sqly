package interactor

import (
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
// Excel files require special handling for sheet names, which is why they have
// a specialized implementation that extends the base functionality.
type excelInteractor struct {
	*baseFileInteractor
	r repository.ExcelRepository
}

// NewExcelInteractor return ExcelInteractor
func NewExcelInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.ExcelRepository,
) usecase.ExcelUsecase {
	return &excelInteractor{
		baseFileInteractor: newBaseFileInteractor(filesqlAdapter, nil), // Excel doesn't use file repo for dumps
		r:                  r,
	}
}

// List get Excel data using filesql for improved performance and compression support.
// Excel files have unique sheet name handling that differs from simple file-based formats.
func (ei *excelInteractor) List(excelFilePath, sheetName string) (*model.Table, error) {
	// Load the Excel file using base functionality
	ctx, err := ei.loadFile(excelFilePath, "Excel")
	if err != nil {
		return nil, err
	}

	// For Excel files, filesql creates table names in the format "filename_sheetname"
	// If sheetName is provided, we need to find the actual table name
	var tableName string
	if sheetName != "" {
		// Get all table names and find one that ends with the requested sheet name
		tables, err := ei.getTableNames(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get table names: %w", err)
		}

		// Sanitize the sheet name to match how filesql creates table names.
		// filesql replaces spaces, accented characters, and other special characters
		// with underscores when creating SQL-safe table names.
		sanitizedSheetName := filesql.SanitizeForSQL(sheetName)

		// Look for a table that ends with the sanitized sheet name
		for _, table := range tables {
			if strings.HasSuffix(table.Name(), "_"+sanitizedSheetName) || table.Name() == sanitizedSheetName {
				tableName = table.Name()
				break
			}
		}

		if tableName == "" {
			return nil, fmt.Errorf("sheet '%s' not found in Excel file", sheetName)
		}
	} else {
		// If no sheet name specified, use the first available table
		tables, err := ei.getTableNames(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get table names: %w", err)
		}

		if len(tables) == 0 {
			return nil, fmt.Errorf("no sheets found in Excel file %q", excelFilePath)
		}

		tableName = tables[0].Name()
	}

	query := "SELECT * FROM " + tableName
	return ei.queryTable(ctx, query, "Excel")
}

// Dump write contents of DB table to Excel file
func (ei *excelInteractor) Dump(excelFilePath string, table *model.Table) error {
	return ei.r.Dump(filepath.Clean(excelFilePath), table)
}
