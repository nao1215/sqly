package interactor

import (
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.LTSVUsecase = (*ltsvInteractor)(nil)

// ltsvInteractor implementation of use cases related to LTSV handler.
type ltsvInteractor struct {
	*baseFileInteractor
	r repository.LTSVRepository
}

// NewLTSVInteractor return ltsvInteractor
func NewLTSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.LTSVRepository,
	f repository.FileRepository,
) usecase.LTSVUsecase {
	return &ltsvInteractor{
		baseFileInteractor: newBaseFileInteractor(filesqlAdapter, f),
		r:                  r,
	}
}

// List get LTSV data using filesql for improved performance and compression support.
func (li *ltsvInteractor) List(ltsvFilePath string) (*model.Table, error) {
	return li.list(ltsvFilePath, "LTSV")
}

// Dump write contents of DB table to LTSV file
func (li *ltsvInteractor) Dump(ltsvFilePath string, table *model.Table) error {
	return li.dump(ltsvFilePath, table, li.r.Dump)
}
