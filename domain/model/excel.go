package model

// Excel is XLAM / XLSM / XLSX / XLTM / XLTX data with header.
type Excel struct {
	// name is sheet name.
	name string
	// header is excel header.
	header Header
	// records is excel record.
	records []Record
}

// NewExcel create new Excel.
func NewExcel(
	name string,
	header Header,
	records []Record,
) *Excel {
	return &Excel{
		name:    name,
		header:  header,
		records: records,
	}
}

// ToTable convert Excel to Table.
func (e *Excel) ToTable() *Table {
	return NewTable(
		e.name,
		e.header,
		e.records,
	)
}

// Equal compare Excel.
func (e *Excel) Equal(e2 *Excel) bool {
	if e.name != e2.name {
		return false
	}
	if !e.header.Equal(e2.header) {
		return false
	}
	if len(e.records) != len(e2.records) {
		return false
	}
	for i, record := range e.records {
		if !record.Equal(e2.records[i]) {
			return false
		}
	}
	return true
}
