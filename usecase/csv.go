package usecase

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// CSVInteractor implementation of use cases related to CSV handler.
type CSVInteractor struct {
	Repository repository.CSVRepository
}

// NewCSVInteractor return CSVInteractor
func NewCSVInteractor(r repository.CSVRepository) *CSVInteractor {
	return &CSVInteractor{Repository: r}
}

// List get CSV data.
// The sqly command does not open many CSV files. Therefore, the file is
// opened and closed in the usecase layer without worrying about processing speed.
func (ci *CSVInteractor) List(csvFilePath string) (*model.CSV, error) {
	f, err := os.Open(csvFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	csv, err := ci.Repository.List(f)
	if err != nil {
		return nil, err
	}

	return csv, nil
}
