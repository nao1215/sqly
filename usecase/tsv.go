package usecase

import (
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// TSVInteractor implementation of use cases related to TSV handler.
type TSVInteractor struct {
	Repository repository.TSVRepository
}

// NewTSVInteractor return TSVInteractor
func NewTSVInteractor(r repository.TSVRepository) *TSVInteractor {
	return &TSVInteractor{Repository: r}
}

// List get TSV data.
// The sqly command does not open many TSV files. Therefore, the file is
// opened and closed in the usecase layer without worrying about processing speed.
func (ti *TSVInteractor) List(tsvFilePath string) (*model.TSV, error) {
	f, err := os.Open(filepath.Clean(tsvFilePath))
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint

	TSV, err := ti.Repository.List(f)
	if err != nil {
		return nil, err
	}
	return TSV, nil
}

// Dump write contents of DB table to TSV file
func (ti *TSVInteractor) Dump(tsvFilePath string, table *model.Table) error {
	f, err := os.OpenFile(filepath.Clean(tsvFilePath), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint

	return ti.Repository.Dump(f, table)
}
