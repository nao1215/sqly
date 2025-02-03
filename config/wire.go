package config

import "github.com/google/wire"

// Set is config wire set.
var Set = wire.NewSet(
	NewConfig,
	NewInMemDB,
	NewHistoryDB,
	NewArg,
)
