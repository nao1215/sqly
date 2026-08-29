// Package interactor implements the usecase layer.
package interactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/domain/sqltext"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// Interface implementation checks. One concrete interactor satisfies the three
// focused session interfaces; commands depend on the narrow one they need.
var (
	_ usecase.QueryUsecase       = (*SQLite3Interactor)(nil)
	_ usecase.ImportUsecase      = (*SQLite3Interactor)(nil)
	_ usecase.MetadataUsecase    = (*SQLite3Interactor)(nil)
	_ usecase.PersistenceUsecase = (*SQLite3Interactor)(nil)
)

// SQLite3Interactor implements the SQLite3-backed session use cases. It handles
// SQL execution via the repository and file import via the filesql adapter.
// It is exported so dependency injection can bind the QueryUsecase,
// ImportUsecase, and MetadataUsecase interfaces to a single instance.
type SQLite3Interactor struct {
	r       repository.SQLite3Repository
	sql     *SQL
	adapter *filesql.FileSQLAdapter
	// sqlDialect is the SQL dialect applied to user queries run through ExecSQL.
	// It only affects user SQL; the internally generated SQLite statements that
	// other commands run go straight to the repository untranslated.
	sqlDialect dialect.Dialect
}

// NewSQLite3Interactor returns a new SQLite3Interactor that implements the
// QueryUsecase, ImportUsecase, and MetadataUsecase interfaces.
func NewSQLite3Interactor(
	r repository.SQLite3Repository,
	sql *SQL,
	adapter *filesql.FileSQLAdapter,
) *SQLite3Interactor {
	return &SQLite3Interactor{
		r:          r,
		sql:        sql,
		adapter:    adapter,
		sqlDialect: dialect.SQLite,
	}
}

// SetDialect sets the SQL dialect applied to subsequent user queries.
func (si *SQLite3Interactor) SetDialect(d dialect.Dialect) {
	if d == "" {
		d = dialect.SQLite
	}
	si.sqlDialect = d
}

// Dialect returns the current SQL dialect.
func (si *SQLite3Interactor) Dialect() dialect.Dialect {
	if si.sqlDialect == "" {
		return dialect.SQLite
	}
	return si.sqlDialect
}

// NewQueryUsecase exposes the interactor as the focused QueryUsecase.
// It exists so dependency injection hands shell a usecase interface rather than
// the concrete interactor.
func NewQueryUsecase(i *SQLite3Interactor) usecase.QueryUsecase { return i }

// NewImportUsecase exposes the interactor as the focused ImportUsecase.
func NewImportUsecase(i *SQLite3Interactor) usecase.ImportUsecase { return i }

// NewMetadataUsecase exposes the interactor as the focused MetadataUsecase.
func NewMetadataUsecase(i *SQLite3Interactor) usecase.MetadataUsecase { return i }

// NewPersistenceUsecase exposes the interactor as the focused PersistenceUsecase
// (native financial write-back).
func NewPersistenceUsecase(i *SQLite3Interactor) usecase.PersistenceUsecase { return i }

// TablesName return all table name.
func (si *SQLite3Interactor) TablesName(ctx context.Context) ([]*model.Table, error) {
	return si.r.TablesName(ctx)
}

// SchemaObjects returns every queryable table and view, including TEMP tables and
// views, for enumeration by .tables.
func (si *SQLite3Interactor) SchemaObjects(ctx context.Context) ([]*model.Table, error) {
	return si.r.SchemaObjects(ctx)
}

// List get records in the specified table
func (si *SQLite3Interactor) List(ctx context.Context, tableName string) (*model.Table, error) {
	return si.r.List(ctx, tableName)
}

// Header get table header name.
func (si *SQLite3Interactor) Header(ctx context.Context, tableName string) (*model.Table, error) {
	return si.r.Header(ctx, tableName)
}

// Query execute "SELECT" or "EXPLAIN" query
func (si *SQLite3Interactor) Query(ctx context.Context, query string) (*model.Table, error) {
	return si.r.Query(ctx, query)
}

