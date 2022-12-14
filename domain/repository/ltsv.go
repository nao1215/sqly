package repository

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
)

// LTSVRepository is a repository that handles LTSV file.
type LTSVRepository interface {
	// List get ltsv all data with label.
	List(ltsv *os.File) (*model.LTSV, error)
	// Dump write contents of DB table to LTSV file
	Dump(ltsv *os.File, table *model.Table) error
}
