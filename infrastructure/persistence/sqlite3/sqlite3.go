// Package sqlite3 handle sqlite3 database.
package sqlite3

import (
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
func (cr *sqlite3Repository) CreateTable(t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}
	_, err := cr.db.Exec(generateCreateTableStatement((t)))
	if err != nil {
		return err
	}
	return nil
}

// Insert set records in DB
func (cr *sqlite3Repository) Insert(t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	tx, err := cr.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// TODO: Improvement in execution speed
	for _, v := range t.Records {
		if _, err := tx.Exec(generateInsertStatement(t.Name, v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (cr *sqlite3Repository) Exec(query string) error {
	tx, err := cr.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		aa := ""
		err := rows.Scan(&aa)
		if err != nil {
			return err
		}
		fmt.Println(aa)
	}
	err = rows.Err()
	if err != nil {
		return err
	}
	fmt.Println(query)
	return tx.Commit()
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
