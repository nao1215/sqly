// Package repository abstract the infrastructure layer.
package repository

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination mock_$GOFILE

// CSVRepository is a repository that handles CSV file.
// The process of opening and closing CSV files is the responsibility of the upper layer.
type CSVRepository interface {
	// List get csv all data with header.
	List(csv *os.File) (*model.CSV, error)
	// Dump write contents of DB table to CSV file
	Dump(csv *os.File, table *model.Table) error
}
