package shell

import "github.com/nao1215/sqly/usecase"

// Usecases is a structure that holds the usecase layer.
type Usecases struct {
	csv     usecase.CSVUsecase
	tsv     usecase.TSVUsecase
	ltsv    usecase.LTSVUsecase
	sqlite3 usecase.DatabaseUsecase
	history usecase.HistoryUsecase
	excel   usecase.ExcelUsecase
	filesql usecase.FileSQLUsecase
}

// NewUsecases return *usecases that is assigned the result of parsing os.Args.
func NewUsecases(
	csv usecase.CSVUsecase,
	tsv usecase.TSVUsecase,
	ltsv usecase.LTSVUsecase,
	sqlite3 usecase.DatabaseUsecase,
	history usecase.HistoryUsecase,
	excel usecase.ExcelUsecase,
	filesql usecase.FileSQLUsecase,
) Usecases {
	return Usecases{
		csv:     csv,
		tsv:     tsv,
		ltsv:    ltsv,
		sqlite3: sqlite3,
		history: history,
		excel:   excel,
		filesql: filesql,
	}
}
