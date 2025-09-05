package filesql

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// sqlite3Repository implements SQLite3Repository using filesql
type sqlite3Repository struct {
	adapter *FileSQLAdapter
}

// NewSQLite3Repository creates a new SQLite3Repository using filesql
func NewSQLite3Repository(adapter *FileSQLAdapter) repository.SQLite3Repository {
	return &sqlite3Repository{
		adapter: adapter,
	}
}

// CreateTable creates a DB table with columns given as model.Table
// For filesql integration, tables are created when files are loaded
func (r *sqlite3Repository) CreateTable(_ context.Context, _ *model.Table) error {
	// With filesql, tables are automatically created when files are loaded
	// This method is kept for interface compatibility but may not be needed
	return nil
}

// TablesName returns all table names
func (r *sqlite3Repository) TablesName(ctx context.Context) ([]*model.Table, error) {
	return r.adapter.GetTableNames(ctx)
}

// Insert sets records in DB
// For filesql, data is loaded from files, so this is primarily for compatibility
func (r *sqlite3Repository) Insert(_ context.Context, _ *model.Table) error {
	// With filesql, data insertion happens during file loading
	// This method is kept for interface compatibility
	return nil
}

// List gets records in the specified table
func (r *sqlite3Repository) List(ctx context.Context, tableName string) (*model.Table, error) {
	query := "SELECT * FROM " + tableName
	return r.adapter.Query(ctx, query)
}

// Header gets table header name
func (r *sqlite3Repository) Header(ctx context.Context, tableName string) (*model.Table, error) {
	return r.adapter.GetTableHeader(ctx, tableName)
}

// Query executes "SELECT" or "EXPLAIN" query
func (r *sqlite3Repository) Query(ctx context.Context, query string) (*model.Table, error) {
	return r.adapter.Query(ctx, query)
}

// Exec executes "INSERT" or "UPDATE" or "DELETE" statement
func (r *sqlite3Repository) Exec(ctx context.Context, statement string) (int64, error) {
	return r.adapter.Exec(ctx, statement)
}
