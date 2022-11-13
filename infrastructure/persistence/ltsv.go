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

func (lr *ltsvRepository) labelAndData(s string) (string, string, error) {
	idx := strings.Index(s, ":")
	if idx == -1 || idx == 0 {
		return "", "", infrastructure.ErrNoLabel
	}
	return s[:idx], s[idx+1:], nil
}
