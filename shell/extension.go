package shell

import (
	"path/filepath"
	"strings"
)

// isCSV checks if the file is CSV.
func isCSV(filePath string) bool {
	return ext(filePath) == ".csv"
}

// isTSV checks if the file is TSV.
func isTSV(filePath string) bool {
	return ext(filePath) == ".tsv"
}

// isLTSV checks if the file is LTSV.
func isLTSV(filePath string) bool {
	return ext(filePath) == ".ltsv"
}

// isXLAM checks if the file is XLAM.
// XLAM: Excel Add-in
func isXLAM(filePath string) bool {
	return ext(filePath) == ".xlam"
}

// isXLSM checks if the file is XLSM.
// XLSM: Excel Macro-Enabled Workbook
func isXLSM(filePath string) bool {
	return ext(filePath) == ".xlsm"
}

// isXLSX checks if the file is XLSX.
// XLSX: Excel Workbook
func isXLSX(filePath string) bool {
	return ext(filePath) == ".xlsx"
}

// isXLTM checks if the file is XLTM.
// XLTM: Excel Macro-Enabled Template
func isXLTM(filePath string) bool {
	return ext(filePath) == ".xltm"
}

// isXLTX checks if the file is XLTX.
// XLTX: Excel Template
func isXLTX(filePath string) bool {
	return ext(filePath) == ".xltx"
}

// ext extracts file extension from path.
// If path does not have extension, ext return "".
func ext(path string) string {
	base := filepath.Base(path)
	pos := strings.LastIndex(base, ".")
	if pos <= 0 {
		return ""
	}
	return base[pos:]
}
