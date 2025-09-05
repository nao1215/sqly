package persistence

import (
	"github.com/google/wire"
)

// Set is persistence wire set.
var Set = wire.NewSet(
	NewCSVRepository,
	NewTSVRepository,
	NewLTSVRepository,
	NewExcelRepository,
	NewHistoryRepository,
	NewFileRepository,
)

// HistorySet is minimal persistence wire set for History functionality only.
var HistorySet = wire.NewSet(
	NewHistoryRepository,
)
