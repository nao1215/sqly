// Package model defines Data Transfer Object (Entity, Value Object)
package model

import "strings"

// TSV is tsv data with header.
type TSV struct {
	name    string
	header  Header
	records []Record
}

// NewTSV create new TSV.
func NewTSV(
	name string,
	header Header,
	records []Record,
) *TSV {
	return &TSV{
		name:    name,
		header:  header,
		records: records,
	}
}

// ToTable convert TSV to Table.
func (t *TSV) ToTable() *Table {
	return NewTable(
		strings.TrimSuffix(t.name, ".tsv"),
		t.header,
		t.records,
	)
}