// ExecSQL executes "SELECT/EXPLAIN" query or "INSERT/UPDATE/DELETE" statement.
// Returns:
// - For SELECT/EXPLAIN: (*model.Table, 0, error)
// - For INSERT/UPDATE/DELETE: (nil, affected_rows, error)
// - For unsupported commands: (nil, 0, error)
func (si *SQLite3Interactor) ExecSQL(ctx context.Context, statement string) (*model.Table, int64, error) {
	// Strip a leading BOM and leading comments so the statement classifies and
	// runs the same way it does on the batch and --sql-file paths. The session's
	// dialect decides what a comment is: MySQL and GoogleSQL open one with "#"
	// as well, and reading such a line as something to run made it reach the
	// engine as an empty statement.
	stmt := sqltext.StripNoise(statement, si.Dialect())
	if stmt == "" {
		return nil, 0, errors.New("no executable SQL statement: " + color.CyanString(statement))
	}
	// Translate the user statement from the configured dialect to SQLite before
	// classification and execution. This is a no-op for the SQLite dialect.
	translated, err := dialect.Translate(si.Dialect(), stmt)
	if err != nil {
		return nil, 0, fmt.Errorf("translate error (%s): %w: %s", si.Dialect(), err, color.CyanString(statement))
	}
	stmt = translated
	// Rewrite shorthands the engine does not accept (e.g. "TABLE name").
	stmt = normalizeStatement(stmt)

	// The check above ran on what the user wrote. This one runs on what the
	// translation produced, which is SQLite, and catches a statement that
	// translates to nothing at all. Asking the engine to run nothing is not a
	// statement, so it is refused the same way an empty one is.
	if sqltext.StripNoise(stmt, dialect.SQLite) == "" {
		return nil, 0, errors.New("no executable SQL statement: " + color.CyanString(statement))
	}

	// Reject statements sqly cannot run safely or correctly under its per-statement
	// transaction and in-memory session model (explicit transaction control,
	// VACUUM, ATTACH/DETACH), with a clear error instead of SQLite's confusing
	// internal message.
	if reason := unsupportedStatementReason(stmt); reason != "" {
		return nil, 0, fmt.Errorf("%s: %s", reason, color.CyanString(statement))
	}

	// sqly targets SQLite, so every supported statement is routed by shape: a
	// rowset-producing statement runs on the query path and prints its rows, while
	// any other statement (DML without RETURNING, DDL, ANALYZE, ...) runs on the
	// exec path and reports an affected-row count. SQLite is the authority on
	// validity, so an unsupported statement surfaces SQLite's own error.
	if si.sql.producesRowset(stmt) {
		table, err := si.Query(ctx, stmt)
		if err == nil {
			return table, 0, nil
		}
		// A no-rowset PRAGMA (a setter like "PRAGMA user_version = 1" or a command
		// like "PRAGMA incremental_vacuum") is routed here by keyword but yields no
		// result columns, so the query path reports ErrNoRows. Re-run it on the exec
		// path so it commits and reports neutral success instead of a misleading "no
		// records" error.
		if !errors.Is(err, repository.ErrNoRows) || sqltext.LeadingKeyword(stmt, dialect.SQLite) != sqlPRAGMA {
			return nil, 0, fmt.Errorf("execute query error: %w: %s", err, color.CyanString(statement))
		}
	}

	affectedRows, err := si.r.Exec(ctx, stmt)
	if err != nil {
		return nil, 0, fmt.Errorf("execute statement error: %w: %s", err, color.CyanString(statement))
	}
	return nil, affectedRows, nil
}

// LoadFiles loads multiple files or directories into the database.
func (si *SQLite3Interactor) LoadFiles(ctx context.Context, filePaths ...string) error {
	return si.adapter.LoadFiles(ctx, filePaths...)
}

// SkippedRows reports what the row-mismatch policy dropped for the named
// tables, and forgets it.
func (si *SQLite3Interactor) SkippedRows(tables []string) []model.SkippedRows {
	return si.adapter.SkippedRows(tables)
}

// SetRowMismatchPolicy sets how a mismatched CSV/TSV row is handled by subsequent
// imports.
func (si *SQLite3Interactor) SetRowMismatchPolicy(policy model.RowMismatchPolicy) {
	si.adapter.SetRowMismatchPolicy(policy)
}

// SetIncludeHiddenSheets decides whether subsequent Excel imports load the
// sheets a workbook hides as well as the ones it shows.
func (si *SQLite3Interactor) SetIncludeHiddenSheets(include bool) {
	si.adapter.SetIncludeHiddenSheets(include)
}

// IncludeHiddenSheets reports whether Excel imports load hidden sheets.
func (si *SQLite3Interactor) IncludeHiddenSheets() bool {
	return si.adapter.IncludeHiddenSheets()
}

// ExcelSheets reports every sheet of the workbook at path, in workbook order,
// and whether the workbook shows it.
func (si *SQLite3Interactor) ExcelSheets(path string) ([]model.ExcelSheet, error) {
	return si.adapter.ExcelSheets(path)
}

// ExcelSheetTableNames maps each named sheet of a workbook to the table it is
// loaded as.
func (si *SQLite3Interactor) ExcelSheetTableNames(path string, sheetNames []string) ([]string, error) {
	return si.adapter.ExcelSheetTableNames(path, sheetNames)
}

// IsSupportedFile checks if the file has a format supported by filesql.
func (si *SQLite3Interactor) IsSupportedFile(filePath string) bool {
	return filesql.IsSupportedFile(filePath)
}

// IsExcelFile checks if the file is an Excel format (.xlsx).
func (si *SQLite3Interactor) IsExcelFile(filePath string) bool {
	return filesql.IsExcelFile(filePath)
}

// QuoteIdentifier safely quotes a SQL identifier.
func (si *SQLite3Interactor) QuoteIdentifier(identifier string) string {
	return filesql.QuoteIdentifier(identifier)
}

// GetTableNameFromFilePath derives a table name from a file path.
func (si *SQLite3Interactor) GetTableNameFromFilePath(filePath string) string {
	return filesql.GetTableNameFromFilePath(filePath)
}

// DumpACHFile reconstructs a complete ACH file at outputPath from the table set
// registered under baseName, reflecting any session UPDATEs.
func (si *SQLite3Interactor) DumpACHFile(ctx context.Context, baseName, outputPath string) error {
	return si.adapter.DumpACHFile(ctx, baseName, outputPath)
}

// DumpFedWireFile reconstructs a complete Fedwire file at outputPath from the
// message table registered under baseName, reflecting any session UPDATEs.
func (si *SQLite3Interactor) DumpFedWireFile(ctx context.Context, baseName, outputPath string) error {
	return si.adapter.DumpFedWireFile(ctx, baseName, outputPath)
}
