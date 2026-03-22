// Package repository abstract the infrastructure layer.
package repository

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
)

// CSVRepository is a repository that handles CSV file export.
type CSVRepository interface {
	// Dump write contents of DB table to CSV file
	Dump(csv *os.File, table *model.Table) error
}
