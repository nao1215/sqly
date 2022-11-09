package usecase

import (
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// JSONInteractor implementation of use cases related to JSON handler.
type JSONInteractor struct {
	Repository repository.JSONRepository
}

// NewJSONInteractor return JSONInteractor
func NewJSONInteractor(r repository.JSONRepository) *JSONInteractor {
	return &JSONInteractor{Repository: r}
}

// List get JSON data.
func (i *JSONInteractor) List(jsonFilePath string) (*model.JSON, error) {
	return i.Repository.List(jsonFilePath)
}
