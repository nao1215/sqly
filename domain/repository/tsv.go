package repository

import (
	"io"

	"github.com/nao1215/sqly/domain/model"
)

// TSVRepository is a repository that handles TSV file export.
type TSVRepository interface {
	// Dump write contents of DB table to a TSV writer. Taking an io.Writer rather
	// than *os.File lets callers wrap the destination with a compression codec.
	Dump(tsv io.Writer, table *model.Table) error
}
