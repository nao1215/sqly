package interactor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
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

// DumpTable exports a table to a file in the specified format.
func (e *exportInteractor) DumpTable(filePath string, table *model.Table, format model.ExportFormat) error {
	switch format {
	case model.ExportCSV:
		return e.dumpWithFile(filePath, table, e.csvRepo.Dump)
	case model.ExportTSV:
		return e.dumpWithFile(filePath, table, e.tsvRepo.Dump)
	case model.ExportLTSV:
		return e.dumpWithFile(filePath, table, e.ltsvRepo.Dump)
	case model.ExportExcel:
		return e.excelRepo.Dump(filepath.Clean(filePath), table)
	case model.ExportMarkdown:
		return e.dumpMarkdown(filePath, table)
	default:
		return e.dumpWithFile(filePath, table, e.csvRepo.Dump)
	}
}

// dumpWithFile creates a file and writes table data using the provided dump function.
func (e *exportInteractor) dumpWithFile(filePath string, table *model.Table, dumpFunc func(*os.File, *model.Table) error) (err error) {
	f, err := e.fileRepo.Create(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("create output file %q: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close output file %q: %w", filePath, cerr)
		}
	}()
	if err = dumpFunc(f, table); err != nil {
		return fmt.Errorf("dump to %q: %w", filePath, err)
	}
	return nil
}

// dumpMarkdown writes table data to file in Markdown table format.
func (e *exportInteractor) dumpMarkdown(filePath string, table *model.Table) error {
	f, err := os.Create(filepath.Clean(filePath)) // #nosec G304 - path is validated by caller
	if err != nil {
		return fmt.Errorf("failed to create markdown file %s: %w", filePath, err)
	}
	defer f.Close()

	return table.Print(f, model.PrintModeMarkdownTable)
}
