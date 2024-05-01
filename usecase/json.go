package usecase

import (
	"os"

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

// Dump write contents of DB table to JSON file
func (i *JSONInteractor) Dump(jsonFilePath string, table *model.Table) error {
	f, err := os.OpenFile(jsonFilePath, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	return i.Repository.Dump(f, table)
}
