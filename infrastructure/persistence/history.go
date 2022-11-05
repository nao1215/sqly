package persistence

import (
	"context"
	"database/sql"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/repository"
)

const historyTableName = "`history`"

type historyRepository struct {
	db *sql.DB
}

// NewHistoryRepository return HistoryRepository
func NewHistoryRepository(db config.HistoryDB) repository.HistoryRepository {
	return &historyRepository{db: db}
}

// CreateTable create a DB table for sqly shell history
func (h *historyRepository) CreateTable(ctx context.Context) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := "CREATE TABLE IF NOT EXISTS `history` (id INTEGER, request TEXT)"
	_, err = tx.ExecContext(ctx, q)
	if err != nil {
		return err
	}

	q = "CREATE INDEX IF NOT EXISTS `history_id_index` ON `history`(`id`)"
	_, err = tx.ExecContext(ctx, q)
	if err != nil {
		return err
	}
	return tx.Commit()
}
