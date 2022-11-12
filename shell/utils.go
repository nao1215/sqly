package shell

import (
	"path/filepath"
	"strings"
)

func trimWordGaps(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func isJSON(path string) bool {
	return ext(path) == ".json"
}

func isCSV(path string) bool {
	return ext(path) == ".csv"
}

func isTSV(path string) bool {
	return ext(path) == ".tsv"
}

func ext(path string) string {
	base := filepath.Base(path)
	pos := strings.LastIndex(base, ".")
	if pos <= 0 {
		return ""
	}
	// hidden file
	if strings.HasPrefix(path, ".") && pos == 0 {
		return ""
	}
	return base[pos:]
}
