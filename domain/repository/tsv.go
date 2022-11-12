package repository

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
)

// TSVRepository is a repository that handles TSV file.
type TSVRepository interface {
	// List get tsv all data with header.
	List(tsv *os.File) (*model.TSV, error)
}
