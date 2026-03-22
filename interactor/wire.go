// Package interactor implements the usecase layer.
package interactor

import (
	"github.com/google/wire"
)

// Set is interactor wire set.
var Set = wire.NewSet(
	NewSQLite3Interactor,
	NewHistoryInteractor,
	NewFileSQLInteractor,
	NewExportInteractor,
	NewSQL,
)
