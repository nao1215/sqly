package model

import (
	"encoding/json"
	"fmt"
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
	// PrintModeCSV print data in csv format
	PrintModeCSV
	// PrintModeJSON print data in json format
	PrintModeJSON
)

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

// Print print all record with header
func (t *Table) Print(out *os.File, mode PrintMode) {
	switch mode {
	case PrintModeTable:
		t.printTable(out)
	case PrintModeCSV:
		t.printCSV(out)
	case PrintModeJSON:
		t.printJSON(out)
	default:
		t.printTable(out)
	}
}

// Print print all record with header; output format is table
func (t *Table) printTable(out *os.File) {
	table := tablewriter.NewWriter(out)
	table.SetHeader(t.Header)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)

	for _, v := range t.Records {
		table.Append(v)
	}
	table.Render()
}

// Print print all record with header; output format is csv
func (t *Table) printCSV(out *os.File) {
	fmt.Fprintln(out, strings.Join(t.Header, ","))
	for _, v := range t.Records {
		fmt.Fprintln(out, strings.Join(v, ","))
	}
}

// Print print all record in json format
func (t *Table) printJSON(out *os.File) {
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
	}
	fmt.Fprintln(out, string(b))
}
