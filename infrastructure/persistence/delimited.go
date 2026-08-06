package persistence

import (
	"io"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// delimitedRepository handles Dump for delimiter-separated formats (CSV, TSV).
type delimitedRepository struct {
	mode model.PrintMode
}

// NewCSVRepository return CSVRepository
func NewCSVRepository() repository.CSVRepository {
	return &delimitedRepository{mode: model.PrintModeCSV}
}

// NewTSVRepository return TSVRepository
func NewTSVRepository() repository.TSVRepository {
	return &delimitedRepository{mode: model.PrintModeTSV}
}

// Dump write contents of DB table to a delimited writer.
//
// The table prints itself: a second writer here meant a second set of rules, and
// only this one had learned to quote a lone empty field.
func (dr *delimitedRepository) Dump(f io.Writer, table *model.Table) error {
	return table.Print(f, dr.mode)
}
