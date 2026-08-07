package interactor

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.HistoryUsecase = (*historyInteractor)(nil)

// historyInteractor implementation of use cases related to sqly history
type historyInteractor struct {
	r repository.HistoryRepository
}

// NewHistoryInteractor return CSVInteractor
func NewHistoryInteractor(r repository.HistoryRepository) usecase.HistoryUsecase {
	return &historyInteractor{r: r}
}

// Init prepares the history store and reports whether it can be written.
func (hi *historyInteractor) Init(ctx context.Context) error {
	return hi.r.Init(ctx)
}

// Append adds one entry to the history.
func (hi *historyInteractor) Append(ctx context.Context, history model.History) error {
	return hi.r.Append(ctx, history)
}

// List get all sqly history.
func (hi *historyInteractor) List(ctx context.Context) (model.Histories, error) {
	return hi.r.List(ctx)
}
