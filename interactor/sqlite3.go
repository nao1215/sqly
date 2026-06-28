package interactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
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

// NewPersistenceUsecase exposes the interactor as the focused PersistenceUsecase
// (native financial write-back and the import cache).
func NewPersistenceUsecase(i *SQLite3Interactor) usecase.PersistenceUsecase { return i }

// CreateTable create a DB table with columns given as model.Table
func (si *SQLite3Interactor) CreateTable(ctx context.Context, t *model.Table) error {
	return si.r.CreateTable(ctx, t)
}

// TablesName return all table name.
func (si *SQLite3Interactor) TablesName(ctx context.Context) ([]*model.Table, error) {
	return si.r.TablesName(ctx)
}

// SchemaObjects returns every queryable table and view, including TEMP tables and
// views, for enumeration by .tables.
func (si *SQLite3Interactor) SchemaObjects(ctx context.Context) ([]*model.Table, error) {
	return si.r.SchemaObjects(ctx)
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
	// Strip a leading BOM and leading comments so the statement classifies and
	// runs the same way it does on the batch and --sql-file paths.
	stmt := stripSQLNoise(statement)
	if stmt == "" {
		return nil, 0, errors.New("no executable SQL statement: " + color.CyanString(statement))
	}
	// Rewrite shorthands the engine does not accept (e.g. "TABLE name").
	stmt = normalizeStatement(stmt)

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
		if !errors.Is(err, repository.ErrNoRows) || leadingKeyword(stmt) != sqlPRAGMA {
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

// ListExcelSheetNames returns the worksheet names of an Excel workbook.
func (si *SQLite3Interactor) ListExcelSheetNames(filePath string) ([]string, error) {
	return filesql.SheetNames(filePath)
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

// SnapshotToCache writes the current session tables to cachePath as a standalone
// SQLite database for later reuse.
func (si *SQLite3Interactor) SnapshotToCache(ctx context.Context, cachePath string) error {
	return si.adapter.SnapshotToCache(ctx, cachePath)
}

// LoadFromCache populates the session database from a cache written by
// SnapshotToCache.
func (si *SQLite3Interactor) LoadFromCache(ctx context.Context, cachePath string) error {
	return si.adapter.LoadFromCache(ctx, cachePath)
}
