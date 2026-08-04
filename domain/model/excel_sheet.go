package model

// ExcelSheet is one sheet of a workbook and whether the workbook shows it.
//
// A workbook can hide a sheet, and a hidden one usually holds the
// spreadsheet's own working-out rather than data anyone meant to publish. sqly
// imports only the sheets a workbook shows unless asked otherwise, so the
// answer to "what else was in that file?" has to come from somewhere: this is
// what --inspect reports and what the import notice counts.
//
// Excel separates "hidden", which a reader can undo from the sheet tabs, from
// "very hidden", which only the VBA editor can. The library sqly reads
// workbooks with reports one boolean covering both, so this type carries one
// too, and nothing in sqly claims to tell the two apart.
type ExcelSheet struct {
	// Name is the sheet name as the workbook stores it, before it is sanitized
	// into a table name.
	Name string
	// Visible is false for a sheet the workbook does not show.
	Visible bool
}

// HiddenExcelSheets returns the sheets a workbook does not show, in workbook
// order.
func HiddenExcelSheets(sheets []ExcelSheet) []ExcelSheet {
	var hidden []ExcelSheet
	for _, sheet := range sheets {
		if !sheet.Visible {
			hidden = append(hidden, sheet)
		}
	}
	return hidden
}
