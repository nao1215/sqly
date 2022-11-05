package model

import (
	"os"

	"github.com/nao1215/sqly/domain"
	"github.com/olekukonko/tablewriter"
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
func (t *Table) Print(out *os.File) {
	table := tablewriter.NewWriter(out)
	table.SetHeader(t.Header)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)

	for _, v := range t.Records {
		table.Append(v)
	}
	table.Render()
}
