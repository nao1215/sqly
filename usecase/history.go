package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// HistoryUsecase handle sqly history.
type HistoryUsecase interface {
	// Init prepares the history store and reports whether it can be written, so
	// an unwritable location is found once at startup rather than per statement.
	Init(ctx context.Context) error
	// Append adds one entry to the history.
	Append(ctx context.Context, history model.History) error
	// List returns the retained history, oldest first.
	List(ctx context.Context) (model.Histories, error)
}
