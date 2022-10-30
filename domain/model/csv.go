// Package model defines Data Transfer Object (Entity, Value Object)
package model

import "strings"

// CSV is csv data with header.
type CSV struct {
	Name    string
	Header  Header
	Records []Record
}

// Header is CSV header.
type Header []string

// Record is CSV records.
type Record []string

// IsHeaderEmpty return wherther header is empty or not
func (c *CSV) IsHeaderEmpty() bool {
	return len(c.Header) == 0
}

// SetHeader set header column.
func (c *CSV) SetHeader(header Header) {
	c.Header = append(c.Header, header...)
}

// SetRecord set csv record.
func (c *CSV) SetRecord(record Record) {
	c.Records = append(c.Records, record)
}

// ToTable convert CSV to Table.
func (c *CSV) ToTable() *Table {
	return &Table{
		Name:    strings.TrimSuffix(c.Name, ".csv"),
		Header:  c.Header,
		Records: c.Records,
	}
}
