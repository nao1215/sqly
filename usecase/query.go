package usecase

import (
	"context"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// QueryUsecase executes SQL against the in-memory database.
// Commands that run user SQL depend on this interface only, not on import or
// metadata capabilities.
type QueryUsecase interface {
	// Query execute "SELECT" or "EXPLAIN" query
	Query(ctx context.Context, query string) (*model.Table, error)
	// ExecSQL executes "SELECT/EXPLAIN" query or "INSERT/UPDATE/DELETE" statement.
	// It is the only entry point for a statement the user typed: it decides by
	// shape whether the statement produces rows, so a caller never has to.
	ExecSQL(ctx context.Context, statement string) (*model.Table, int64, error)
	// ExecSQLTo runs a statement like ExecSQL, but hands a rowset to sink a row
	// at a time instead of returning it. rowset reports whether the statement
	// produced one; when it did not, affected is its affected-row count.
	ExecSQLTo(ctx context.Context, statement string, sink model.RowSink) (rowset bool, affected int64, err error)
	// SetDialect sets the SQL dialect applied to subsequent user queries run via
	// ExecSQL. Loading and internally generated statements always use SQLite.
	SetDialect(d dialect.Dialect)
	// Dialect returns the current SQL dialect.
	Dialect() dialect.Dialect
}
