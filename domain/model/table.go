package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nao1215/sqly/domain"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// getColumnData extracts data from a specific column index
func getColumnData(records []Record, columnIndex int) []string {
	var columnData []string
	for _, record := range records {
		if columnIndex < len(record) {
			columnData = append(columnData, record[columnIndex])
		}
	}
	return columnData
}

// isAllNumeric checks if all values in a column look like numbers
func isAllNumeric(values []string) bool {
	if len(values) == 0 {
		return false
	}

	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		// Remove commas for number parsing (e.g., "1,000" -> "1000")
		v = strings.ReplaceAll(v, ",", "")
		// Try to parse as float
		if _, err := strconv.ParseFloat(v, 64); err != nil {
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
	formatNDJSON   = "ndjson"
	formatParquet  = "parquet"
)

// Extension name constants.
const (
	ExtCSV      = ".csv"
	ExtTSV      = ".tsv"
	ExtLTSV     = ".ltsv"
	ExtMarkdown = ".md"
	ExtExcel    = ".xlsx"
	ExtJSON     = ".json"
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
	// PrintModeNDJSON print data as newline-delimited JSON (one object per line)
	PrintModeNDJSON
	// PrintModeParquet is an export-only mode; on screen it renders like CSV and
	// only writes a Parquet file via .dump or --output (same pattern as Excel).
	PrintModeParquet
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
	case PrintModeNDJSON:
		return formatNDJSON
	case PrintModeParquet:
		return formatParquet
	}
	return "unknown"
}

// Table is DB table.
type Table struct {
	// Name is table name.
	name string
	// Header is table header.
	header Header
	// Records is table records.
	records []Record
}

// NewTable create new Table.
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

// Name return table name.
func (t *Table) Name() string {
	return t.name
}

// Header return table header.
func (t *Table) Header() Header {
	return t.header
}

// Records return table records.
func (t *Table) Records() []Record {
	return t.records
}

