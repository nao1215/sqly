package shell

import (
	"path/filepath"
	"strings"

	"github.com/nao1215/fileparser"
)

// isSupportedFile checks if the file has a format supported by fileparser.
// This includes CSV, TSV, LTSV, JSON, JSONL, Parquet, XLSX and all supported
// compression variants (.gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4).
func isSupportedFile(filePath string) bool {
	return fileparser.DetectFileType(filePath) != fileparser.Unsupported
}

// isExcelFile checks if the file is an Excel format (.xlsx).
func isExcelFile(filePath string) bool {
	ft := fileparser.DetectFileType(filePath)
	base := ft
	if fileparser.IsCompressed(ft) {
		base = fileparser.BaseFileType(ft)
	}
	return base == fileparser.XLSX
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
	compressedExtensions := []string{".gz", ".bz2", ".xz", ".zst", ".z", ".snappy", ".s2", ".lz4"}
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
