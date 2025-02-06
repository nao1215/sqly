package interactor

import (
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.TSVUsecase = (*tsvInteractor)(nil)

// tsvInteractor implementation of use cases related to TSV handler.
type tsvInteractor struct {
	f repository.FileRepository
	r repository.TSVRepository
}

// NewTSVInteractor return TSVInteractor
func NewTSVInteractor(
	f repository.FileRepository,
	r repository.TSVRepository,
) usecase.TSVUsecase {
	return &tsvInteractor{
		f: f,
		r: r,
	}
}

// List get TSV data.
// The sqly command does not open many TSV files. Therefore, the file is
// opened and closed in the usecase layer without worrying about processing speed.
func (ti *tsvInteractor) List(tsvFilePath string) (*model.Table, error) {
	f, err := ti.f.Open(filepath.Clean(tsvFilePath))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tsv, err := ti.r.List(f)
	if err != nil {
		return nil, err
	}
	return tsv.ToTable(), nil
}

// Dump write contents of DB table to TSV file
func (ti *tsvInteractor) Dump(tsvFilePath string, table *model.Table) error {
	f, err := ti.f.Create(filepath.Clean(tsvFilePath))
	if err != nil {
		return err
	}
	defer f.Close()

	return ti.r.Dump(f, table)
}
