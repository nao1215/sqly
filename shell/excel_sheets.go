package shell

import (
	"fmt"
	"sort"

	"github.com/nao1215/sqly/domain/model"
)

// A workbook that quietly contributes fewer tables than it has sheets is the
// one way sqly's Excel default can surprise someone. Importing only the sheets
// a workbook shows is the right default — a hidden sheet usually holds the
// spreadsheet's own working-out — but "right" is no help to a user staring at a
// missing table. So an import that left sheets behind says so.
//
// What it says is a count, not a list. The names of hidden sheets are the part
// of a workbook its author chose not to present, and printing them into a
// terminal (or into a log, or a CI transcript) publishes exactly what hiding
// them was meant to avoid. --inspect names them, because asking for a report of
// what a file holds is asking for precisely that; an ordinary query is not.

// excelWorkbookImport is what one workbook held when it was imported: every
// sheet in workbook order, and the table each imported sheet became.
//
// It is recorded at import time rather than read back when --inspect runs,
// because by then the file may be gone. A workbook fetched over HTTP is staged
// into a temp directory that the import's cleanup removes, and re-reading the
// URL would be a second download that could return something else entirely.
type excelWorkbookImport struct {
	// source is the path or URL the user named, not the staged copy.
	source string
	// sheets is every sheet the workbook held, in workbook order.
	sheets []model.ExcelSheet
	// tables maps an imported sheet's name to the table it became. A sheet the
	// policy left out is absent.
	tables map[string]string
}

// recordExcelSheets remembers what a workbook held at the moment it was
// imported, so --inspect can report the sheets that did not become tables and
// the import notice can count them.
//
// A workbook whose sheets cannot be read, or whose sheet names cannot be mapped
// onto tables, is not recorded. The import that just succeeded proves the file
// was readable, so this is the unlikely case of the file changing underfoot;
// there is nothing useful to say about it here, and the import itself already
// reported whatever it found.
func (s *Shell) recordExcelSheets(loadPath, displayPath string) {
	if !s.usecases.importer.IsExcelFile(loadPath) {
		return
	}
	sheets, err := s.usecases.importer.ExcelSheets(loadPath)
	if err != nil {
		return
	}

	loadedNames := make([]string, 0, len(sheets))
	for _, sheet := range sheets {
		if sheet.Visible || s.usecases.importer.IncludeHiddenSheets() {
			loadedNames = append(loadedNames, sheet.Name)
		}
	}
	tableNames, err := s.usecases.importer.ExcelSheetTableNames(loadPath, loadedNames)
	if err != nil {
		return
	}
	tables := make(map[string]string, len(loadedNames))
	for i, name := range loadedNames {
		tables[name] = tableNames[i]
	}

	s.excelWorkbooks = append(s.excelWorkbooks, excelWorkbookImport{
		source: displayPath,
		sheets: sheets,
		tables: tables,
	})
}

// warnSkippedExcelSheets reports on stderr how many sheets an import left
// behind because the workbook hides them. It is a no-op for a workbook with
// nothing hidden, a session importing hidden sheets anyway, and an --inspect
// run, whose report already lists every sheet.
func (s *Shell) warnSkippedExcelSheets(loadPath, displayPath string) {
	s.recordExcelSheets(loadPath, displayPath)
	if s.reportOnly() || s.usecases.importer.IncludeHiddenSheets() {
		return
	}
	record, ok := s.lastExcelWorkbook(displayPath)
	if !ok {
		return
	}
	hidden := len(model.HiddenExcelSheets(record.sheets))
	if hidden == 0 {
		return
	}
	noun, pronoun := "sheets", "them"
	if hidden == 1 {
		noun, pronoun = "sheet", "it"
	}
	fmt.Fprintf(s.importStatusWriter(),
		"Skipped %d hidden %s in %s; use --include-hidden-sheets to import %s.\n",
		hidden, noun, displayPath, pronoun)
}

// lastExcelWorkbook returns the most recent record for a source, which is the
// one the import that just ran produced.
func (s *Shell) lastExcelWorkbook(source string) (excelWorkbookImport, bool) {
	for i := len(s.excelWorkbooks) - 1; i >= 0; i-- {
		if s.excelWorkbooks[i].source == source {
			return s.excelWorkbooks[i], true
		}
	}
	return excelWorkbookImport{}, false
}

// excelSheetReports renders every recorded workbook for the --inspect report,
// in a fixed order: sources sorted by name, sheets in the order the workbook
// stores them.
//
// Re-importing the same source replaces its earlier record rather than adding a
// second one, so a session that imported a workbook twice reports it once, as
// it stands now.
func (s *Shell) excelSheetReports() []inspectExcelSheet {
	latest := make(map[string]excelWorkbookImport, len(s.excelWorkbooks))
	sources := make([]string, 0, len(s.excelWorkbooks))
	for _, record := range s.excelWorkbooks {
		if _, seen := latest[record.source]; !seen {
			sources = append(sources, record.source)
		}
		latest[record.source] = record
	}
	sort.Strings(sources)

	var reports []inspectExcelSheet
	for _, source := range sources {
		record := latest[source]
		for _, sheet := range record.sheets {
			table, imported := record.tables[sheet.Name]
			reports = append(reports, inspectExcelSheet{
				Source:   source,
				Name:     sheet.Name,
				Visible:  sheet.Visible,
				Imported: imported,
				Table:    table,
			})
		}
	}
	return reports
}
