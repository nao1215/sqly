package shell

import (
	"path/filepath"
	"strings"
)

// isCSV checks if the file is CSV.
func isCSV(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".csv"
}

// isTSV checks if the file is TSV.
func isTSV(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".tsv"
}

// isLTSV checks if the file is LTSV.
func isLTSV(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".ltsv"
}

// isXLAM checks if the file is XLAM.
// XLAM: Excel Add-in
func isXLAM(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".xlam"
}

// isXLSM checks if the file is XLSM.
// XLSM: Excel Macro-Enabled Workbook
func isXLSM(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".xlsm"
}

// isXLSX checks if the file is XLSX.
// XLSX: Excel Workbook
func isXLSX(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".xlsx"
}

// isXLTM checks if the file is XLTM.
// XLTM: Excel Macro-Enabled Template
func isXLTM(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".xltm"
}

// isXLTX checks if the file is XLTX.
// XLTX: Excel Template
func isXLTX(filePath string) bool {
	return getFileTypeFromPath(filePath) == ".xltx"
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

// getFileTypeFromPath returns the file type by removing compression extensions first.
// This handles compressed files like sample.xlsx.gz correctly by returning ".xlsx".
func getFileTypeFromPath(filePath string) string {
	name := filepath.Base(filePath)

	// Handle compressed files by removing compression extensions first
	compressedExtensions := []string{".gz", ".bz2", ".xz", ".zst"}
	for {
		found := false
		for _, compExt := range compressedExtensions {
			if strings.HasSuffix(name, compExt) {
				name = strings.TrimSuffix(name, compExt)
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	// Return the final extension
	pos := strings.LastIndex(name, ".")
	if pos <= 0 {
		return ""
	}
	return name[pos:]
}
