// Package model defines Data Transfer Object (Entity, Value Object)
package model

import "strings"

// Label is LTSV label.
type Label []string

// LTSV is Labeled Tab-separated Values data with label.
type LTSV struct {
	Name    string
	Label   Label
	Records []Record
}

// IsLabelEmpty return wherther label is empty or not
func (l *LTSV) IsLabelEmpty() bool {
	return len(l.Label) == 0
}

// SetLabel set label column.
func (l *LTSV) SetLabel(label Label) {
	l.Label = append(l.Label, label...)
}

// SetRecord set tsv record.
func (l *LTSV) SetRecord(record Record) {
	l.Records = append(l.Records, record)
}

// ToTable convert TSV to Table.
func (l *LTSV) ToTable() *Table {
	return &Table{
		Name:    strings.TrimSuffix(l.Name, ".ltsv"),
		Header:  Header(l.Label),
		Records: l.Records,
	}
}
