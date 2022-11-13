package infrastructure

import "errors"

var (
	// ErrNoRows is same as sql.ErrNoRows
	ErrNoRows = errors.New("execute query, however return no records")
	// ErrNoLabel is error when label not found during LTSV parsing
	ErrNoLabel = errors.New("no labels in the data")
)
