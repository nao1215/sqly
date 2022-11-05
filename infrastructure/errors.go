package infrastructure

import "errors"

var (
	// ErrNoRows is same as sql.ErrNoRows
	ErrNoRows = errors.New("execute query, however return no records")
)
