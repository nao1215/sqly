package shell

import "github.com/nao1215/sqly/usecase"

// Usecases is a structure that holds the usecase layer.
type Usecases struct {
	csv     usecase.CSVUsecase
	tsv     usecase.TSVUsecase
	ltsv    usecase.LTSVUsecase
	json    usecase.JSONUsecase
	sqlite3 usecase.DatabaseUsecase
	history usecase.HistoryUsecase
	excel   usecase.ExcelUsecase
}

// NewUsecases return *usecases that is assigned the result of parsing os.Args.
func NewUsecases(
	csv usecase.CSVUsecase,
	tsv usecase.TSVUsecase,
	ltsv usecase.LTSVUsecase,
	json usecase.JSONUsecase,
	sqlite3 usecase.DatabaseUsecase,
	history usecase.HistoryUsecase,
	excel usecase.ExcelUsecase,
) Usecases {
	return Usecases{
		csv:     csv,
		tsv:     tsv,
		ltsv:    ltsv,
		json:    json,
		sqlite3: sqlite3,
		history: history,
		excel:   excel,
	}
}
