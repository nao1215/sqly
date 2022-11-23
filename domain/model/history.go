package model

import (
	"strconv"
)

// Histories is sqly history all record.
type Histories []*History

// History is sqly history record.
type History struct {
	// ID is history id. 1 is oldest
	ID int
	// Request is sqly history record that is user input from sqly prompt
	Request string
}

// ToTable convert History to Table.
func (h Histories) ToTable() *Table {
	var records []Record
	for _, v := range h {
		records = append(records, Record{
			strconv.Itoa(v.ID), v.Request,
		})
	}

	return &Table{
		Name:    "history",
		Header:  []string{"id", "request"},
		Records: records,
	}
}

// ToStringList convert history to string list.
func (h Histories) ToStringList() []string {
	var histories []string
	for _, v := range h {
		histories = append(histories, v.Request)
	}
	return histories
}
