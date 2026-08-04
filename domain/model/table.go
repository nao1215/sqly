package model

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nao1215/sqly/domain"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// columnData extracts one column's display strings, reading the table's own
// storage rather than a copy of every row.
func (t *Table) columnData(columnIndex int) []string {
	var columnData []string
	for _, record := range t.Rows {
		if columnIndex < record.Len() {
			columnData = append(columnData, record.At(columnIndex))
		}
	}
	return columnData
}

// IsNumericValue reports whether s is what a human would call a number. It
// strips comma thousands separators ("1,000") and surrounding whitespace, then
// requires a finite decimal. It rejects the Go-specific float spellings
// ParseFloat also accepts but data rarely means as numbers: hexadecimal floats
// ("0x1p4"), underscore digit separators ("1_000"), and the Infinity/NaN words.
//
// This is the numeric contract used by table-mode right alignment. Keeping the
// predicate in the model layer avoids duplicating display rules elsewhere.
func IsNumericValue(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	if strings.ContainsAny(s, "xXpP_") {
		return false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return false
	}
	return !math.IsInf(f, 0) && !math.IsNaN(f)
}

// isAllNumeric checks if all values in a column look like numbers, skipping
// blank cells. It uses IsNumericValue so column alignment follows one contract.
func isAllNumeric(values []string) bool {
	if len(values) == 0 {
		return false
	}

	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if !IsNumericValue(v) {
			return false
		}
	}
	return true
}

// PrintMode is enum to specify output method
type PrintMode uint

// Format name constants shared between PrintMode and ExportFormat.
const (
	formatCSV      = "csv"
	formatTSV      = "tsv"
	formatLTSV     = "ltsv"
	formatMarkdown = "markdown"
	formatExcel    = "excel"
	formatJSON     = "json"
	formatJSONL    = "jsonl"
	formatParquet  = "parquet"
	formatVertical = "vertical"
)

// Extension name constants.
const (
	ExtCSV      = ".csv"
	ExtTSV      = ".tsv"
	ExtLTSV     = ".ltsv"
	ExtMarkdown = ".md"
	ExtExcel    = ".xlsx"
	ExtJSON     = ".json"
	ExtJSONL    = ".jsonl"
	ExtNDJSON   = ".ndjson"
	ExtParquet  = ".parquet"
)

const (
	// PrintModeTable print data in table format
	PrintModeTable PrintMode = iota
	// PrintModeMarkdownTable print data in markdown table format
	PrintModeMarkdownTable
	// PrintModeCSV print data in csv format
	PrintModeCSV
	// PrintModeTSV print data in tsv format
	PrintModeTSV
	// PrintModeLTSV print data in ltsv format
	PrintModeLTSV
	// PrintModeExcel print data in excel format
	PrintModeExcel
	// PrintModeJSON print data as a JSON array of objects
	PrintModeJSON
	// PrintModeJSONL print data as newline-delimited JSON (one object per line)
	PrintModeJSONL
	// PrintModeParquet is an export-only mode; on screen it renders like CSV and
	// only writes a Parquet file via .dump or --output (same pattern as Excel).
	PrintModeParquet
	// PrintModeVertical prints one column per line, in a block per record. It is
	// display-only, like PrintModeTable: a row wider than the terminal is what the
	// table, csv, tsv, and ltsv modes all fail at, and a 300-column row is the case
	// sqly exists for.
	PrintModeVertical
)

// String return string of PrintMode.
func (p PrintMode) String() string {
	switch p {
	case PrintModeTable:
		return "table"
	case PrintModeMarkdownTable:
		return formatMarkdown
	case PrintModeCSV:
		return formatCSV
	case PrintModeTSV:
		return formatTSV
	case PrintModeLTSV:
		return formatLTSV
	case PrintModeExcel:
		return formatExcel
	case PrintModeJSON:
		return formatJSON
	case PrintModeJSONL:
		return formatJSONL
	case PrintModeParquet:
		return formatParquet
	case PrintModeVertical:
		return formatVertical
	}
	return "unknown"
}

// IsDisplayOnly reports whether the mode only decides what the screen looks like,
// so it names no export format.
//
// The export path asks this to decide whether the mode chose the destination's
// format or the destination's extension did: `.dump out.tsv` while the session is
// in a display-only mode writes a TSV, where the same call in csv mode is a
// conflict the caller has to resolve. Table and vertical are the two — vertical is
// a way of reading a wide row, not a file format anything else can parse back.
func (p PrintMode) IsDisplayOnly() bool {
	return p == PrintModeTable || p == PrintModeVertical
}

