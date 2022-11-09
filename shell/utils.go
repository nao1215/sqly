package shell

import "strings"

func trimWordGaps(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func isJSON(path string) bool {
	return ext(path) == ".json"
}

func isCSV(path string) bool {
	return ext(path) == ".csv"
}

func ext(path string) string {
	pos := strings.LastIndex(path, ".")
	return path[pos:]
}
