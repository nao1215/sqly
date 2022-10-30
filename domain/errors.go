package domain

import "errors"

var (
	// ErrEmptyTableName means table name is not set.
	ErrEmptyTableName = errors.New("table name is not set")
	// ErrEmptyHeader means header value is empty.
	ErrEmptyHeader = errors.New("header value is empty")
	// ErrEmptyRecords means records are empty.
	ErrEmptyRecords = errors.New("records is empty")
)
