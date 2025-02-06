// Package model defines Data Transfer Object (Entity, Value Object)
package model

import "strings"

// Label is LTSV label.
type Label []string

// LTSV is Labeled Tab-separated Values data with label.
type LTSV struct {
	name    string
	label   Label
	records []Record
}

// NewLTSV create new LTSV.
func NewLTSV(name string, label Label, records []Record) *LTSV {
	return &LTSV{
		name:    name,
		label:   label,
		records: records,
	}
}

// ToTable convert TSV to Table.
func (l *LTSV) ToTable() *Table {
	return NewTable(
		strings.TrimSuffix(l.name, ".ltsv"),
		Header(l.label),
		l.records,
	)
}
