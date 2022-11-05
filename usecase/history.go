package usecase

import (
	"context"

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
