package usecase

import "github.com/nao1215/sqly/domain/repository"

// HistoryInteractor implementation of use cases related to sqly history
type HistoryInteractor struct {
	Repository repository.HistoryRepository
}

// NewHistoryInteractor return CSVInteractor
func NewHistoryInteractor(r repository.HistoryRepository) *HistoryInteractor {
	return &HistoryInteractor{Repository: r}
}
