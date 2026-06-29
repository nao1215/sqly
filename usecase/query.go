package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// QueryUsecase executes SQL against the in-memory database.
// Commands that run user SQL depend on this interface only, not on import or
// metadata capabilities.
type QueryUsecase interface {
	// Query execute "SELECT" or "EXPLAIN" query
	Query(ctx context.Context, query string) (*model.Table, error)
	// QueryStream executes a "SELECT"/"EXPLAIN" query and streams each result row
	// to fn, so callers can aggregate without materializing the whole result set.
	// fn receives one row's cell strings and a per-cell SQL NULL flag; returning an
	// error stops the scan.
	QueryStream(ctx context.Context, query string, fn func(record []string, nulls []bool) error) error
	// Exec execute "INSERT" or "UPDATE" or "DELETE" statement
	Exec(ctx context.Context, statement string) (int64, error)
	// ExecSQL executes "SELECT/EXPLAIN" query or "INSERT/UPDATE/DELETE" statement
	ExecSQL(ctx context.Context, statement string) (*model.Table, int64, error)
}
