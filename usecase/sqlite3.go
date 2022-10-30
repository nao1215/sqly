package usecase

import (
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

func (si *SQLite3Interactor) CreateTable(t *model.Table) error {
	return si.Repository.CreateTable(t)
}

func (si *SQLite3Interactor) Insert(t *model.Table) error {
	return si.Repository.Insert(t)
}

func (si *SQLite3Interactor) Exec(query string) error {
	return si.Repository.Exec(query)
}
