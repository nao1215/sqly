// Package memory handle sqlite3 in memory mode
package memory

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	infra "github.com/nao1215/sqly/infrastructure"
)

type sqlite3Repository struct {
	db *sql.DB
}

// NewSQLite3Repository return sqlite3Repository
func NewSQLite3Repository(db config.MemoryDB) repository.SQLite3Repository {
	return &sqlite3Repository{db: db}
}

// CreateTable create a DB table with columns given as model.Table
func (r *sqlite3Repository) CreateTable(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, infra.GenerateCreateTableStatement((t)))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// TablesName return all table name.
func (r *sqlite3Repository) TablesName(ctx context.Context) ([]*model.Table, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []*model.Table{}
	var name string
	for rows.Next() {
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, &model.Table{Name: name})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tables, nil
}

// Insert set records in DB
func (r *sqlite3Repository) Insert(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, v := range t.Records {
		if _, err := tx.ExecContext(ctx, infra.GenerateInsertStatement(t.Name, v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// List get records in the specified table
func (r *sqlite3Repository) List(ctx context.Context, tableName string) (*model.Table, error) {
	table, err := r.Query(ctx, "SELECT * FROM "+infra.Quote(tableName))
	if err != nil {
		return nil, err
	}
	table.Name = tableName
	return table, nil
}

// Header get table header name.
func (r *sqlite3Repository) Header(ctx context.Context, tableName string) (*model.Table, error) {
	return r.Query(ctx, fmt.Sprintf("SELECT * FROM %s LIMIT 1", infra.Quote(tableName)))
}

// Query execute "SELECT" or "EXPLAIN" query
func (r *sqlite3Repository) Query(ctx context.Context, query string) (*model.Table, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	header, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(header) == 0 {
		return nil, infra.ErrNoRows
	}

	table := model.Table{
		Name:   "",
		Header: header,
	}

	scanDest := make([]interface{}, len(header))
	rawResult := make([][]byte, len(header))

	for i := range header {
		scanDest[i] = &rawResult[i]
	}

	for rows.Next() {
		result := make([]string, len(header))
		err := rows.Scan(scanDest...)
		if err != nil {
			return nil, err
		}

		for i, raw := range rawResult {
			result[i] = string(raw)
		}
		table.Records = append(table.Records, result)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &table, nil
}

// Exec execute "INSERT" or "UPDATE" or "DELETE" statement
func (r *sqlite3Repository) Exec(ctx context.Context, statement string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, statement)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
