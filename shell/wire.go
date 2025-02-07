package shell

import "github.com/google/wire"

// Set is shell wire set.
var Set = wire.NewSet(
	NewShell,
	NewCommands,
	NewUsecases,
)
