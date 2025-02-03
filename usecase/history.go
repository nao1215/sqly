package usecase

import (
	"context"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// HistoryUsecase handle sqly history.
type HistoryUsecase interface {
	// CreateTable create table for sqly history.
	CreateTable(ctx context.Context) error
	// Create create history record.
	Create(ctx context.Context, history model.History) error
	// List get all sqly history.
	List(ctx context.Context) (model.Histories, error)
}
