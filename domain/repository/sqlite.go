// Package repository abstract the infrastructure layer.
package repository

import (
	"context"
	"errors"

	"github.com/nao1215/sqly/domain/model"
)

// ErrNoRows is returned by SQLite3Repository.Query when a statement produced no
// result columns (for example a setter or command PRAGMA routed through the query
// path). It is part of the Query contract so callers in any layer can detect the
// no-rowset case with errors.Is without depending on the infrastructure layer.
var ErrNoRows = errors.New("execute query, however return no records")

//go:generate mockgen -typed -source=$GOFILE -destination=../../infrastructure/mock/$GOFILE -package mock

// SQLite3Repository is a repository that handles SQLite3.
type SQLite3Repository interface {
	// CreateTable create a DB table with columns given as model.Table
	CreateTable(ctx context.Context, t *model.Table) error
	// TablesName return all table name.
	TablesName(ctx context.Context) ([]*model.Table, error)
	// SchemaObjects returns every queryable table and view in the session,
	// including TEMP tables and views, for enumeration by .tables.
	SchemaObjects(ctx context.Context) ([]*model.Table, error)
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
}
