// Package model defines Data Transfer Object (Entity, Value Object)
package model

import "strings"

// CSV is csv data with header.
type CSV struct {
	name    string
	header  Header
	records []Record
}

// NewCSV create new CSV.
func NewCSV(
	name string,
	header Header,
	records []Record,
) *CSV {
	return &CSV{
		name:    name,
		header:  header,
		records: records,
	}
}

// ToTable convert CSV to Table.
func (c *CSV) ToTable() *Table {
	return NewTable(
		strings.TrimSuffix(c.name, ".csv"),
		c.header,
		c.records,
	)
}