// AllowsMultipleResults reports whether this format can carry more than one
// result set in a single stream.
//
// A format a person reads can: two tables, two vertical blocks, or two Markdown
// tables separated by a blank line are still exactly what they look like. A
// format a program parses cannot. Two CSV bodies concatenated are one CSV whose
// third line is a second header row; two JSON arrays back to back are not a JSON
// document; and JSONL has no way to say "a new result starts here". Emitting
// those anyway produces a file that parses — into the wrong thing, or not at
// all — which is worse than refusing, so a run that would need one is rejected.
func (p PrintMode) AllowsMultipleResults() bool {
	switch p {
	case PrintModeTable, PrintModeVertical, PrintModeMarkdownTable:
		return true
	default:
		return false
	}
}

// Table is DB table.
type Table struct {
	// Name is table name.
	name string
	// Header is table header.
	header Header
	// Records is table records: the display string of every cell, which is what
	// the table, CSV, TSV, LTSV, and Markdown formats write.
	records []Record
	// cells optionally holds the driver's native value for every cell of a query
	// result, stored row-major in a single allocation of columns per row rather
	// than one slice per row.
	// It is the source the records above were derived from, so a cell's string
	// form and its JSON form always describe the same value. It is nil for a
	// Table built from strings (an imported file, a synthesized report), where
	// there is no type or NULL information to preserve and every cell is TEXT.
	cells []Cell
	// columns is the row width of cells. It is header's length; it is stored so
	// indexing does not depend on the header slice a caller may have replaced.
	columns int
}

// NewTable create new Table from string records. Use it for tables that have no
// database types to preserve — an imported file, a synthesized report — where
// every cell is TEXT and no cell is SQL NULL. Query results should be built with
// NewTableFromCells so their INTEGER/REAL/TEXT/NULL distinctions survive to the
// JSON and Parquet writers.
//
// The caller passes ownership of records; Table does not copy them.
func NewTable(
	name string,
	header Header,
	records []Record,
) *Table {
	return &Table{
		name:    name,
		header:  header,
		records: records,
	}
}

// NewTableFromCells builds a query-result table from the driver's native cell
// values. Each row must hold exactly one cell per header column; a row of any
// other width returns ErrCellShapeMismatch rather than being carried into a
// formatter, where the mismatch would surface only after part of the output had
// already been written.
//
// The cells are copied into storage the Table owns, so mutating rows afterwards
// cannot change the Table, and Records() is derived from them once here, so the
// displayed string and the JSON scalar of a cell can never disagree.
func NewTableFromCells(name string, header Header, rows [][]Cell) (*Table, error) {
	columns := len(header)
	flat := make([]Cell, 0, len(rows)*columns)
	records := make([]Record, 0, len(rows))
	for i, row := range rows {
		if len(row) != columns {
			return nil, fmt.Errorf("%w: row %d has %d cells, header has %d columns", ErrCellShapeMismatch, i, len(row), columns)
		}
		record := make(Record, columns)
		for j, cell := range row {
			record[j] = cell.String()
		}
		flat = append(flat, row...)
		records = append(records, record)
	}
	return &Table{
		name:    name,
		header:  header,
		records: records,
		cells:   flat,
		columns: columns,
	}, nil
}

// cell returns the native cell at (row, col) and whether the table carries
// native values at all.
func (t *Table) cell(row, col int) (Cell, bool) {
	if t.cells == nil || t.columns == 0 || col < 0 || col >= t.columns || row < 0 {
		return Cell{}, false
	}
	idx := row*t.columns + col
	if idx >= len(t.cells) {
		return Cell{}, false
	}
	return t.cells[idx], true
}

// IsNull reports whether the cell at (row, col) is a known SQL NULL, as opposed
// to an empty string. It returns false when no NULL information is available
// (the table did not come from a query).
func (t *Table) IsNull(row, col int) bool {
	c, ok := t.cell(row, col)
	return ok && c.IsNull()
}

// Name return table name.
func (t *Table) Name() string {
	return t.name
}

