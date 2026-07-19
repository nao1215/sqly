package model

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
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

// IsNumericValue reports whether s is what a human would call a number. It
// strips comma thousands separators ("1,000") and surrounding whitespace, then
// requires a finite decimal. It rejects the Go-specific float spellings
// ParseFloat also accepts but data rarely means as numbers: hexadecimal floats
// ("0x1p4"), underscore digit separators ("1_000"), and the Infinity/NaN words.
//
// This is the single numeric contract shared by data profiling and table-mode
// right alignment, so the same cell is classified the same way in both. Why
// expose it from the model layer: presentation and profiling both depend on the
// model, so a model-level predicate keeps the two surfaces in agreement without
// duplicating the rules.
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
// blank cells. It uses IsNumericValue so column alignment follows the same
// numeric contract as data profiling.
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
	// nulls optionally marks which cells are SQL NULL (as opposed to an empty
	// string), indexed as nulls[row][col]. It is set only for query results,
	// where the distinction is known, and is consulted by JSON/NDJSON output so a
	// NULL is emitted as JSON null. nil means "no NULL information"; text formats
	// ignore it and render every cell as a string.
	nulls [][]bool
	// jsonTyped opts JSON/NDJSON output into the typed contract: a cell that is a
	// canonical JSON number is emitted as a native number, "true"/"false" as a
	// native boolean, and everything else as a JSON string. It is false by
	// default so the legacy string contract stays the default; only the explicit
	// opt-in (--json-typed/--ndjson-typed or .mode json-typed/ndjson-typed) turns
	// it on. Other output formats ignore it.
	jsonTyped bool
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

// SetNulls records which cells are SQL NULL, indexed as nulls[row][col]. It is
// used by query results so JSON/NDJSON output can emit a NULL as JSON null
// rather than an empty string. Other output formats ignore it.
func (t *Table) SetNulls(nulls [][]bool) {
	t.nulls = nulls
}

// SetJSONTyped opts JSON and NDJSON output into the typed contract, where a cell
// that is a canonical JSON number is emitted as a native number (large integers
// verbatim, so no precision loss or scientific notation), "true"/"false" as a
// native boolean, a SQL NULL as null, and every other value as a JSON string.
// It has no effect on non-JSON output formats. The default (false) preserves the
// legacy contract that emits every non-NULL value as a string.
func (t *Table) SetJSONTyped(typed bool) {
	t.jsonTyped = typed
}

// isNull reports whether the cell at (row, col) is a known SQL NULL.
func (t *Table) isNull(row, col int) bool {
	return row < len(t.nulls) && col < len(t.nulls[row]) && t.nulls[row][col]
}

