package usecase

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

type (
	// CSVUsecase handle CSV file.
	CSVUsecase interface {
		// List get CSV data.
		List(csvFilePath string) (*model.CSV, error)
		// Dump write contents of DB table to CSV file.
		Dump(csvFilePath string, table *model.Table) error
	}
)
