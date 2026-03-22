package persistence

import (
	"encoding/csv"
	"os"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// delimitedRepository handles Dump for delimiter-separated formats (CSV, TSV).
type delimitedRepository struct {
	delimiter rune
}

// NewCSVRepository return CSVRepository
func NewCSVRepository() repository.CSVRepository {
	return &delimitedRepository{delimiter: ','}
}

// NewTSVRepository return TSVRepository
func NewTSVRepository() repository.TSVRepository {
	return &delimitedRepository{delimiter: '\t'}
}

// Dump write contents of DB table to a delimited file
func (dr *delimitedRepository) Dump(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)
	w.Comma = dr.delimiter

	records := make([][]string, 0, 1+len(table.Records()))
	records = append(records, table.Header())
	for _, v := range table.Records() {
		records = append(records, v)
	}
	return w.WriteAll(records)
}
