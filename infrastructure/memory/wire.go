package memory

import "github.com/google/wire"

// Set is memory wire set.
var Set = wire.NewSet(
	NewSQLite3Repository,
)
