// Package model defines Data Transfer Object (Entity, Value Object)
package model

import "strings"

// TSV is tsv data with header.
type TSV struct {
	Name    string
	Header  Header
	Records []Record
}

// IsHeaderEmpty return wherther header is empty or not
func (t *TSV) IsHeaderEmpty() bool {
	return len(t.Header) == 0
}

// SetHeader set header column.
func (t *TSV) SetHeader(header Header) {
	t.Header = append(t.Header, header...)
}

// SetRecord set tsv record.
func (t *TSV) SetRecord(record Record) {
	t.Records = append(t.Records, record)
}

// ToTable convert TSV to Table.
func (t *TSV) ToTable() *Table {
	return &Table{
		Name:    strings.TrimSuffix(t.Name, ".tsv"),
		Header:  t.Header,
		Records: t.Records,
	}
}
