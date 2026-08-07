package repository

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../../infrastructure/mock/$GOFILE -package mock

// HistoryRepository is a repository that handles sqly shell history.
type HistoryRepository interface {
	// Init prepares the history store and reports whether it can be written.
	// A session calls it once at startup so an unwritable location disables
	// history with one warning rather than failing on every line typed.
	Init(ctx context.Context) error
	// Append adds one entry to the history.
	Append(ctx context.Context, history model.History) error
	// List returns the retained history, oldest first.
	List(ctx context.Context) (model.Histories, error)
}
