package interactor

import (
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.JSONUsecase = (*jsonInteractor)(nil)

// jsonInteractor implementation of use cases related to JSON handler.
type jsonInteractor struct {
	f repository.FileRepository
	r repository.JSONRepository
}

// NewJSONInteractor return JSONInteractor
func NewJSONInteractor(
	f repository.FileRepository,
	r repository.JSONRepository,
) usecase.JSONUsecase {
	return &jsonInteractor{
		f: f,
		r: r,
	}
}

// List get JSON data.
func (i *jsonInteractor) List(jsonFilePath string) (*model.Table, error) {
	json, err := i.r.List(jsonFilePath)
	if err != nil {
		return nil, err
	}
	return json.ToTable(), nil
}

// Dump write contents of DB table to JSON file
func (i *jsonInteractor) Dump(jsonFilePath string, table *model.Table) error {
	f, err := i.f.Create(filepath.Clean(jsonFilePath))
	if err != nil {
		return err
	}
	defer f.Close()

	return i.r.Dump(f, table)
}
