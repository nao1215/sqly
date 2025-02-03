package persistence

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure"
)

// _ interface implementation check
var _ repository.LTSVRepository = (*ltsvRepository)(nil)

type ltsvRepository struct{}

// NewLTSVRepository return TSVRepository
func NewLTSVRepository() repository.LTSVRepository {
	return &ltsvRepository{}
}

// List return tsv all record.
func (lr *ltsvRepository) List(f *os.File) (*model.LTSV, error) {
	r := csv.NewReader(f)
	r.Comma = '\t'

	t := model.LTSV{
		Name: filepath.Base(f.Name()),
	}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		if t.IsLabelEmpty() {
			label := model.Label{}
			for _, v := range row {
				l, _, err := lr.labelAndData(v)
				if err != nil {
					return nil, err
				}
				label = append(label, l)
			}
			t.Label = label
		}

		r := model.Record{}
		for _, v := range row {
			_, data, err := lr.labelAndData(v)
			if err != nil {
				return nil, err
			}
			r = append(r, data)
		}
		t.SetRecord(r)
	}
	return &t, nil
}

// Dump write contents of DB table to LTSV file
func (lr *ltsvRepository) Dump(f *os.File, table *model.Table) error {
	w := csv.NewWriter(f)
	w.Comma = '\t'

	records := [][]string{}
	for _, v := range table.Records {
		r := model.Record{}
		for i, data := range v {
			r = append(r, table.Header[i]+":"+data)
		}
		records = append(records, r)
	}
	return w.WriteAll(records)
}

func (lr *ltsvRepository) labelAndData(s string) (string, string, error) {
	idx := strings.Index(s, ":")
	if idx == -1 || idx == 0 {
		return "", "", infrastructure.ErrNoLabel
	}
	return s[:idx], s[idx+1:], nil
}
