package persistence

import (
	"github.com/google/wire"
)

// Set is persistence wire set.
var Set = wire.NewSet(
	NewCSVRepository,
	NewTSVRepository,
	NewLTSVRepository,
	NewJSONRepository,
	NewExcelRepository,
	NewHistoryRepository,
	NewFileRepository,
)
