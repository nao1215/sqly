package interactor

import (
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.CSVUsecase = (*csvInteractor)(nil)

// csvInteractor implementation of use cases related to CSV handler.
type csvInteractor struct {
	*baseFileInteractor
	r repository.CSVRepository
}

// NewCSVInteractor return CSVInteractor
func NewCSVInteractor(
	filesqlAdapter *filesql.FileSQLAdapter,
	r repository.CSVRepository,
	f repository.FileRepository,
) usecase.CSVUsecase {
	return &csvInteractor{
		baseFileInteractor: newBaseFileInteractor(filesqlAdapter, f),
		r:                  r,
	}
}

// List get CSV data using filesql for improved performance and compression support.
func (ci *csvInteractor) List(csvFilePath string) (*model.Table, error) {
	return ci.list(csvFilePath, "CSV")
}

// Dump write contents of DB table to CSV file
func (ci *csvInteractor) Dump(csvFilePath string, table *model.Table) error {
	return ci.dump(csvFilePath, table, ci.r.Dump)
}
