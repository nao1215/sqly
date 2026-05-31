package repository

import (
	"io"

	"github.com/nao1215/sqly/domain/model"
)

// LTSVRepository is a repository that handles LTSV file export.
type LTSVRepository interface {
	// Dump write contents of DB table to an LTSV writer. Taking an io.Writer rather
	// than *os.File lets callers wrap the destination with a compression codec.
	Dump(ltsv io.Writer, table *model.Table) error
}
