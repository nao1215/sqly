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

type tsvRepository struct{}

// NewTSVRepository return TSVRepository
func NewTSVRepository() repository.TSVRepository {
	return &tsvRepository{}
}

// List return tsv all record.
func (tr *tsvRepository) List(f *os.File) (*model.TSV, error) {
	r := csv.NewReader(f)
	r.Comma = '\t'

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
		records = append(records, row)
	}
	return model.NewTSV(filepath.Base(f.Name()), header, records), nil
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
