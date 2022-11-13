package usecase

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// LTSVInteractor implementation of use cases related to LTSV handler.
type LTSVInteractor struct {
	Repository repository.LTSVRepository
}

// NewLTSVInteractor return LTSVInteractor
func NewLTSVInteractor(r repository.LTSVRepository) *LTSVInteractor {
	return &LTSVInteractor{Repository: r}
}

// List get LTSV data.
func (li *LTSVInteractor) List(ltsvFilePath string) (*model.LTSV, error) {
	f, err := os.Open(ltsvFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	TSV, err := li.Repository.List(f)
	if err != nil {
		return nil, err
	}
	return TSV, nil
}

// Dump write contents of DB table to LTSV file
func (li *LTSVInteractor) Dump(ltsvFilePath string, table *model.Table) error {
	f, err := os.OpenFile(ltsvFilePath, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	return li.Repository.Dump(f, table)
}
