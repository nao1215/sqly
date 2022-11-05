package repository

import (
	"context"
)

// HistoryRepository is a repository that handles sqly shell history.
type HistoryRepository interface {
	// CreateTable create a DB table for sqly shell history
	CreateTable(ctx context.Context) error
}
