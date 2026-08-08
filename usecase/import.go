package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// ImportUsecase loads files into the database and exposes the filesql helpers
// the import command needs to name, validate, and quote tables. It is kept
// separate from query and metadata so non-import commands do not depend on
// file loading.
//
// The Excel methods are part of it rather than an interface of their own: they
// answer what a workbook held and which of it an import took, which is a
// question only importing raises, and no command reaches them without also
// importing. The same holds for SkippedRows, and it is why the method count is
// allowed past the linter's limit: splitting on the count would put "what did
// the import do" behind two interfaces instead of one.
//
//nolint:interfacebloat // See the paragraph above.
type ImportUsecase interface {
	// LoadFiles loads multiple files or directories into the database
	LoadFiles(ctx context.Context, filePaths ...string) error
	// SetRowMismatchPolicy sets how a CSV/TSV row (one whose field count
	// differs from the header) is handled by subsequent imports.
	SetRowMismatchPolicy(policy model.RowMismatchPolicy)
	// SkippedRows reports what the row-mismatch policy dropped for the named
	// tables, and forgets it, so a count is reported once — at the import that
	// produced it. Tables that lost nothing are not listed.
	SkippedRows(tables []string) []model.SkippedRows
	// IsSupportedFile checks if the file has a format supported by filesql
	IsSupportedFile(filePath string) bool
	// QuoteIdentifier safely quotes a SQL identifier
	QuoteIdentifier(identifier string) string
	// GetTableNameFromFilePath derives a table name from a file path
	GetTableNameFromFilePath(filePath string) string

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