// IsNull reports whether the cell at (row, col) is a known SQL NULL, as opposed
// to an empty string. It returns false when no NULL information was recorded
// (the table did not come from a query). It lets callers such as the profile
// workflow distinguish a NULL from a blank value.
func (t *Table) IsNull(row, col int) bool {
	return t.isNull(row, col)
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
	case PrintModeNDJSON:
		return t.printNDJSON(out)
	case PrintModeParquet:
		// Export-only: on screen, render like CSV. The Parquet file is written
		// by the export path (.dump / --output), not here.
		return t.printCSV(out)
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
	for _, h := range t.Header() {
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
	for range t.Header() {
		if _, err := fmt.Fprint(out, "-----|"); err != nil {
			return fmt.Errorf("failed to write markdown separator cell: %w", err)
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return fmt.Errorf("failed to write markdown separator newline: %w", err)
	}

	// Print data rows
	for rowIdx, record := range t.Records() {
		if _, err := fmt.Fprint(out, "|"); err != nil {
			return fmt.Errorf("failed to write markdown row %d prefix: %w", rowIdx, err)
		}
		for _, cell := range record {
			if _, err := fmt.Fprintf(out, " %s |", markdownCell(cell)); err != nil {
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
	for _, v := range t.Records() {
		if err := w.Write([]string(v)); err != nil {
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
	for _, v := range t.Records() {
		r := make(Record, 0, len(v))
		for i, data := range v {
			if err := ensureLTSVValueRepresentable(t.Header()[i], data); err != nil {
				return err
			}
			r = append(r, t.Header()[i]+":"+data)
		}
		if _, err := fmt.Fprintln(out, strings.Join(r, "\t")); err != nil {
			return fmt.Errorf("failed to write LTSV record %v: %w", r, err)
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
// column order. Why string values: the table model stores every cell as a
// string, so emitting strings keeps output lossless (e.g. "007" stays "007")
// and consistent with the other text formats. Why a manual builder: encoding's
// map marshaling sorts keys alphabetically, which would drop column order.
func (t *Table) rowToJSONObject(row int, record Record) ([]byte, error) {
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

		// Emit a SQL NULL as JSON null so it is distinguishable from an empty
		// string in machine-readable output.
		if t.isNull(row, i) {
			b.WriteString("null")
			continue
		}

		var val string
		if i < len(record) {
			val = record[i]
		}
		var value []byte
		if t.jsonTyped {
			value, err = jsonScalarToken(val)
		} else {
			value, err = json.Marshal(val)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to encode value for column %q: %w", h, err)
		}
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// JSON boolean literals recognized by the typed output contract.
const (
	jsonLiteralTrue  = "true"
	jsonLiteralFalse = "false"
)

// jsonScalarToken returns the JSON token for a cell value in typed output mode.
// A value that is a canonical JSON number is emitted verbatim, so a large integer
// stays lossless and never regresses into scientific notation; the JSON literals
// "true" and "false" become native booleans; everything else is emitted as a JSON
// string. A SQL NULL is handled by the caller before this is reached.
func jsonScalarToken(s string) ([]byte, error) {
	if s == jsonLiteralTrue || s == jsonLiteralFalse {
		return []byte(s), nil
	}
	if isCanonicalJSONNumber(s) {
		return []byte(s), nil
	}
	return json.Marshal(s)
}

// isCanonicalJSONNumber reports whether s is a number in the exact JSON grammar
// (RFC 8259): an optional leading minus, an integer part with no redundant
// leading zero, an optional fraction, and an optional exponent. Emitting only
// such strings verbatim as JSON numbers keeps the output valid while preserving
// the original digits, so "007" stays a string and a 30-digit integer is not
// rounded. Values like "+1", "1.", ".5", "1e", "NaN", or surrounding spaces are
// rejected and fall back to a JSON string.
func isCanonicalJSONNumber(s string) bool {
	if s == "" {
		return false
	}
	i, n := 0, len(s)
	if s[i] == '-' {
		i++
		if i == n {
			return false
		}
	}
	// Integer part: a single "0", or a non-zero digit followed by more digits.
	switch {
	case s[i] == '0':
		i++
	case s[i] >= '1' && s[i] <= '9':
		i++
		for i < n && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	default:
		return false
	}
	// Optional fraction.
	if i < n && s[i] == '.' {
		i++
		if i >= n || s[i] < '0' || s[i] > '9' {
			return false
		}
		for i < n && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	// Optional exponent.
	if i < n && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < n && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i >= n || s[i] < '0' || s[i] > '9' {
			return false
		}
		for i < n && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	return i == n
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
	if len(t.Records()) == 0 {
		_, err := fmt.Fprintln(out, "[]")
		return err
	}
	if _, err := fmt.Fprintln(out, "["); err != nil {
		return err
	}
	for i, record := range t.Records() {
		obj, err := t.rowToJSONObject(i, record)
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
	if dup := t.duplicateColumnName(); dup != "" {
		return fmt.Errorf("ndjson output requires unique column names, but %q appears more than once; alias the duplicate columns", dup)
	}
	for i, record := range t.Records() {
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
