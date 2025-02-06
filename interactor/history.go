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

// CreateTable create table for sqly history.
func (hi *historyInteractor) CreateTable(ctx context.Context) error {
	return hi.r.CreateTable(ctx)
}

// Create create history record.
func (hi *historyInteractor) Create(ctx context.Context, history model.History) error {
	h := model.Histories{history}
	return hi.r.Create(ctx, h.ToTable())
}

// List get all sqly history.
func (hi *historyInteractor) List(ctx context.Context) (model.Histories, error) {
	return hi.r.List(ctx)
}
