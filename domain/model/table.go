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
)

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
	}
	return "unknown"
}

// Table is DB table.
type Table struct {
	Name    string
	Header  Header
	Records []Record
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
	return t.Name == ""
}

// IsEmptyHeader return wherther table header is empty or not
func (t *Table) IsEmptyHeader() bool {
	return len(t.Header) == 0
}

// IsEmptyRecords return wherther table records is empty or not
func (t *Table) IsEmptyRecords() bool {
	return len(t.Records) == 0
}

// IsSameHeaderColumnName return whether the table has a header column with the same name
func (t *Table) IsSameHeaderColumnName() bool {
	encountered := map[string]bool{}
	for i := 0; i < len(t.Header); i++ {
		if !encountered[t.Header[i]] {
			encountered[t.Header[i]] = true
			continue
		} else {
			return true
		}
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
	default:
		t.printTable(out)
	}
}

// printTables print all record with header; output format is table
func (t *Table) printTable(out io.Writer) {
	table := tablewriter.NewWriter(out)
	table.SetHeader(t.Header)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)

	for _, v := range t.Records {
		table.Append(v)
	}
	table.Render()
}

// printMarkdownTable print all record with header; output format is markdown
func (t *Table) printMarkdownTable(out io.Writer) {
	table := tablewriter.NewWriter(out)
	table.SetHeader(t.Header)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")

	for _, v := range t.Records {
		table.Append(v)
	}
	table.Render()
}

// printCSV print all record with header; output format is csv
func (t *Table) printCSV(out io.Writer) {
	fmt.Fprintln(out, strings.Join(t.Header, ","))
	for _, v := range t.Records {
		fmt.Fprintln(out, strings.Join(v, ","))
	}
}

// printTSV print all record with header; output format is tsv
func (t *Table) printTSV(out io.Writer) {
	fmt.Fprintln(out, strings.Join(t.Header, "\t"))
	for _, v := range t.Records {
		fmt.Fprintln(out, strings.Join(v, "\t"))
	}
}

// Print print all record with header; output format is ltsv
func (t *Table) printLTSV(out io.Writer) {
	for _, v := range t.Records {
		r := Record{}
		for i, data := range v {
			r = append(r, t.Header[i]+":"+data)
		}
		fmt.Fprintln(out, strings.Join(r, "\t"))
	}
}

// printJSON print all record in json format
func (t *Table) printJSON(out io.Writer) {
	data := make([]map[string]interface{}, 0)

	for _, v := range t.Records {
		d := make(map[string]interface{}, 0)
		for i, r := range v {
			d[t.Header[i]] = r
		}
		data = append(data, d)
	}
	b, err := json.MarshalIndent(data, "", "   ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: "+err.Error())
		return
	}
	fmt.Fprintln(out, string(b))
}
