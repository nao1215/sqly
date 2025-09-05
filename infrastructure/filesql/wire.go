package filesql

import (
	"github.com/google/wire"
	"github.com/nao1215/sqly/usecase"
)

// Set is a Wire provider set for filesql infrastructure
var Set = wire.NewSet(
	NewFileSQLAdapter,
	NewSQLite3Repository,
	NewCSVInteractor,
	NewTSVInteractor,
	NewLTSVInteractor,
	NewExcelInteractor,
	// We still need these from the original interactor
	wire.Bind(new(usecase.DatabaseUsecase), new(*sqlite3Repository)),
)
