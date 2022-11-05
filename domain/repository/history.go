package repository

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

// HistoryRepository is a repository that handles sqly shell history.
type HistoryRepository interface {
	// CreateTable create a DB table for sqly shell history
	CreateTable(ctx context.Context) error
	// Insert set history record in DB
	Create(ctx context.Context, t *model.Table) error
}
