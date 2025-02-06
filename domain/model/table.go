package model

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nao1215/sqly/domain"
	"github.com/olekukonko/tablewriter"
)

// PrintMode is enum to specify output method
type PrintMode uint

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
	// PrintModeJSON print data in json format
	PrintModeJSON
	// PrintModeExcel print data in excel format
	PrintModeExcel
)

// String return string of PrintMode.
func (p PrintMode) String() string {
	switch p {
	case PrintModeTable:
		return "table"
	case PrintModeMarkdownTable:
		return "markdown"
	case PrintModeCSV:
		return "csv"
	case PrintModeTSV:
		return "tsv"
	case PrintModeLTSV:
		return "ltsv"
	case PrintModeJSON:
		return "json"
	case PrintModeExcel:
		return "excel"
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
func (t *Table) Print(out io.Writer, mode PrintMode) {
	switch mode {
	case PrintModeTable:
		t.printTable(out)
	case PrintModeMarkdownTable:
		t.printMarkdownTable(out)
	case PrintModeCSV:
		t.printCSV(out)
	case PrintModeTSV:
		t.printTSV(out)
	case PrintModeLTSV:
		t.printLTSV(out)
	case PrintModeJSON:
		t.printJSON(out)
	case PrintModeExcel:
		t.printExcel(out)
	default:
		t.printTable(out)
	}
}

// printTables print all record with header; output format is table
func (t *Table) printTable(out io.Writer) {
	table := tablewriter.NewWriter(out)
	table.SetHeader(t.Header())
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)

	for _, v := range t.Records() {
		table.Append(v)
	}
	table.Render()
}

// printMarkdownTable print all record with header; output format is markdown
func (t *Table) printMarkdownTable(out io.Writer) {
	table := tablewriter.NewWriter(out)
	table.SetHeader(t.Header())
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")

	for _, v := range t.Records() {
		table.Append(v)
	}
	table.Render()
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
		r := Record{}
		for i, data := range v {
			r = append(r, t.Header()[i]+":"+data)
		}
		fmt.Fprintln(out, strings.Join(r, "\t"))
	}
}

// printJSON print all record in json format
func (t *Table) printJSON(out io.Writer) {
	data := make([]map[string]interface{}, 0)

	for _, v := range t.Records() {
		d := make(map[string]interface{}, 0)
		for i, r := range v {
			d[t.Header()[i]] = r
		}
		data = append(data, d)
	}
	b, err := json.MarshalIndent(data, "", "   ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %s", err.Error())
		return
	}
	fmt.Fprintln(out, string(b))
}

// printExcel print all record in excel format.
// This is the same as printCSV.
func (t *Table) printExcel(out io.Writer) {
	t.printCSV(out)
}