// WithName returns a copy with a different table name, preserving the native
// cell values so a rename does not downgrade a query result to strings — .dump
// and the describe path both re-wrap a result under the user's table name, and
// JSON output of the renamed table has to keep emitting numbers and nulls.
//
// The header, the record slice, and the cell slice are all cloned, so the two
// tables share no mutable storage: renaming a column on one cannot rename it on
// the other, and appending to one cannot write into the other's backing array.
// The row contents themselves are shared, which is safe because nothing can
// reach them except through a RecordView or a copy.
func (t *Table) WithName(name string) *Table {
	cloned := &Table{
		name:    name,
		columns: t.columns,
	}
	if t.header != nil {
		cloned.header = append(make(Header, 0, len(t.header)), t.header...)
	}
	if t.records != nil {
		cloned.records = append(make([]Record, 0, len(t.records)), t.records...)
	}
	if t.cells != nil {
		cloned.cells = append(make([]Cell, 0, len(t.cells)), t.cells...)
	}
	return cloned
}

// Header returns a copy of the table's column names.
//
// The copy is why `table.Header()[0] = "corrupted"` cannot rename a column
// behind the table's back. Code that only needs to read the names should use
// ColumnCount, ColumnName, or Columns, which allocate nothing.
func (t *Table) Header() Header {
	if t.header == nil {
		return nil
	}
	return append(make(Header, 0, len(t.header)), t.header...)
}

// ColumnCount returns the number of columns without copying the header.
func (t *Table) ColumnCount() int {
	return len(t.header)
}

// ColumnName returns the name of column i, or the empty string when i is
// outside the header. It reads the header without copying it.
func (t *Table) ColumnName(i int) string {
	if i < 0 || i >= len(t.header) {
		return ""
	}
	return t.header[i]
}

// Columns iterates the column names in order without copying them.
func (t *Table) Columns(yield func(int, string) bool) {
	for i, name := range t.header {
		if !yield(i, name) {
			return
		}
	}
}

// Records returns a copy of the table's records: the display string of every
// cell, one Record per row.
//
// The copy is deep, so writing to the result cannot reach the Table:
//
//	records := table.Records()
//	records[0][0] = "corrupted" // affects only the caller's copy
//
// This matters beyond ordinary defensiveness. A query result's strings are
// derived from its native cells, and the JSON and Parquet writers read those
// cells; a caller able to edit the strings in place could make the same table
// print "corrupted" as CSV and the original value as JSON. Handing out the
// internal slice made that a one-line mistake, and no test could have noticed
// which of the two representations was right afterwards.
//
// Copying is the price of a safe public API. Code inside sqly that walks every
// row — the formatters, the exporters, the INSERT builder — should use Rows or
// RowCount instead, which read the same storage without copying it.
func (t *Table) Records() []Record {
	if t.records == nil {
		return nil
	}
	records := make([]Record, len(t.records))
	for i, record := range t.records {
		records[i] = append(make(Record, 0, len(record)), record...)
	}
	return records
}

// RowCount returns the number of records without materializing them, so a
// caller asking only "is this empty?" does not pay for a copy of the table.
func (t *Table) RowCount() int {
	return len(t.records)
}

// Rows iterates the table's rows in order, as a range-over-func sequence:
//
//	for i, row := range table.Rows {
//	    name := row.At(1)
//	}
//
// The RecordView borrows the Table's storage, so iterating costs no allocation
// however many rows there are — and it exposes no way to write through it, so
// the borrow cannot corrupt the table. Call RecordView.Record if a caller needs
// a copy it may keep or modify.
func (t *Table) Rows(yield func(int, RecordView) bool) {
	for i, record := range t.records {
		if !yield(i, newRecordView(record)) {
			return
		}
	}
}

// Row returns a read-only view of row i and whether it exists.
func (t *Table) Row(i int) (RecordView, bool) {
	if i < 0 || i >= len(t.records) {
		return RecordView{}, false
	}
	return newRecordView(t.records[i]), true
}

// ValueAt returns the display string at (row, column), or the empty string when
// either index is outside the table. It is the cheapest read there is: no view,
// no copy, no allocation.
func (t *Table) ValueAt(row, column int) string {
	if row < 0 || row >= len(t.records) {
		return ""
	}
	record := t.records[row]
	if column < 0 || column >= len(record) {
		return ""
	}
	return record[column]
}

