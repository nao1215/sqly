package interactor

import (
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.CSVUsecase = (*csvInteractor)(nil)

// csvInteractor implementation of use cases related to CSV handler.
type csvInteractor struct {
	f repository.FileRepository
	r repository.CSVRepository
}

// NewCSVInteractor return CSVInteractor
func NewCSVInteractor(
	f repository.FileRepository,
	r repository.CSVRepository,
) usecase.CSVUsecase {
	return &csvInteractor{
		f: f,
		r: r,
	}
}

// List get CSV data.
// The sqly command does not open many CSV files. Therefore, the file is
// opened and closed in the usecase layer without worrying about processing speed.
func (ci *csvInteractor) List(csvFilePath string) (*model.Table, error) {
	f, err := ci.f.Open(csvFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	csv, err := ci.r.List(f)
	if err != nil {
		return nil, err
	}
	return csv.ToTable(), nil
}

// Dump write contents of DB table to CSV file
func (ci *csvInteractor) Dump(csvFilePath string, table *model.Table) error {
	f, err := ci.f.Create(csvFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ci.r.Dump(f, table)
}
