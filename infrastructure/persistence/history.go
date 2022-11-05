// Package persistence handle sqlite3, csv
package persistence

import (
	"context"
	"database/sql"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	infra "github.com/nao1215/sqly/infrastructure"
)

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

	q := "CREATE TABLE IF NOT EXISTS `history` (id INTEGER PRIMARY KEY AUTOINCREMENT, request TEXT)"
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

// Create set record in DB
func (h *historyRepository) Create(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	tx, err := h.db.BeginTx(ctx, nil)
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

// List get sql shell all history.
func (h *historyRepository) List(ctx context.Context) (model.Histories, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		"SELECT `id`, `request` FROM `history` ORDER BY `id` ASC")
	if err != nil {
		return nil, err
	}

	var id int
	var request string
	histories := model.Histories{}
	for rows.Next() {
		if err := rows.Scan(&id, &request); err != nil {
			return nil, err
		}
		histories = append(histories, &model.History{
			ID:      id,
			Request: request,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return histories, nil
}
