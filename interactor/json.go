package interactor

import (
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.JSONUsecase = (*jsonInteractor)(nil)

// jsonInteractor implementation of use cases related to JSON handler.
type jsonInteractor struct {
	r repository.JSONRepository
}

// NewJSONInteractor return JSONInteractor
func NewJSONInteractor(r repository.JSONRepository) usecase.JSONUsecase {
	return &jsonInteractor{r: r}
}

// List get JSON data.
func (i *jsonInteractor) List(jsonFilePath string) (*model.JSON, error) {
	return i.r.List(jsonFilePath)
}

// Dump write contents of DB table to JSON file
func (i *jsonInteractor) Dump(jsonFilePath string, table *model.Table) error {
	f, err := os.OpenFile(filepath.Clean(jsonFilePath), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return i.r.Dump(f, table)
}
