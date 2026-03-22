package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// DatabaseUsecase handle Relational Database and file import operations.
// This interface unifies session-level operations: SQL execution, table management,
// and file import via filesql.
type DatabaseUsecase interface {
	// CreateTable create a DB table with columns given as model.Table
	CreateTable(ctx context.Context, t *model.Table) error
	// TablesName return all table name.
	TablesName(ctx context.Context) ([]*model.Table, error)
	// Insert set records in DB
	Insert(ctx context.Context, t *model.Table) error
	// List get records in the specified table
	List(ctx context.Context, tableName string) (*model.Table, error)
	// Header get table header name.
	Header(ctx context.Context, tableName string) (*model.Table, error)
	// Query execute "SELECT" or "EXPLAIN" query
	Query(ctx context.Context, query string) (*model.Table, error)
	// Exec execute "INSERT" or "UPDATE" or "DELETE" statement
	Exec(ctx context.Context, statement string) (int64, error)
	// ExecSQL executes "SELECT/EXPLAIN" query or "INSERT/UPDATE/DELETE" statement
	ExecSQL(ctx context.Context, statement string) (*model.Table, int64, error)

	// LoadFiles loads multiple files or directories into the database
	LoadFiles(ctx context.Context, filePaths ...string) error
	// GetTableNames returns the list of tables in the database
	GetTableNames(ctx context.Context) ([]*model.Table, error)
	// IsSupportedFile checks if the file has a format supported by filesql
	IsSupportedFile(filePath string) bool
	// IsExcelFile checks if the file is an Excel format
	IsExcelFile(filePath string) bool
	// SanitizeForSQL sanitizes a string to be SQL-safe
	SanitizeForSQL(name string) string
	// QuoteIdentifier safely quotes a SQL identifier
	QuoteIdentifier(identifier string) string
	// GetTableNameFromFilePath derives a table name from a file path
	GetTableNameFromFilePath(filePath string) string
}
