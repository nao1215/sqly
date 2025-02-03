package interactor

import (
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.CSVUsecase = (*csvInteractor)(nil)

// csvInteractor implementation of use cases related to CSV handler.
type csvInteractor struct {
	r repository.CSVRepository
}

// NewCSVInteractor return CSVInteractor
func NewCSVInteractor(r repository.CSVRepository) usecase.CSVUsecase {
	return &csvInteractor{r: r}
}

// List get CSV data.
// The sqly command does not open many CSV files. Therefore, the file is
// opened and closed in the usecase layer without worrying about processing speed.
func (ci *csvInteractor) List(csvFilePath string) (*model.CSV, error) {
	f, err := os.Open(filepath.Clean(csvFilePath))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	csv, err := ci.r.List(f)
	if err != nil {
		return nil, err
	}

	return csv, nil
}

// Dump write contents of DB table to CSV file
func (ci *csvInteractor) Dump(csvFilePath string, table *model.Table) error {
	f, err := os.OpenFile(filepath.Clean(csvFilePath), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return ci.r.Dump(f, table)
}
