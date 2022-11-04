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
	// ShowTables return all table name.
	ShowTables(ctx context.Context) ([]*model.Table, error)
	// Insert set records in DB
	Insert(ctx context.Context, t *model.Table) error
	// Exec execute query
	Exec(ctx context.Context, query string) error
}
