package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// HistoryInteractor implementation of use cases related to sqly history
type HistoryInteractor struct {
	Repository repository.HistoryRepository
}

// NewHistoryInteractor return CSVInteractor
func NewHistoryInteractor(r repository.HistoryRepository) *HistoryInteractor {
	return &HistoryInteractor{Repository: r}
}

// CreateTable create table for sqly history.
func (hi *HistoryInteractor) CreateTable(ctx context.Context) error {
	return hi.Repository.CreateTable(ctx)
}

// Create create history record.
func (hi *HistoryInteractor) Create(ctx context.Context, history model.History) error {
	h := model.Histories{&history}
	return hi.Repository.Create(ctx, h.ToTable())
}
