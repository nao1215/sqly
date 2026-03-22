package persistence

import (
	"encoding/csv"
	"os"

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
