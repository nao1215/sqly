package shell

import (
	"github.com/nao1215/gorky/path"
)

// isJSON checks if the file is JSON.
func isJSON(filePath string) bool {
	return path.Ext(filePath) == ".json"
}

// isCSV checks if the file is CSV.
func isCSV(filePath string) bool {
	return path.Ext(filePath) == ".csv"
}

// isTSV checks if the file is TSV.
func isTSV(filePath string) bool {
	return path.Ext(filePath) == ".tsv"
}

// isLTSV checks if the file is LTSV.
func isLTSV(filePath string) bool {
	return path.Ext(filePath) == ".ltsv"
}

// isXLAM checks if the file is XLAM.
// XLAM: Excel Add-in
func isXLAM(filePath string) bool {
	return path.Ext(filePath) == ".xlam"
}

// isXLSM checks if the file is XLSM.
// XLSM: Excel Macro-Enabled Workbook
func isXLSM(filePath string) bool {
	return path.Ext(filePath) == ".xlsm"
}

// isXLSX checks if the file is XLSX.
// XLSX: Excel Workbook
func isXLSX(filePath string) bool {
	return path.Ext(filePath) == ".xlsx"
}

// isXLTM checks if the file is XLTM.
// XLTM: Excel Macro-Enabled Template
func isXLTM(filePath string) bool {
	return path.Ext(filePath) == ".xltm"
}

// isXLTX checks if the file is XLTX.
// XLTX: Excel Template
func isXLTX(filePath string) bool {
	return path.Ext(filePath) == ".xltx"
}
