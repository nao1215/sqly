package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// SQLite3Interactor implementation of use cases related to SQLite3 handler.
type SQLite3Interactor struct {
	Repository repository.SQLite3Repository
}

// NewSQLite3Interactor return CSVInteractor
func NewSQLite3Interactor(r repository.SQLite3Repository) *SQLite3Interactor {
	return &SQLite3Interactor{Repository: r}
}

// CreateTable create a DB table with columns given as model.Table
func (si *SQLite3Interactor) CreateTable(ctx context.Context, t *model.Table) error {
	return si.Repository.CreateTable(ctx, t)
}

// TablesName return all table name.
func (si *SQLite3Interactor) TablesName(ctx context.Context) ([]*model.Table, error) {
	return si.Repository.TablesName(ctx)
}

// Insert set records in DB
func (si *SQLite3Interactor) Insert(ctx context.Context, t *model.Table) error {
	return si.Repository.Insert(ctx, t)
}

// Query execute query
func (si *SQLite3Interactor) Query(ctx context.Context, query string) (*model.Table, error) {
	return si.Repository.Query(ctx, query)
}
