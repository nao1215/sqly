package persistence

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/nao1215/sqly/domain/model"
	"github.com/xuri/excelize/v2"
)

// excelSheetNameMaxLen is Excel's hard limit on a worksheet name length.
const excelSheetNameMaxLen = 31

// excelForbiddenSheetChars are the characters Excel rejects in a worksheet name.
const excelForbiddenSheetChars = `:\/?*[]`

// excelSheetName adapts a table name to Excel's worksheet-name rules so an
// export never fails on excelize's NewSheet call. It replaces the forbidden
// characters (: \ / ? * [ ]) with '_', caps the length at 31 runes, and trims
// surrounding apostrophes (which Excel disallows at the edges), falling back to
// a default when nothing usable remains. A table name comes from the source
// filename, so a long or punctuated name is ordinary input, not user error.
func excelSheetName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if strings.ContainsRune(excelForbiddenSheetChars, r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	sheet := []rune(b.String())
	if len(sheet) > excelSheetNameMaxLen {
		sheet = sheet[:excelSheetNameMaxLen]
	}
	name = strings.Trim(string(sheet), "'")
	if name == "" {
		return "Sheet1"
	}
	return name
}

// DumpExcel writes contents of DB table to an XLSX file. It takes a path rather
// than an io.Writer because excelize builds the whole workbook in memory and
// saves it itself, so there is no stream for a compression codec to wrap.
func DumpExcel(excelFilePath string, table *model.Table) (err error) {
	f := excelize.NewFile()
	defer func() {
		if e := f.Close(); err != nil {
			err = errors.Join(err, e)
		}
	}()

	// Excel worksheet names are limited to 31 characters and cannot contain
	// : \ / ? * [ ], so the table name is adapted before it becomes a sheet.
	sheetName := excelSheetName(table.Name())

	// A new excelize file already has one sheet named "Sheet1"; rename it to the
	// target rather than adding a second sheet and deleting the default. Excelize
	// matches sheet names case-insensitively, so adding "sheet1" would collide
	// with the default and renaming sidesteps that entirely. When the target only
	// differs from "Sheet1" by case, keep the default name so row writes still
	// address the sheet that exists.
	const defaultSheet = "Sheet1"
	if strings.EqualFold(sheetName, defaultSheet) {
		sheetName = defaultSheet
	} else if err = f.SetSheetName(defaultSheet, sheetName); err != nil {
		return err
	}

	f.SetActiveSheet(0)
	header := table.Header()
	for _, label := range header {
		if err := ensureExcelRepresentable(label, label); err != nil {
			return err
		}
	}
	if err := model.EnsureHeaderReimportable("excel", header); err != nil {
		return err
	}
	if err := f.SetSheetRow(sheetName, "A1", &header); err != nil {
		return err
	}

	if err := ensureLastRowSurvives(table); err != nil {
		return err
	}

	const excelRowOffset = 2
	values := make([]string, 0, table.ColumnCount())
	cells := make([]any, table.ColumnCount())
	for i, record := range table.Rows {
		values = record.AppendTo(values[:0])
		for col, value := range values {
			if err := ensureExcelRepresentable(table.ColumnName(col), value); err != nil {
				return err
			}
			// A NULL is written as no cell at all, which is what a workbook has
			// instead of a null: a reader sees a blank cell rather than a cell
			// holding the empty string, and those are different values to every
			// tool that opens the file. Writing the empty string for both made an
			// export claim the column had a value everywhere.
			if col < len(cells) {
				if table.IsNull(i, col) {
					cells[col] = nil
					continue
				}
				cells[col] = value
			}
		}
		for col := len(values); col < len(cells); col++ {
			cells[col] = nil
		}
		if err := f.SetSheetRow(sheetName, fmt.Sprintf("A%d", i+excelRowOffset), &cells); err != nil {
			return err
		}
	}
	if err := f.SaveAs(excelFilePath); err != nil {
		return err
	}
	// excelize's SaveAs creates the file with os.ModePerm (0777), which leaves
	// the export executable. Reset to the same non-executable mode as other
	// outputs so .xlsx files are plain data files. Why not pass excelize
	// Options: SaveAs hard-codes the mode and ignores a permissions option.
	return os.Chmod(excelFilePath, defaultFilePerm)
}

// ensureLastRowSurvives reports an error when the table's last row holds nothing
// in every column, because a workbook cannot carry it.
//
// XLSX stores cells, not rows: a row whose every value is empty leaves no cell
// behind to mark where it was, and a reader counting rows stops at the last one
// that has a value. Writing such a table produced a file that read back a row
// short, with the export reporting success — three rows written, two read.
// Only the tail is at risk, since an empty row with data after it is found by
// the rows that follow it.
//
// Refusing is what the character check below already does for a value XLSX
// cannot carry: a file that cannot be read back is worse than an export that
// says why it stopped.
func ensureLastRowSurvives(table *model.Table) error {
	if table.RowCount() == 0 {
		return nil
	}
	last, ok := table.Row(table.RowCount() - 1)
	if !ok {
		return nil
	}
	for col := range table.ColumnCount() {
		if last.At(col) != "" {
			return nil
		}
	}
	return errors.New(
		"excel: the last row is empty in every column, and a workbook has no way to store it: the file would read back one row short; export to csv, tsv, json, or parquet instead, or add a column that is not empty")
}

// ensureExcelRepresentable reports an error when a value holds a character XLSX
// cannot carry, so the export is refused before a file is written.
//
// XLSX is XML, and XML 1.0's Char production is the whole rule: of the
// characters below U+0020 it allows only tab, line feed, and carriage return,
// and it stops the BMP at U+FFFD, excluding the two noncharacters above it. The
// writer substitutes U+FFFD for everything outside that, which makes an export
// silent data loss — the file appears, the process succeeds, and the value is
// gone. LTSV already refuses what it cannot represent rather than writing
// something that cannot be read back; this is the same rule for the same reason.
// The replacement character also has a meaning in sqly's output already: it says
// a file was read with the wrong --encoding, and an export that produces it on
// its own would make that signal a guess.
//
// Everything else passes: DEL and U+FFFD itself are written unchanged.
func ensureExcelRepresentable(label, value string) error {
	// Bytes that are not UTF-8 at all are checked first, because ranging over a
	// string cannot see them: Go decodes each invalid byte as U+FFFD, which
	// xmlCanCarry then passes, and the writer wrote U+FFFD in its place. That is
	// the substitution this function exists to stop, arriving through the one
	// door it was not watching. Such a value is almost always a file read with
	// the wrong --encoding, so the message says so.
	if !utf8.ValidString(value) {
		return fmt.Errorf("excel: value for column %q is not valid UTF-8, so XLSX cannot carry it unchanged; re-read the input with the right --encoding, or export to csv/tsv", label)
	}
	for _, r := range value {
		if !xmlCanCarry(r) {
			return fmt.Errorf("excel: value for column %q contains the character U+%04X, which XLSX cannot represent; remove it or export to csv/tsv/json", label, r)
		}
	}
	return nil
}

// xmlCanCarry reports whether XML 1.0 can hold r, following its Char production.
// A Go string never yields a surrogate when ranged over — invalid bytes decode to
// U+FFFD — so that half of the production cannot be reached from here. The caller
// rejects a value holding such bytes before this runs, so a U+FFFD reaching it is
// one the data really contained.
func xmlCanCarry(r rune) bool {
	switch {
	case r == '\t' || r == '\n' || r == '\r':
		return true
	case r < 0x20:
		return false
	case r == 0xFFFE || r == 0xFFFF:
		return false
	default:
		return true
	}
}
