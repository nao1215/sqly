package repository

import (
	"github.com/nao1215/sqly/domain/model"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination mock_$GOFILE

// JSONRepository is a repository that handles JSON file.
type JSONRepository interface {
	// List get csv all data with header.
	List(jsonFilePath string) (*model.JSON, error)
}
