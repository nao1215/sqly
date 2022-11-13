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
func (ti *LTSVInteractor) List(LTSVFilePath string) (*model.LTSV, error) {
	f, err := os.Open(LTSVFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	TSV, err := ti.Repository.List(f)
	if err != nil {
		return nil, err
	}
	return TSV, nil
}
