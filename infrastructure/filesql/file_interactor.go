package filesql

import (
	"context"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/usecase"
)

// fileInteractor implements file format use cases using filesql
type fileInteractor struct {
	adapter *FileSQLAdapter
}

// NewCSVInteractor creates a CSV use case interactor using filesql
func NewCSVInteractor(adapter *FileSQLAdapter) usecase.CSVUsecase {
	return &fileInteractor{adapter: adapter}
}

// NewTSVInteractor creates a TSV use case interactor using filesql
func NewTSVInteractor(adapter *FileSQLAdapter) usecase.TSVUsecase {
	return &fileInteractor{adapter: adapter}
}

// NewLTSVInteractor creates a LTSV use case interactor using filesql
func NewLTSVInteractor(adapter *FileSQLAdapter) usecase.LTSVUsecase {
	return &fileInteractor{adapter: adapter}
}

// NewExcelInteractor creates an Excel use case interactor using filesql
func NewExcelInteractor(adapter *FileSQLAdapter) usecase.ExcelUsecase {
	return &excelInteractor{adapter: adapter}
}

// List loads a file and returns its data as a table
func (fi *fileInteractor) List(filePath string) (*model.Table, error) {
	ctx := context.Background()

	// Load the file using filesql
	if err := fi.adapter.LoadFile(ctx, filePath); err != nil {
		return nil, err
	}

	// Get the table name from file path
	tableName := GetTableNameFromFilePath(filePath)

	// Query all data from the table
	query := "SELECT * FROM " + tableName
	return fi.adapter.Query(ctx, query)
}

// Dump writes contents of DB table to file
// For now, this maintains the same interface but will need to be implemented
// based on the specific format requirements
func (fi *fileInteractor) Dump(_ string, _ *model.Table) error {
	// TODO: Implement dump functionality
	// This would need to use filesql's save capabilities or fall back to existing logic
	return &FileSQLError{Op: "dump", Err: "dump functionality not yet implemented with filesql"}
}

// excelInteractor implements Excel-specific use cases using filesql
type excelInteractor struct {
	adapter *FileSQLAdapter
}

// List loads Excel file with sheet name and returns its data as a table
func (ei *excelInteractor) List(excelFilePath, sheetName string) (*model.Table, error) {
	ctx := context.Background()

	// Load the Excel file
	if err := ei.adapter.LoadFile(ctx, excelFilePath); err != nil {
		return nil, err
	}

	// For Excel files, table names are typically filename_sheetname
	baseName := GetTableNameFromFilePath(excelFilePath)
	tableName := baseName + "_" + sheetName

	// Query data from the specific sheet table
	query := "SELECT * FROM " + tableName
	return ei.adapter.Query(ctx, query)
}

// Dump writes contents of DB table to Excel file
func (ei *excelInteractor) Dump(_ string, _ *model.Table) error {
	// TODO: Implement Excel dump functionality
	return &FileSQLError{Op: "dump", Err: "Excel dump functionality not yet implemented with filesql"}
}

// GetSheetNames returns all sheet names in an Excel file
func (ei *excelInteractor) GetSheetNames(excelFilePath string) ([]string, error) {
	ctx := context.Background()

	// Load the Excel file
	if err := ei.adapter.LoadFile(ctx, excelFilePath); err != nil {
		return nil, err
	}

	// Get all table names
	tables, err := ei.adapter.GetTableNames(ctx)
	if err != nil {
		return nil, err
	}

	// Filter tables that match the Excel file pattern
	baseName := GetTableNameFromFilePath(excelFilePath)
	prefix := baseName + "_"

	var sheetNames []string
	for _, table := range tables {
		if strings.HasPrefix(table.Name(), prefix) {
			sheetName := strings.TrimPrefix(table.Name(), prefix)
			sheetNames = append(sheetNames, sheetName)
		}
	}

	return sheetNames, nil
}
