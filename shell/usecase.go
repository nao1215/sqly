package shell

import "github.com/nao1215/sqly/usecase"

// Usecases is a structure that holds the usecase layer.
// After consolidation, only three service boundaries remain:
// - sqlite3 (DatabaseUsecase): session operations including SQL execution and file import
// - history (HistoryUsecase): command history management
// - export (ExportUsecase): table export to various file formats
type Usecases struct {
	sqlite3 usecase.DatabaseUsecase
	history usecase.HistoryUsecase
	export  usecase.ExportUsecase
}

// NewUsecases return Usecases with all required usecase dependencies.
func NewUsecases(
	sqlite3 usecase.DatabaseUsecase,
	history usecase.HistoryUsecase,
	export usecase.ExportUsecase,
) Usecases {
	return Usecases{
		sqlite3: sqlite3,
		history: history,
		export:  export,
	}
}
