package persistence

import (
	"database/sql"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/repository"
)

type historyRepository struct {
	db *sql.DB
}

// NewHistoryRepository return HistoryRepository
func NewHistoryRepository(db config.HistoryDB) repository.HistoryRepository {
	return &historyRepository{db: db}
}
