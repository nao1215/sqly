package persistence

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// _ interface implementation check
var _ repository.CSVRepository = (*csvRepository)(nil)

type csvRepository struct{}

// NewCSVRepository return CSVRepository
func NewCSVRepository() repository.CSVRepository {
	return &csvRepository{}
}

// CSVRepository is a repository that handles CSV file.
// The process of opening and closing CSV files is the responsibility of the upper layer.
// TODO: convert from *** to UTF-8
func (cr *csvRepository) List(f *os.File) (*model.CSV, error) {
	r := csv.NewReader(f)

	header := model.Header{}
	records := []model.Record{}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if len(header) == 0 {
			header = row
			continue
		}
		records = append(records, model.NewRecord(row))
	}
	return model.NewCSV(filepath.Base(f.Name()), model.NewHeader(header), records), nil
}

// Dump write contents of DB table to CSV file
func (cr *csvRepository) Dump(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)

	records := make([][]string, 0, 1+len(table.Records()))
	records = append(records, table.Header())
	for _, v := range table.Records() {
		records = append(records, v)
	}
	return w.WriteAll(records)
}
