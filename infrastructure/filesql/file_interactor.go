package filesql

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/usecase"
	"github.com/xuri/excelize/v2"
)

const (
	defaultExcelSheetName = "Sheet1"
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

// Dump writes contents of DB table to file in the format determined by file extension
// Supports CSV, TSV, LTSV, and Markdown formats with automatic format detection
func (fi *fileInteractor) Dump(filePath string, table *model.Table) error {
	// Check for nil table
	if table == nil {
		return &FileSQLError{Op: "dump", Err: "table cannot be nil"}
	}

	// Validate and clean the file path
	cleanPath, err := validateFilePath(filePath)
	if err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("invalid file path: %v", err)}
	}

	// Create or open the output file
	file, err := os.Create(cleanPath) // #nosec G304 - path is validated above
	if err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to create file %s: %v", filePath, err)}
	}
	defer func() {
		logCloseError("dump", file.Close())
	}()

	// Determine format from file extension and write accordingly
	ext := strings.ToLower(filepath.Ext(cleanPath))
	switch ext {
	case ".csv":
		return fi.dumpCSV(file, table)
	case ".tsv":
		return fi.dumpTSV(file, table)
	case ".ltsv":
		return fi.dumpLTSV(file, table)
	case ".md", ".markdown":
		return fi.dumpMarkdown(file, table)
	default:
		// Default to CSV format
		return fi.dumpCSV(file, table)
	}
}

// dumpCSV writes table data to file in CSV format
func (fi *fileInteractor) dumpCSV(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)

	records := [][]string{
		table.Header(),
	}
	for _, v := range table.Records() {
		records = append(records, v)
	}
	//nolint:errcheck // WriteAll doesn't return errors, check w.Error() instead
	_ = w.WriteAll(records)
	if err := w.Error(); err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to write CSV: %v", err)}
	}
	return nil
}

// dumpTSV writes table data to file in TSV format
func (fi *fileInteractor) dumpTSV(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)
	w.Comma = '\t'
	records := make([][]string, 0, 1+len(table.Records()))
	records = append(records, table.Header())
	for _, v := range table.Records() {
		records = append(records, v)
	}
	//nolint:errcheck // WriteAll doesn't return errors, check w.Error() instead
	_ = w.WriteAll(records)
	if err := w.Error(); err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to write TSV: %v", err)}
	}
	return nil
}

// dumpLTSV writes table data to file in LTSV format
func (fi *fileInteractor) dumpLTSV(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)
	w.Comma = '\t'
	records := make([][]string, 0, len(table.Records()))
	hdr := table.Header()
	for _, v := range table.Records() {
		// Add bounds checking to prevent panic
		maxLen := len(v)
		if len(hdr) < maxLen {
			maxLen = len(hdr)
		}

		r := make([]string, 0, maxLen)
		for i := range maxLen {
			r = append(r, hdr[i]+":"+v[i])
		}
		records = append(records, r)
	}
	//nolint:errcheck // WriteAll doesn't return errors, check w.Error() instead
	_ = w.WriteAll(records)
	if err := w.Error(); err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to write LTSV: %v", err)}
	}
	return nil
}

// dumpMarkdown writes table data to file in Markdown table format
func (fi *fileInteractor) dumpMarkdown(f *os.File, table *model.Table) error {
	// Use the existing printMarkdownTable method from the domain model
	// Note: table.Print doesn't return errors, it writes directly to the writer
	table.Print(f, model.PrintModeMarkdownTable)
	return nil
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
func (ei *excelInteractor) Dump(filePath string, table *model.Table) error {
	// Check for nil table
	if table == nil {
		return &FileSQLError{Op: "dump", Err: "table cannot be nil"}
	}

	// Validate and clean the file path
	cleanPath, err := validateFilePath(filePath)
	if err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("invalid file path: %v", err)}
	}

	// Use excelize library to create Excel file
	f := excelize.NewFile()
	defer func() {
		logCloseError("excel dump", f.Close())
	}()

	// Sanitize the table name for Excel sheet name
	sheetName := sanitizeExcelSheetName(table.Name())

	// Handle Sheet1 collision by using alternative name
	if sheetName == defaultExcelSheetName {
		sheetName = defaultExcelSheetName + "_1"
	}

	// Create a new sheet with the sanitized name
	idx, err := f.NewSheet(sheetName)
	if err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to create sheet: %v", err)}
	}

	// Delete default sheet
	if err := f.DeleteSheet(defaultExcelSheetName); err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to delete default sheet: %v", err)}
	}

	// Set the newly created sheet as active
	f.SetActiveSheet(idx)

	// Write header - optimize by directly converting slice
	headerSlice := table.Header()
	header := make([]any, len(headerSlice))
	for i, h := range headerSlice {
		header[i] = h
	}
	if err := f.SetSheetRow(sheetName, "A1", &header); err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to write header: %v", err)}
	}

	// Write records - optimize by directly converting each record
	for i, record := range table.Records() {
		row := make([]any, len(record))
		for j, c := range record {
			row[j] = c
		}
		if err := f.SetSheetRow(sheetName, fmt.Sprintf("A%d", i+2), &row); err != nil {
			return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to write record %d: %v", i, err)}
		}
	}

	// Save the file using validated path
	if err := f.SaveAs(cleanPath); err != nil {
		return &FileSQLError{Op: "dump", Err: fmt.Sprintf("failed to save Excel file: %v", err)}
	}

	return nil
}

// sanitizeExcelSheetName sanitizes a table name for use as an Excel sheet name
// Excel sheet names have restrictions:
// - Max 31 characters
// - Cannot contain: / \ ? * [ ] :
// - Cannot be empty
func sanitizeExcelSheetName(name string) string {
	if name == "" {
		return defaultExcelSheetName
	}

	// Replace invalid characters with underscores
	invalidChars := []string{"/", "\\", "?", "*", "[", "]", ":"}
	sanitized := name
	for _, char := range invalidChars {
		sanitized = strings.ReplaceAll(sanitized, char, "_")
	}

	// Trim to max length (31 characters)
	if len(sanitized) > 31 {
		sanitized = sanitized[:31]
	}

	// Ensure it's not empty after sanitization
	if strings.TrimSpace(sanitized) == "" {
		sanitized = defaultExcelSheetName
	}

	return sanitized
}

// validateFilePath validates and cleans the file path to prevent directory traversal attacks
func validateFilePath(filePath string) (string, error) {
	// Clean the path
	cleanPath := filepath.Clean(filePath)

	// Check for directory traversal attempts by inspecting path segments
	for _, seg := range strings.Split(cleanPath, string(filepath.Separator)) {
		if seg == ".." {
			return "", errors.New("invalid file path: directory traversal not allowed")
		}
	}

	// Check if path is absolute and potentially dangerous
	if filepath.IsAbs(cleanPath) {
		// For absolute paths, ensure they don't point to system directories
		systemDirs := []string{"/etc", "/bin", "/sbin", "/usr/bin", "/usr/sbin", "/sys", "/proc"}
		for _, sysDir := range systemDirs {
			if strings.HasPrefix(cleanPath, sysDir) {
				return "", errors.New("invalid file path: access to system directories not allowed")
			}
		}
	}

	return cleanPath, nil
}

// logCloseError logs file close errors that occur in defer statements
// Following project guideline to never omit error handling
func logCloseError(op string, err error) {
	if err != nil {
		// In a production environment, this would use a proper logger
		// For now, we use fmt.Fprintf to stderr as per project conventions
		fmt.Fprintf(os.Stderr, "Warning: failed to close file during %s: %v\n", op, err)
	}
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