// Equal compare Table.
func (t *Table) Equal(t2 *Table) bool {
	if t.Name() != t2.Name() {
		return false
	}
	if !t.header.Equal(t2.header) {
		return false
	}
	if t.RowCount() != t2.RowCount() {
		return false
	}
	for i, record := range t.Rows {
		other, ok := t2.Row(i)
		if !ok || !record.Record().Equal(other.Record()) {
			return false
		}
	}
	return true
}

// Valid check the contents of a Table.
func (t *Table) Valid() error {
	if t.IsEmptyName() {
		return domain.ErrEmptyTableName
	}

	if t.IsEmptyHeader() {
		return domain.ErrEmptyHeader
	}

	if t.IsEmptyRecords() {
		return domain.ErrEmptyRecords
	}

	if t.IsSameHeaderColumnName() {
		return domain.ErrSameHeaderColumns
	}

	return nil
}

// IsEmptyName return wherther table name is empty or not
func (t *Table) IsEmptyName() bool {
	return t.name == ""
}

// IsEmptyHeader return wherther table header is empty or not
func (t *Table) IsEmptyHeader() bool {
	return len(t.header) == 0
}

// IsEmptyRecords return wherther table records is empty or not
func (t *Table) IsEmptyRecords() bool {
	return len(t.records) == 0
}

// IsSameHeaderColumnName return whether the table has a header column with the same name
func (t *Table) IsSameHeaderColumnName() bool {
	encountered := make(map[string]bool, len(t.header))
	for _, name := range t.Columns {
		if encountered[name] {
			return true
		}
		encountered[name] = true
	}
	return false
}

// Print print all record with header
func (t *Table) Print(out io.Writer, mode PrintMode) error {
	switch mode {
	case PrintModeTable:
		return t.printTable(out)
	case PrintModeMarkdownTable:
		return t.printMarkdownTable(out)
	case PrintModeCSV:
		return t.printCSV(out)
	case PrintModeTSV:
		return t.printTSV(out)
	case PrintModeLTSV:
		return t.printLTSV(out)
	case PrintModeExcel:
		return t.printExcel(out)
	case PrintModeJSON:
		return t.printJSON(out)
	case PrintModeJSONL:
		return t.printNDJSON(out)
	case PrintModeParquet:
		// Export-only: on screen, render like CSV. The Parquet file is written
		// by the export path (.dump / --output), not here.
		return t.printCSV(out)
	case PrintModeVertical:
		return t.printVertical(out)
	default:
		return t.printTable(out)
	}
}

// printTable print all record with header; output format is table
func (t *Table) printTable(out io.Writer) error {
	// Create alignment configuration - detect numeric columns and align them right
	alignment := make(tw.Alignment, t.ColumnCount())
	for i, h := range t.Columns {
		// Check if header suggests numeric data or if we should align right
		headerName := strings.ToLower(h)
		// Check for common numeric column patterns
		isNumeric := strings.Contains(headerName, "gross") ||
			strings.Contains(headerName, "number") ||
			strings.Contains(headerName, "average") ||
			strings.Contains(headerName, "total") ||
			strings.Contains(headerName, "count") ||
			strings.Contains(headerName, "price") ||
			strings.Contains(headerName, "amount") ||
			headerName == "id" ||
			strings.Contains(headerName, "age") ||
			strings.Contains(headerName, "年齢") ||
			// Check if all data looks numeric (simple heuristic)
			(t.RowCount() > 0 && isAllNumeric(t.columnData(i)))

		if isNumeric {
			alignment[i] = tw.AlignRight
		} else {
			alignment[i] = tw.AlignLeft
		}
	}

	// Create header alignment configuration - center all headers
	headerAlignment := make(tw.Alignment, t.ColumnCount())
	for i := range t.ColumnCount() {
		headerAlignment[i] = tw.AlignCenter
	}

	table := tablewriter.NewTable(out,
		tablewriter.WithSymbols(tw.NewSymbols(tw.StyleASCII)),
		tablewriter.WithHeaderAutoFormat(tw.State(-1)),
		tablewriter.WithAlignment(alignment),
		tablewriter.WithHeaderAlignmentConfig(tw.CellAlignment{Global: tw.AlignCenter}),
	)

	// Convert Header ([]string) to []any for the new API
	headers := make([]any, t.ColumnCount())
	for i, h := range t.Columns {
		headers[i] = h
	}
	table.Header(headers...)

	for _, v := range t.Rows {
		// Convert the row to []any for the tablewriter API.
		row := make([]any, v.Len())
		for i := range v.Len() {
			row[i] = v.At(i)
		}
		if err := table.Append(row); err != nil {
			return fmt.Errorf("failed to append table row: %w", err)
		}
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("failed to render table: %w", err)
	}
	return nil
}

