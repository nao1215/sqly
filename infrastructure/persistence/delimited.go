package persistence

import (
	"encoding/csv"
	"io"

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

// loneEmptyField is how a record of one empty field is written.
//
// Written plainly it is a blank line, and a blank line is not a record: a reader
// skips it. A one-column result with an empty value therefore lost that row —
// SELECT v FROM t giving alice, "", bob wrote three rows and read back as two.
// The quotes say "one field, and it is empty", which cannot be read as anything
// else. encoding/csv's writer does not quote an empty field, because it has no
// way to know it is the only one on the line.
const loneEmptyField = `""`

// Dump write contents of DB table to a delimited writer
func (dr *delimitedRepository) Dump(f io.Writer, table *model.Table) error {
	w := csv.NewWriter(f)
	w.Comma = dr.delimiter

	records := make([][]string, 0, 1+table.RowCount())
	records = append(records, table.Header())
	for _, v := range table.Rows {
		records = append(records, v.AppendTo(make([]string, 0, v.Len())))
	}

	for _, record := range records {
		if len(record) == 1 && record[0] == "" {
			// Flushing first keeps the two writers' output in order.
			w.Flush()
			if err := w.Error(); err != nil {
				return err
			}
			if _, err := io.WriteString(f, loneEmptyField+"\n"); err != nil {
				return err
			}
			continue
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
