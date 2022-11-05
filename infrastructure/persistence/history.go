package persistence

import "github.com/nao1215/sqly/domain/repository"

type historyRepository struct{}

// NewHistoryRepository return HistoryRepository
func NewHistoryRepository() repository.HistoryRepository {
	return &historyRepository{}
}