// markdownCell renders a cell for a Markdown table. A "|" is escaped so it does
// not start a new column, and an embedded newline is replaced with "<br>" so a
// multi-line value stays on one physical row instead of breaking the table. Ref
func markdownCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", "<br>")
}

// printMarkdownTable print all record with header; output format is markdown
func (t *Table) printMarkdownTable(out io.Writer) error {
	// Print header row
	if _, err := fmt.Fprint(out, "|"); err != nil {
		return fmt.Errorf("failed to write markdown header prefix: %w", err)
	}
	for _, h := range t.Columns {
		if _, err := fmt.Fprintf(out, " %s |", markdownCell(h)); err != nil {
			return fmt.Errorf("failed to write markdown header cell %q: %w", h, err)
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return fmt.Errorf("failed to write markdown header newline: %w", err)
	}

	// Print separator row
	if _, err := fmt.Fprint(out, "|"); err != nil {
		return fmt.Errorf("failed to write markdown separator prefix: %w", err)
	}
	for range t.ColumnCount() {
		if _, err := fmt.Fprint(out, "-----|"); err != nil {
			return fmt.Errorf("failed to write markdown separator cell: %w", err)
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return fmt.Errorf("failed to write markdown separator newline: %w", err)
	}

	// Print data rows
	for rowIdx, record := range t.Rows {
		if _, err := fmt.Fprint(out, "|"); err != nil {
			return fmt.Errorf("failed to write markdown row %d prefix: %w", rowIdx, err)
		}
		for i := range record.Len() {
			if _, err := fmt.Fprintf(out, " %s |", markdownCell(record.At(i))); err != nil {
				return fmt.Errorf("failed to write markdown cell: %w", err)
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return fmt.Errorf("failed to write markdown row %d newline: %w", rowIdx, err)
		}
	}
	return nil
}

// printCSV print all record with header; output format is csv. It uses a CSV
// writer so values that contain commas, quotes, or newlines are quoted and
// escaped, matching the --output file path and staying valid when redirected to
// a file or piped to a CSV-aware tool.
func (t *Table) printCSV(out io.Writer) error {
	return t.writeDelimited(out, ',')
}

// printTSV print all record with header; output format is tsv. Like printCSV it
// uses a writer that quotes values containing the delimiter, quotes, or newlines,
// so the stream stays a valid tabular record when redirected or piped.
func (t *Table) printTSV(out io.Writer) error {
	return t.writeDelimited(out, '\t')
}

// writeDelimited writes the header and records as delimiter-separated values
// using encoding/csv, so the stdout path matches the file-export path exactly.
func (t *Table) writeDelimited(out io.Writer, comma rune) error {
	w := csv.NewWriter(out)
	w.Comma = comma
	if err := w.Write([]string(t.Header())); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	// One buffer, refilled per row: encoding/csv needs a []string, and the view
	// will not surrender its own, so this is the allocation-free bridge.
	buf := make([]string, 0, t.ColumnCount())
	for _, v := range t.Rows {
		buf = v.AppendTo(buf[:0])
		if err := w.Write(buf); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}

// printLTSV print all record with header; output format is ltsv. LTSV has no
// escaping mechanism: a tab separates fields and a newline ends a record, so a
// value containing either cannot be represented losslessly. Reject such a value
// up front instead of emitting output that no longer round-trips as LTSV. Ref
// ,.
func (t *Table) printLTSV(out io.Writer) error {
	if err := EnsureLTSVHeaderWritable(t.Header()); err != nil {
		return err
	}
	for _, v := range t.Rows {
		r := make(Record, 0, v.Len())
		for i := range v.Len() {
			label, data := t.ColumnName(i), v.At(i)
			if err := ensureLTSVValueRepresentable(label, data); err != nil {
				return err
			}
			r = append(r, label+":"+data)
		}
		if _, err := fmt.Fprintln(out, strings.Join(r, "\t")); err != nil {
			return fmt.Errorf("failed to write LTSV record %v: %w", r, err)
		}
	}
	return nil
}

// verticalRecordRuleWidth is the total width of a record's separator line, so
// the rules line up whatever the record number is.
const verticalRecordRuleWidth = 60

// printVertical prints one column per line, in a block per record.
//
// Every other mode lays a record out across the line, which stops working at the
// width sqly exists for: a 300-column row is one 2700-character line in table,
// csv, tsv, and ltsv alike, and no terminal shows it. Turning the row on its side
// costs vertical space, which a terminal scrolls, instead of horizontal space,
// which it does not.
//
// The layout follows psql's expanded output: a numbered record rule, then the
// column names left-aligned in a gutter as wide as the longest one, so scanning
// down the names reads as a list. The gutter is measured in terminal cells, not
// runes or bytes: a full-width character such as 名 occupies two cells, so a
// Japanese header counted by runes left the ASCII names beside it out of line.
//
// A value is written as it is, including a newline. Vertical output is for
// reading, not for parsing, and the modes that have to stay machine-readable —
// csv, tsv, ltsv, json — are unaffected.
func (t *Table) printVertical(out io.Writer) error {
	gutter := 0
	for _, h := range t.Columns {
		if n := runewidth.StringWidth(h); n > gutter {
			gutter = n
		}
	}

	for i, record := range t.Rows {
		rule := fmt.Sprintf("-[ RECORD %d ]", i+1)
		if pad := verticalRecordRuleWidth - runewidth.StringWidth(rule); pad > 0 {
			rule += strings.Repeat("-", pad)
		}
		if _, err := fmt.Fprintln(out, rule); err != nil {
			return fmt.Errorf("failed to write vertical record rule: %w", err)
		}

		for j, header := range t.Columns {
			value := record.At(j)
			name := header + strings.Repeat(" ", gutter-runewidth.StringWidth(header))
			if _, err := fmt.Fprintf(out, "%s | %s\n", name, value); err != nil {
				return fmt.Errorf("failed to write vertical column %s: %w", header, err)
			}
		}
	}
	return nil
}

// isValidLTSVLabel reports whether label matches the LTSV label grammar
// [0-9A-Za-z_.-]+ (https://ltsv.org). A label outside this set — empty, or
// containing ':', a space, a tab, or any other character — cannot be written as a
// distinct "label:value" field that re-imports to the same column, so the LTSV
// writers reject it.
func isValidLTSVLabel(label string) bool {
	if label == "" {
		return false
	}
	for _, r := range label {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '_', r == '.', r == '-':
		default:
			return false
		}
	}
	return true
}

// EnsureLTSVHeaderWritable validates a header for LTSV output: every column name
// must be a valid LTSV label, and the names must be unique. LTSV encodes each
// column as a "label:value" field with no escaping, so an invalid label (e.g.
// "foo:bar") is ambiguous on re-import and a duplicate label silently keeps only
// the last value. Rejecting both up front keeps LTSV output round-trippable.
func EnsureLTSVHeaderWritable(header Header) error {
	seen := make(map[string]struct{}, len(header))
	for _, label := range header {
		if !isValidLTSVLabel(label) {
			return fmt.Errorf("ltsv: column name %q is not a valid LTSV label (allowed: letters, digits, '_', '.', '-')", label)
		}
		if _, ok := seen[label]; ok {
			return fmt.Errorf("ltsv: duplicate column name %q; LTSV labels must be unique or earlier values are lost on re-import", label)
		}
		seen[label] = struct{}{}
	}
	return nil
}

// ensureLTSVValueRepresentable reports an error when a value contains a byte LTSV
// cannot represent (tab or newline), so the caller rejects it before writing
// output that cannot be re-imported as LTSV.
func ensureLTSVValueRepresentable(label, value string) error {
	if strings.ContainsAny(value, "\t\n\r") {
		return fmt.Errorf("ltsv: value for column %q contains a tab or newline, which LTSV cannot represent; use csv/tsv/json for such values", label)
	}
	return nil
}

// printExcel print all record in excel format.
// This is the same as printCSV.
func (t *Table) printExcel(out io.Writer) error {
	return t.printCSV(out)
}

// rowToJSONObject builds a JSON object for one record, preserving the header
// column order. Each value is taken from the row's native cell when the Table
// came from a query, so an INTEGER or REAL column is a JSON number and a NULL is
// JSON null; a table built from strings emits every value as a JSON string, so a
// TEXT "123", "true", or "00123" is never reinterpreted as a number or boolean.
// Why a manual builder: encoding's map marshaling sorts keys alphabetically,
// which would drop column order.
func (t *Table) rowToJSONObject(row int, record RecordView) ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	// t.Columns, not t.Header(): this runs once per row, and Header() copies.
	for i, h := range t.Columns {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(h)
		if err != nil {
			return nil, fmt.Errorf("failed to encode column name %q: %w", h, err)
		}
		b.Write(key)
		b.WriteByte(':')

		var val any
		if cell, ok := t.cell(row, i); ok {
			val = cell.Value()
		} else if i < record.Len() {
			val = record.At(i)
		}
		value, err := jsonScalarToken(val)
		if err != nil {
			return nil, fmt.Errorf("failed to encode value for column %q: %w", h, err)
		}
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// jsonScalarToken serializes a value using its original Go/database type.
// In particular, a database string containing "123" or "true" remains a JSON
// string; only a database numeric or boolean value becomes a JSON scalar. A nil
// is SQL NULL and becomes JSON null, which is what distinguishes it from the
// empty string.
func jsonScalarToken(value any) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	if raw, ok := value.([]byte); ok {
		// SQLite hands back both TEXT and BLOB as bytes, and JSON has no way to
		// hold bytes at all. Text passes through as text; anything that is not
		// valid UTF-8 is binary, and turning it into a string would replace every
		// invalid byte with U+FFFD — output that looks fine and cannot be decoded
		// back. Base64 keeps it, at the cost of being base64.
		if utf8.Valid(raw) {
			return json.Marshal(string(raw))
		}
		return json.Marshal(base64.StdEncoding.EncodeToString(raw))
	}
	if token, ok := jsonNonFiniteToken(value); ok {
		return token, nil
	}
	return json.Marshal(value)
}

// jsonNonFiniteToken renders the three floats JSON cannot express. Without this,
// a Parquet column holding an infinity fails the whole output with an encoder
// error after the opening bracket is already on stdout. The strings are the ones
// PostgreSQL's row_to_json produces, so a consumer that already handles one
// database's JSON handles sqly's.
func jsonNonFiniteToken(value any) ([]byte, bool) {
	var f float64
	switch v := value.(type) {
	case float64:
		f = v
	case float32:
		f = float64(v)
	default:
		return nil, false
	}
	switch {
	case math.IsNaN(f):
		return []byte(`"NaN"`), true
	case math.IsInf(f, 1):
		return []byte(`"Infinity"`), true
	case math.IsInf(f, -1):
		return []byte(`"-Infinity"`), true
	default:
		return nil, false
	}
}

// duplicateColumnName returns the first column name that appears more than once
// in the header, or "" when all names are unique. JSON objects with duplicate
// keys are ambiguous for downstream parsers, so the JSON/NDJSON writers reject
// such a result instead of emitting it.
func (t *Table) duplicateColumnName() string {
	seen := make(map[string]struct{}, len(t.header))
	for _, h := range t.header {
		if _, ok := seen[h]; ok {
			return h
		}
		seen[h] = struct{}{}
	}
	return ""
}

// printJSON prints all records as a JSON array of objects. An empty result set
// prints "[]" so consumers always receive valid JSON.
func (t *Table) printJSON(out io.Writer) error {
	if dup := t.duplicateColumnName(); dup != "" {
		return fmt.Errorf("json output requires unique column names, but %q appears more than once; alias the duplicate columns", dup)
	}
	if t.RowCount() == 0 {
		_, err := fmt.Fprintln(out, "[]")
		return err
	}
	if _, err := fmt.Fprintln(out, "["); err != nil {
		return err
	}
	for i, record := range t.Rows {
		obj, err := t.rowToJSONObject(i, record)
		if err != nil {
			return err
		}
		sep := ""
		if i < t.RowCount()-1 {
			sep = ","
		}
		if _, err := fmt.Fprintf(out, "  %s%s\n", obj, sep); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "]")
	return err
}

// printNDJSON prints one JSON object per line (newline-delimited JSON). An empty
// result set prints nothing — the empty NDJSON stream.
func (t *Table) printNDJSON(out io.Writer) error {
	if dup := t.duplicateColumnName(); dup != "" {
		return fmt.Errorf("jsonl output requires unique column names, but %q appears more than once; alias the duplicate columns", dup)
	}
	for i, record := range t.Rows {
		obj, err := t.rowToJSONObject(i, record)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s\n", obj); err != nil {
			return err
		}
	}
	return nil
}
