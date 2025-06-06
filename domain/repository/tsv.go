package repository

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -typed -source=$GOFILE -destination=../../infrastructure/mock/$GOFILE -package mock

// TSVRepository is a repository that handles TSV file.
type TSVRepository interface {
	// List get tsv all data with header.
	List(tsv *os.File) (*model.TSV, error)
	// Dump write contents of DB table to TSV file
	Dump(tsv *os.File, table *model.Table) error
}
