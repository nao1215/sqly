package persistence

import (
	"io"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
)

// This file is the set of serializers an export can use.
//
// Each one used to sit behind an interface of its own — CSVRepository,
// TSVRepository, LTSVRepository, ExcelRepository, FileRepository — with one
// implementation, one consumer, and one method. Nothing ever substituted them:
// the mocks that justified the indirection were used to test the wiring the
// indirection created. Adding a format meant an interface, an implementation, a
// constructor, a line of dependency injection, and a regenerated mock, for a
// choice that is "which function writes this format".

// defaultFilePerm is the permission for files sqly writes. It is non-executable
// (0600) so exports are treated as ordinary data files, consistent across CSV,
// TSV, LTSV, Parquet, and Excel outputs.
const defaultFilePerm = 0o600

// DumpCSV writes the table as CSV.
//
// The table prints itself here, as in the TSV case: a second writer meant a
// second set of rules, and only one of the two had learned to quote a lone empty
// field.
func DumpCSV(w io.Writer, table *model.Table) error {
	return table.Print(w, model.PrintModeCSV)
}

// DumpTSV writes the table as TSV.
func DumpTSV(w io.Writer, table *model.Table) error {
	return table.Print(w, model.PrintModeTSV)
}

// CreateExportFile creates the file an export writes to, truncating it if it
// already exists, and returns it open for writing. O_TRUNC is required so
// overwriting an existing file with shorter content does not leave stale
// trailing bytes that corrupt the file.
func CreateExportFile(path string) (*os.File, error) {
	return os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultFilePerm)
}
