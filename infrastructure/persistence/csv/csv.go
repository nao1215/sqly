// Package csv handle csv file.
package csv

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

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

	c := model.CSV{
		Name: filepath.Base(f.Name()),
	}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		if c.IsHeaderEmpty() {
			c.SetHeader(row)
			continue
		}
		c.SetRecord(row)
	}
	return &c, nil
}
