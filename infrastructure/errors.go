// Package infrastructure manage sqly infrastructure logic.
package infrastructure

import "errors"

// ErrNoLabel is error when label not found during LTSV parsing
var ErrNoLabel = errors.New("no labels in the data")
