package persistence

import (
	"encoding/csv"
	"os"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

type tsvRepository struct{}

// NewTSVRepository return TSVRepository
func NewTSVRepository() repository.TSVRepository {
	return &tsvRepository{}
}

// Dump write contents of DB table to TSV file
func (tr *tsvRepository) Dump(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)
	w.Comma = '\t'

	records := make([][]string, 0, 1+len(table.Records()))
	records = append(records, table.Header())
	for _, v := range table.Records() {
		records = append(records, v)
	}
	return w.WriteAll(records)
}
