package shell

import "github.com/nao1215/sqly/usecase"

// Usecases holds the usecase-layer dependencies the shell consumes.
// The database session is split into focused interfaces so each command
// depends only on the capability it uses:
//   - query (QueryUsecase): SQL execution
//   - importer (ImportUsecase): file import and table-name helpers
//   - metadata (MetadataUsecase): table listing, header, and record inspection
//   - history (HistoryUsecase): command history management
//   - export (ExportUsecase): table export to various file formats
type Usecases struct {
	query    usecase.QueryUsecase
	importer usecase.ImportUsecase
	metadata usecase.MetadataUsecase
	history  usecase.HistoryUsecase
	export   usecase.ExportUsecase
}

// NewUsecases return Usecases with all required usecase dependencies.
func NewUsecases(
	query usecase.QueryUsecase,
	importer usecase.ImportUsecase,
	metadata usecase.MetadataUsecase,
	history usecase.HistoryUsecase,
	export usecase.ExportUsecase,
) Usecases {
	return Usecases{
		query:    query,
		importer: importer,
		metadata: metadata,
		history:  history,
		export:   export,
	}
}
