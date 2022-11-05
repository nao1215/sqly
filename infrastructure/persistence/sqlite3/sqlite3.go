// Package sqlite3 handle sqlite3 database.
package sqlite3

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

type sqlite3Repository struct {
	db *sql.DB
}

// NewSQLite3Repository return sqlite3Repository
func NewSQLite3Repository(db *sql.DB) repository.SQLite3Repository {
	return &sqlite3Repository{db: db}
}

// CreateTable create a DB table with columns given as model.Table
func (r *sqlite3Repository) CreateTable(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, generateCreateTableStatement((t)))
	if err != nil {
		return err
	}
	return nil
}

// TablesName return all table name.
func (r *sqlite3Repository) TablesName(ctx context.Context) ([]*model.Table, error) {
	res, err := r.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		return nil, err
	}

	tables := []*model.Table{}
	var name string
	for res.Next() {
		if err := res.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, &model.Table{Name: name})
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

	// TODO: Improvement in execution speed
	for _, v := range t.Records {
		if _, err := tx.ExecContext(ctx, generateInsertStatement(t.Name, v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Exec execute query
func (r *sqlite3Repository) Exec(ctx context.Context, query string) (*model.Table, error) {
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
	fmt.Println(table.Records)

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &table, nil
}

func generateCreateTableStatement(t *model.Table) string {
	ddl := "CREATE TABLE " + quote(t.Name) + "("
	for i, v := range t.Header {
		ddl += quote(v)
		if i != len(t.Header)-1 {
			ddl += ", "
		} else {
			ddl += ");"
		}
	}
	return ddl
}

func generateInsertStatement(name string, record model.Record) string {
	dml := "INSERT INTO " + quote(name) + " VALUES ("
	for i, v := range record {
		dml += singleQuote(v)
		if i != len(record)-1 {
			dml += ", "
		} else {
			dml += ");"
		}
	}
	return dml
}
