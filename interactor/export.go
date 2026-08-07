package interactor

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/nao1215/sqly/domain/cleanup"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/usecase"
)

// _ interface implementation check
var _ usecase.ExportUsecase = (*exportInteractor)(nil)

// exportInteractor consolidates all format-specific dump operations into
// a single interactor. This replaces the individual Dump() methods that
// were previously scattered across csv/tsv/ltsv/excel interactors.
type exportInteractor struct {
	csvRepo   repository.CSVRepository
	tsvRepo   repository.TSVRepository
	ltsvRepo  repository.LTSVRepository
	excelRepo repository.ExcelRepository
	fileRepo  repository.FileRepository
}

// NewExportInteractor returns an ExportUsecase that can dump tables in any supported format.
func NewExportInteractor(
	csvRepo repository.CSVRepository,
	tsvRepo repository.TSVRepository,
	ltsvRepo repository.LTSVRepository,
	excelRepo repository.ExcelRepository,
	fileRepo repository.FileRepository,
) usecase.ExportUsecase {
	return &exportInteractor{
		csvRepo:   csvRepo,
		tsvRepo:   tsvRepo,
		ltsvRepo:  ltsvRepo,
		excelRepo: excelRepo,
		fileRepo:  fileRepo,
	}
}

// DumpTable exports a table to a file in the specified format. Text and JSON
// formats honor the compression codec; Excel and Parquet are binary container
// formats and ignore it (callers reject compression for them upstream).
func (e *exportInteractor) DumpTable(filePath string, table *model.Table, format model.ExportFormat, compression model.Compression) error {
	switch format {
	case model.ExportCSV:
		return e.dumpWithFile(filePath, table, compression, e.csvRepo.Dump)
	case model.ExportTSV:
		return e.dumpWithFile(filePath, table, compression, e.tsvRepo.Dump)
	case model.ExportLTSV:
		return e.dumpWithFile(filePath, table, compression, e.ltsvRepo.Dump)
	case model.ExportExcel:
		return e.excelRepo.Dump(filepath.Clean(filePath), table)
	case model.ExportMarkdown:
		return e.dumpViaPrint(filePath, table, compression, model.PrintModeMarkdownTable)
	case model.ExportJSON:
		return e.dumpViaPrint(filePath, table, compression, model.PrintModeJSON)
	case model.ExportJSONL:
		return e.dumpViaPrint(filePath, table, compression, model.PrintModeJSONL)
	case model.ExportParquet:
		return filesql.DumpTableToParquet(filepath.Clean(filePath), table)
	default:
		return e.dumpWithFile(filePath, table, compression, e.csvRepo.Dump)
	}
}

// dumpWithFile creates a file and writes table data using the provided dump
// function, wrapping the destination in a compression codec when requested.
func (e *exportInteractor) dumpWithFile(filePath string, table *model.Table, compression model.Compression, dumpFunc func(io.Writer, *model.Table) error) (err error) {
	return e.withCompressedWriter(filePath, compression, func(w io.Writer) error {
		return dumpFunc(w, table)
	})
}

// dumpViaPrint writes table data to file using Table.Print for formats whose
// file output matches their display rendering (Markdown, JSON, NDJSON). This
// reuses the rendering implementation instead of a separate repository.
func (e *exportInteractor) dumpViaPrint(filePath string, table *model.Table, compression model.Compression, mode model.PrintMode) (err error) {
	return e.withCompressedWriter(filePath, compression, func(w io.Writer) error {
		return table.Print(w, mode)
	})
}

// withCompressedWriter creates filePath, optionally wraps it in a compression
// codec, and passes the resulting writer to write. The codec is finalized before
// the file is closed (deferred close runs in reverse order), so all buffered
// compressed bytes reach disk.
//
// The bytes are written once, straight to filePath. Every caller already hands
// this a scratch path rather than the user's destination — .dump and --output
// through writeFileAtomically, .save through its own staging — and that is where
// "the previous file survives a failed export" is decided. Serializing into a
// second temporary file first, then copying it across, wrote every exported byte
// twice and made the export depend on free space in the OS temp directory as
// well as in the destination's own filesystem.
func (e *exportInteractor) withCompressedWriter(filePath string, compression model.Compression, write func(io.Writer) error) (err error) {
	file, err := e.fileRepo.Create(filePath)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", filePath, err)
	}
	defer func() {
		err = cleanup.Join(err, file.Close(), fmt.Sprintf("close output file %q", filePath))
	}()

	w, closeComp, err := filesql.NewCompressingWriter(file, compression)
	if err != nil {
		return fmt.Errorf("init compression for %q: %w", filePath, err)
	}

	compClosed := false
	finalizeComp := func() error {
		if compClosed {
			return nil
		}
		compClosed = true
		return closeComp()
	}

	defer func() {
		err = cleanup.Join(err, finalizeComp(), fmt.Sprintf("finalize compression for %q", filePath))
	}()

	if err = write(w); err != nil {
		return fmt.Errorf("dump to %q: %w", filePath, err)
	}

	// Finalize the codec here rather than leaving it to the deferred call, so a
	// codec that fails while flushing its tail is reported as the export failing
	// instead of as a cleanup note on a success.
	if err = finalizeComp(); err != nil {
		return fmt.Errorf("finalize compression for %q: %w", filePath, err)
	}

	return nil
}
