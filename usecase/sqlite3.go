package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// SQLite3Interactor implementation of use cases related to SQLite3 handler.
type SQLite3Interactor struct {
	Repository repository.SQLite3Repository
	sql        *SQL
}

// NewSQLite3Interactor return CSVInteractor
func NewSQLite3Interactor(r repository.SQLite3Repository, sql *SQL) *SQLite3Interactor {
	return &SQLite3Interactor{
		Repository: r,
		sql:        sql,
	}
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

// List get records in the specified table
func (si *SQLite3Interactor) List(ctx context.Context, tableName string) (*model.Table, error) {
	return si.Repository.List(ctx, tableName)
}

// Header get table header name.
func (si *SQLite3Interactor) Header(ctx context.Context, tableName string) (*model.Table, error) {
	return si.Repository.Header(ctx, tableName)
}

// Query execute "SELECT" or "EXPLAIN" query
func (si *SQLite3Interactor) Query(ctx context.Context, query string) (*model.Table, error) {
	return si.Repository.Query(ctx, query)
}

// Exec execute "INSERT" or "UPDATE" or "DELETE" statement
func (si *SQLite3Interactor) Exec(ctx context.Context, statement string) (int64, error) {
	return si.Repository.Exec(ctx, statement)
}

// ExecSQL executes "SELECT/EXPLAIN" query or "INSERT/UPDATE/DELETE" statement
func (si *SQLite3Interactor) ExecSQL(ctx context.Context, statement string) (*model.Table, int64, error) {
	argv := strings.Split(trimWordGaps(statement), " ")

	// NOTE: SQLY uses SQLite3. There is some SQL that can be changed from non-support
	// to support in the future. Currently, it is not supported because it is not needed
	// for developer ( == me:) ) use cases.
	switch {
	case si.sql.isDDL(argv[0]):
		return nil, 0, errors.New("not support data definition language: " + strings.Join(si.sql.ddl, ", "))
	case si.sql.isTCL(argv[0]):
		return nil, 0, errors.New("not support transaction control language: " + strings.Join(si.sql.tcl, ", "))
	case si.sql.isDCL(argv[0]):
		return nil, 0, errors.New("not support data control language: " + strings.Join(si.sql.dcl, ", "))
	case !si.sql.isDML(argv[0]):
		return nil, 0, errors.New("this input is not sql query or sqly helper command: " + color.CyanString(statement))
	case si.sql.isSelect(argv[0]) || si.sql.isExplain(argv[0]):
		table, err := si.Query(ctx, statement)
		if err != nil {
			return nil, 0, fmt.Errorf("execute query error: %w: %s", err, color.CyanString(statement))
		}
		return table, 0, nil
	case si.sql.isInsert(argv[0]) || si.sql.isUpdate(argv[0]) || si.sql.isDelete(argv[0]):
		affectedRows, err := si.Repository.Exec(ctx, statement)
		if err != nil {
			return nil, 0, fmt.Errorf("execute statement error: %w: %s", err, color.CyanString(statement))
		}
		return nil, affectedRows, nil
	default:
		return nil, 0, fmt.Errorf("%s\n%s: %s\n%s",
			color.HiRedString("*** sqly bug ***"),
			"please report this log", color.CyanString(statement),
			"Web site: https://github.com/nao1215/sqly/issues")
	}
}
