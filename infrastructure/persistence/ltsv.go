package persistence

import (
	"encoding/csv"
	"os"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure"
)

// _ interface implementation check
var _ repository.LTSVRepository = (*ltsvRepository)(nil)

type ltsvRepository struct{}

// NewLTSVRepository return LTSVRepository
func NewLTSVRepository() repository.LTSVRepository {
	return &ltsvRepository{}
}

// Dump write contents of DB table to LTSV file
func (lr *ltsvRepository) Dump(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)
	w.Comma = '\t'

	records := make([][]string, 0, len(table.Records()))
	for _, v := range table.Records() {
		r := make(model.Record, 0, len(v))
		for i, data := range v {
			r = append(r, table.Header()[i]+":"+data)
		}
		records = append(records, r)
	}
	return w.WriteAll(records)
}

// labelAndData split label and data.
func (lr *ltsvRepository) labelAndData(s string) (string, string, error) {
	idx := strings.Index(s, ":")
	if idx == -1 || idx == 0 {
		return "", "", infrastructure.ErrNoLabel
	}
	return s[:idx], s[idx+1:], nil
}
