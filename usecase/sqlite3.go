package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// DatabaseUsecase handle Relational Database.
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
}
