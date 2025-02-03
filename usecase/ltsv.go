package usecase

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// LTSVUsecase handle LTSV file.
type LTSVUsecase interface {
	// List get LTSV data.
	List(ltsvFilePath string) (*model.LTSV, error)
	// Dump write contents of DB table to LTSV file.
	Dump(ltsvFilePath string, table *model.Table) error
}
