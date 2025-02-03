package usecase

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// TSVUsecase handle TSV file.
type TSVUsecase interface {
	// List get TSV data.
	List(tsvFilePath string) (*model.TSV, error)
	// Dump write contents of DB table to TSV file.
	Dump(tsvFilePath string, table *model.Table) error
}
