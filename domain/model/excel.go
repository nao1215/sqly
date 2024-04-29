package model

// Excel is XLAM / XLSM / XLSX / XLTM / XLTX data with header.
type Excel struct {
	// Name is sheet name.
	Name string
	// Header is excel header.
	Header Header
	// Records is excel record.
	Records []Record
}

// ToTable convert Excel to Table.
func (e *Excel) ToTable() *Table {
	return &Table{
		Name:    e.Name,
		Header:  e.Header,
		Records: e.Records,
	}
}