// Equal compare Table.
func (t *Table) Equal(t2 *Table) bool {
	if t.Name() != t2.Name() {
		return false
	}
	if !t.header.Equal(t2.header) {
		return false
	}
	if len(t.Records()) != len(t2.Records()) {
		return false
	}
	for i, record := range t.Records() {
		if !record.Equal(t2.Records()[i]) {
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
	encountered := map[string]bool{}
	for i := range t.header {
		if !encountered[t.Header()[i]] {
			encountered[t.Header()[i]] = true
			continue
		}
		return true
	}
	return false
}

// Print print all record with header
func (t *Table) Print(out io.Writer, mode PrintMode) error {
	switch mode {
	case PrintModeTable:
		return t.printTable(out)
	case PrintModeMarkdownTable:
		t.printMarkdownTable(out)
		return nil
	case PrintModeCSV:
		t.printCSV(out)
		return nil
	case PrintModeTSV:
		t.printTSV(out)
		return nil
	case PrintModeLTSV:
		t.printLTSV(out)
		return nil
	case PrintModeExcel:
		t.printExcel(out)
		return nil
	case PrintModeJSON:
		return t.printJSON(out)
	case PrintModeNDJSON:
		return t.printNDJSON(out)
	case PrintModeParquet:
		// Export-only: on screen, render like CSV. The Parquet file is written
		// by the export path (.dump / --output), not here.
		t.printCSV(out)
		return nil
	default:
		return t.printTable(out)
	}
}

// printTable print all record with header; output format is table
func (t *Table) printTable(out io.Writer) error {
	// Create alignment configuration - detect numeric columns and align them right
	alignment := make(tw.Alignment, len(t.Header()))
	for i, h := range t.Header() {
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
			(len(t.Records()) > 0 && isAllNumeric(getColumnData(t.Records(), i)))

		if isNumeric {
			alignment[i] = tw.AlignRight
		} else {
			alignment[i] = tw.AlignLeft
		}
	}

	// Create header alignment configuration - center all headers
	headerAlignment := make(tw.Alignment, len(t.Header()))
	for i := range t.Header() {
		headerAlignment[i] = tw.AlignCenter
	}

	table := tablewriter.NewTable(out,
		tablewriter.WithSymbols(tw.NewSymbols(tw.StyleASCII)),
		tablewriter.WithHeaderAutoFormat(tw.State(-1)),
		tablewriter.WithAlignment(alignment),
		tablewriter.WithHeaderAlignmentConfig(tw.CellAlignment{Global: tw.AlignCenter}),
	)

	// Convert Header ([]string) to []any for the new API
	headers := make([]any, len(t.Header()))
	for i, h := range t.Header() {
		headers[i] = h
	}
	table.Header(headers...)

	for _, v := range t.Records() {
		// Convert Record ([]string) to []any for the new API
		row := make([]any, len(v))
		for i, cell := range v {
			row[i] = cell
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

// printMarkdownTable print all record with header; output format is markdown
func (t *Table) printMarkdownTable(out io.Writer) {
	// Print header row
	fmt.Fprint(out, "|")
	for _, h := range t.Header() {
		fmt.Fprintf(out, " %s |", strings.ReplaceAll(h, "|", "\\|"))
	}
	fmt.Fprintln(out)

	// Print separator row
	fmt.Fprint(out, "|")
	for range t.Header() {
		fmt.Fprint(out, "-----|")
	}
	fmt.Fprintln(out)

	// Print data rows
	for _, record := range t.Records() {
		fmt.Fprint(out, "|")
		for _, cell := range record {
			fmt.Fprintf(out, " %s |", strings.ReplaceAll(cell, "|", "\\|"))
		}
		fmt.Fprintln(out)
	}
}

// printCSV print all record with header; output format is csv
func (t *Table) printCSV(out io.Writer) {
	fmt.Fprintln(out, strings.Join(t.Header(), ","))
	for _, v := range t.Records() {
		fmt.Fprintln(out, strings.Join(v, ","))
	}
}

// printTSV print all record with header; output format is tsv
func (t *Table) printTSV(out io.Writer) {
	fmt.Fprintln(out, strings.Join(t.Header(), "\t"))
	for _, v := range t.Records() {
		fmt.Fprintln(out, strings.Join(v, "\t"))
	}
}

// Print print all record with header; output format is ltsv
func (t *Table) printLTSV(out io.Writer) {
	for _, v := range t.Records() {
		r := make(Record, 0, len(v))
		for i, data := range v {
			r = append(r, t.Header()[i]+":"+data)
		}
		fmt.Fprintln(out, strings.Join(r, "\t"))
	}
}

// printExcel print all record in excel format.
// This is the same as printCSV.
func (t *Table) printExcel(out io.Writer) {
	t.printCSV(out)
}

// rowToJSONObject builds a JSON object for one record, preserving the header
// column order. Why string values: the table model stores every cell as a
// string, so emitting strings keeps output lossless (e.g. "007" stays "007")
// and consistent with the other text formats. Why a manual builder: encoding's
// map marshaling sorts keys alphabetically, which would drop column order.
func (t *Table) rowToJSONObject(record Record) ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, h := range t.Header() {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(h)
		if err != nil {
			return nil, fmt.Errorf("failed to encode column name %q: %w", h, err)
		}
		b.Write(key)
		b.WriteByte(':')

		var val string
		if i < len(record) {
			val = record[i]
		}
		value, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("failed to encode value for column %q: %w", h, err)
		}
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// printJSON prints all records as a JSON array of objects. An empty result set
// prints "[]" so consumers always receive valid JSON.
func (t *Table) printJSON(out io.Writer) error {
	if len(t.Records()) == 0 {
		_, err := fmt.Fprintln(out, "[]")
		return err
	}
	if _, err := fmt.Fprintln(out, "["); err != nil {
		return err
	}
	for i, record := range t.Records() {
		obj, err := t.rowToJSONObject(record)
		if err != nil {
			return err
		}
		sep := ""
		if i < len(t.Records())-1 {
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
	for _, record := range t.Records() {
		obj, err := t.rowToJSONObject(record)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s\n", obj); err != nil {
			return err
		}
	}
	return nil
}
