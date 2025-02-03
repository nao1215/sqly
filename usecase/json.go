package usecase

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// JSONUsecase handle JSON file.
type JSONUsecase interface {
	// List get JSON data.
	List(jsonFilePath string) (*model.JSON, error)
	// Dump write contents of DB table to JSON file.
	Dump(jsonFilePath string, table *model.Table) error
}
