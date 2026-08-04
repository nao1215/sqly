package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// ExcelSheetUsecase is the part of importing that only a workbook has: sheets,
// some of which the workbook does not show. It is separated from the rest of
// importing because it answers a different question — not "load this file" but
// "what was in it, and which of it did we take" — and because that question has
// no meaning for any other format.
type ExcelSheetUsecase interface {
	// IsExcelFile reports whether the path names an Excel workbook.
	IsExcelFile(filePath string) bool
	// SetIncludeHiddenSheets decides whether subsequent Excel imports load the
	// sheets a workbook hides as well as the ones it shows.
	SetIncludeHiddenSheets(include bool)
	// IncludeHiddenSheets reports whether Excel imports load hidden sheets.
	IncludeHiddenSheets() bool
	// ExcelSheets reports every sheet of the workbook at path, in workbook
	// order, and whether the workbook shows it. It reads only the sheet
	// directory, so it answers for a workbook that has not been imported.
	ExcelSheets(path string) ([]model.ExcelSheet, error)
	// ExcelSheetTableNames maps each named sheet of the workbook at path to the
	// table it is loaded as, parallel to sheetNames. It reports an error when
	// two of the sheets would share a table.
	ExcelSheetTableNames(path string, sheetNames []string) ([]string, error)
}

// ImportUsecase loads files into the database and exposes the filesql helpers
// the import command needs to name, validate, and quote tables. It is kept
// separate from query and metadata so non-import commands do not depend on
// file loading.
type ImportUsecase interface {
	ExcelSheetUsecase

	// LoadFiles loads multiple files or directories into the database
	LoadFiles(ctx context.Context, filePaths ...string) error
	// SetRowMismatchPolicy sets how a CSV/TSV row (one whose field count
	// differs from the header) is handled by subsequent imports.
	SetRowMismatchPolicy(policy model.RowMismatchPolicy)
	// RowMismatchPolicy returns the policy applied to mismatched CSV/TSV rows.
	RowMismatchPolicy() model.RowMismatchPolicy
	// GetTableNames returns the list of tables in the database
	GetTableNames(ctx context.Context) ([]*model.Table, error)
	// IsSupportedFile checks if the file has a format supported by filesql
	IsSupportedFile(filePath string) bool
	// SanitizeForSQL sanitizes a string to be SQL-safe
	SanitizeForSQL(name string) string
	// QuoteIdentifier safely quotes a SQL identifier
	QuoteIdentifier(identifier string) string
	// GetTableNameFromFilePath derives a table name from a file path
	GetTableNameFromFilePath(filePath string) string
}
