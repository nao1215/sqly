// Package repository abstract the infrastructure layer.
package repository

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

// SQLite3Repository is a repository that handles SQLite3.
type SQLite3Repository interface {
	// CreateTable create a DB table with columns given as model.Table
	CreateTable(ctx context.Context, t *model.Table) error
	// TablesName return all table name.
	TablesName(ctx context.Context) ([]*model.Table, error)
	// Insert set records in DB
	Insert(ctx context.Context, t *model.Table) error
	// Query execute SELECT query
	Query(ctx context.Context, query string) (*model.Table, error)
}
