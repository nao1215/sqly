package repository

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
)

// JSONRepository is a repository that handles JSON file.
type JSONRepository interface {
	// List get csv all data with header.
	List(jsonFilePath string) (*model.JSON, error)
	// Dump write contents of DB table to JSON file
	Dump(f *os.File, table *model.Table) error
}
