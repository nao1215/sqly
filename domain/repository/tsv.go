package repository

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
)

// TSVRepository is a repository that handles TSV file export.
type TSVRepository interface {
	// Dump write contents of DB table to TSV file
	Dump(tsv *os.File, table *model.Table) error
}
