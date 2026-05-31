// Package repository abstract the infrastructure layer.
package repository

import (
	"io"

	"github.com/nao1215/sqly/domain/model"
)

// CSVRepository is a repository that handles CSV file export.
type CSVRepository interface {
	// Dump write contents of DB table to a CSV writer. Taking an io.Writer rather
	// than *os.File lets callers wrap the destination with a compression codec.
	Dump(csv io.Writer, table *model.Table) error
}
