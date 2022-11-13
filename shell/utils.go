package shell

import (
	"github.com/nao1215/gorky/path"
)

func isJSON(filePath string) bool {
	return path.Ext(filePath) == ".json"
}

func isCSV(filePath string) bool {
	return path.Ext(filePath) == ".csv"
}

func isTSV(filePath string) bool {
	return path.Ext(filePath) == ".tsv"
}

func isLTSV(filePath string) bool {
	return path.Ext(filePath) == ".ltsv"
}
