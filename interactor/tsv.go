package interactor

import (
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.TSVUsecase = (*tsvInteractor)(nil)

// tsvInteractor implementation of use cases related to TSV handler.
type tsvInteractor struct {
	*baseFileInteractor
	r repository.TSVRepository
}

// NewTSVInteractor return TSVInteractor
func NewTSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.TSVRepository,
	f repository.FileRepository,
) usecase.TSVUsecase {
	return &tsvInteractor{
		baseFileInteractor: newBaseFileInteractor(filesqlAdapter, f),
		r:                  r,
	}
}

// List get TSV data using filesql for improved performance and compression support.
func (ti *tsvInteractor) List(tsvFilePath string) (*model.Table, error) {
	return ti.list(tsvFilePath, "TSV")
}

// Dump write contents of DB table to TSV file
func (ti *tsvInteractor) Dump(tsvFilePath string, table *model.Table) error {
	return ti.dump(tsvFilePath, table, ti.r.Dump)
}
