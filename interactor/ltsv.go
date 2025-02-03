package interactor

import (
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.LTSVUsecase = (*LTSVInteractor)(nil)

// LTSVInteractor implementation of use cases related to LTSV handler.
type LTSVInteractor struct {
	f repository.FileRepository
	r repository.LTSVRepository
}

// NewLTSVInteractor return LTSVInteractor
func NewLTSVInteractor(
	f repository.FileRepository,
	r repository.LTSVRepository,
) usecase.LTSVUsecase {
	return &LTSVInteractor{
		f: f,
		r: r,
	}
}

// List get LTSV data.
func (li *LTSVInteractor) List(ltsvFilePath string) (*model.LTSV, error) {
	f, err := li.f.Open(filepath.Clean(ltsvFilePath))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return li.r.List(f)
}

// Dump write contents of DB table to LTSV file
func (li *LTSVInteractor) Dump(ltsvFilePath string, table *model.Table) error {
	f, err := li.f.Create(filepath.Clean(ltsvFilePath))
	if err != nil {
		return err
	}
	defer f.Close()

	return li.r.Dump(f, table)
}
