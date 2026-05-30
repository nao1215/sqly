package interactor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// Interface implementation checks. One concrete interactor satisfies the three
// focused session interfaces; commands depend on the narrow one they need.
var (
	_ usecase.QueryUsecase    = (*SQLite3Interactor)(nil)
	_ usecase.ImportUsecase   = (*SQLite3Interactor)(nil)
	_ usecase.MetadataUsecase = (*SQLite3Interactor)(nil)
)

// SQLite3Interactor implements the SQLite3-backed session use cases. It handles
// SQL execution via the repository and file import via the filesql adapter.
// It is exported so dependency injection can bind the QueryUsecase,
// ImportUsecase, and MetadataUsecase interfaces to a single instance.
type SQLite3Interactor struct {
	r       repository.SQLite3Repository
	sql     *SQL
	adapter *filesql.FileSQLAdapter
}

// NewSQLite3Interactor returns a new SQLite3Interactor that implements the
// QueryUsecase, ImportUsecase, and MetadataUsecase interfaces.
func NewSQLite3Interactor(
	r repository.SQLite3Repository,
	sql *SQL,
	adapter *filesql.FileSQLAdapter,
) *SQLite3Interactor {
	return &SQLite3Interactor{
		r:       r,
		sql:     sql,
		adapter: adapter,
	}
}

// NewQueryUsecase exposes the interactor as the focused QueryUsecase.
// It exists so dependency injection hands shell a usecase interface rather than
// the concrete interactor.
func NewQueryUsecase(i *SQLite3Interactor) usecase.QueryUsecase { return i }

// NewImportUsecase exposes the interactor as the focused ImportUsecase.
func NewImportUsecase(i *SQLite3Interactor) usecase.ImportUsecase { return i }

// NewMetadataUsecase exposes the interactor as the focused MetadataUsecase.
func NewMetadataUsecase(i *SQLite3Interactor) usecase.MetadataUsecase { return i }

// CreateTable create a DB table with columns given as model.Table
func (si *SQLite3Interactor) CreateTable(ctx context.Context, t *model.Table) error {
	return si.r.CreateTable(ctx, t)
}

// TablesName return all table name.
func (si *SQLite3Interactor) TablesName(ctx context.Context) ([]*model.Table, error) {
	return si.r.TablesName(ctx)
}

// Insert set records in DB
func (si *SQLite3Interactor) Insert(ctx context.Context, t *model.Table) error {
	return si.r.Insert(ctx, t)
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

// Exec execute "INSERT" or "UPDATE" or "DELETE" statement
func (si *SQLite3Interactor) Exec(ctx context.Context, statement string) (int64, error) {
	return si.r.Exec(ctx, statement)
}

// ExecSQL executes "SELECT/EXPLAIN" query or "INSERT/UPDATE/DELETE" statement.
// Returns:
// - For SELECT/EXPLAIN: (*model.Table, 0, error)
// - For INSERT/UPDATE/DELETE: (nil, affected_rows, error)
// - For unsupported commands: (nil, 0, error)
func (si *SQLite3Interactor) ExecSQL(ctx context.Context, statement string) (*model.Table, int64, error) {
	argv := strings.Split(trimWordGaps(statement), " ")

	// NOTE: SQLY uses SQLite3. There is some SQL that can be changed from non-support
	// to support in the future. Currently, it is not supported because it is not needed
	// for developer ( == me:) ) use cases.
	switch {
	case si.sql.isDDL(argv[0]):
		return nil, 0, errors.New("not support data definition language: " + strings.Join(si.sql.ddl, ", "))
	case si.sql.isTCL(argv[0]):
		return nil, 0, errors.New("not support transaction control language: " + strings.Join(si.sql.tcl, ", "))
	case si.sql.isDCL(argv[0]):
		return nil, 0, errors.New("not support data control language: " + strings.Join(si.sql.dcl, ", "))
	case !si.sql.isDML(argv[0]):
		return nil, 0, errors.New("this input is not sql query or sqly helper command: " + color.CyanString(statement))
	case si.sql.isSelect(argv[0]) || si.sql.isExplain(argv[0]) || si.sql.isWithCTE(argv[0]):
		table, err := si.Query(ctx, statement)
		if err != nil {
			return nil, 0, fmt.Errorf("execute query error: %w: %s", err, color.CyanString(statement))
		}
		return table, 0, nil
	case si.sql.isInsert(argv[0]) || si.sql.isUpdate(argv[0]) || si.sql.isDelete(argv[0]):
		affectedRows, err := si.r.Exec(ctx, statement)
		if err != nil {
			return nil, 0, fmt.Errorf("execute statement error: %w: %s", err, color.CyanString(statement))
		}
		return nil, affectedRows, nil
	default:
		return nil, 0, fmt.Errorf("%s\n%s: %s\n%s",
			color.HiRedString("*** sqly bug ***"),
			"please report this log", color.CyanString(statement),
			"Web site: https://github.com/nao1215/sqly/issues")
	}
}

// LoadFiles loads multiple files or directories into the database.
func (si *SQLite3Interactor) LoadFiles(ctx context.Context, filePaths ...string) error {
	return si.adapter.LoadFiles(ctx, filePaths...)
}

// GetTableNames returns the list of tables currently available in the database.
func (si *SQLite3Interactor) GetTableNames(ctx context.Context) ([]*model.Table, error) {
	return si.adapter.GetTableNames(ctx)
}

// IsSupportedFile checks if the file has a format supported by filesql.
func (si *SQLite3Interactor) IsSupportedFile(filePath string) bool {
	return filesql.IsSupportedFile(filePath)
}

// IsExcelFile checks if the file is an Excel format (.xlsx).
func (si *SQLite3Interactor) IsExcelFile(filePath string) bool {
	return filesql.IsExcelFile(filePath)
}

// SanitizeForSQL sanitizes a string to be SQL-safe.
func (si *SQLite3Interactor) SanitizeForSQL(name string) string {
	return filesql.SanitizeForSQL(name)
}

// QuoteIdentifier safely quotes a SQL identifier.
func (si *SQLite3Interactor) QuoteIdentifier(identifier string) string {
	return filesql.QuoteIdentifier(identifier)
}

// GetTableNameFromFilePath derives a table name from a file path.
func (si *SQLite3Interactor) GetTableNameFromFilePath(filePath string) string {
	return filesql.GetTableNameFromFilePath(filePath)
}
