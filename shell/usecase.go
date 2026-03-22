package shell

import "github.com/nao1215/sqly/usecase"

// Usecases is a structure that holds the usecase layer.
type Usecases struct {
	sqlite3 usecase.DatabaseUsecase
	history usecase.HistoryUsecase
	filesql usecase.FileSQLUsecase
	export  usecase.ExportUsecase
}

// NewUsecases return Usecases with all required usecase dependencies.
func NewUsecases(
	sqlite3 usecase.DatabaseUsecase,
	history usecase.HistoryUsecase,
	filesql usecase.FileSQLUsecase,
	export usecase.ExportUsecase,
) Usecases {
	return Usecases{
		sqlite3: sqlite3,
		history: history,
		filesql: filesql,
		export:  export,
	}
}
