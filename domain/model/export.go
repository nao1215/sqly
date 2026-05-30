package model

// ExportFormat represents a file export format, separate from display modes.
// This allows adding new export targets (e.g. Parquet, compressed formats)
// without modifying the terminal display mode enum.
type ExportFormat uint

const (
	// ExportCSV exports data as CSV
	ExportCSV ExportFormat = iota
	// ExportTSV exports data as TSV
	ExportTSV
	// ExportLTSV exports data as LTSV
	ExportLTSV
	// ExportMarkdown exports data as Markdown table
	ExportMarkdown
	// ExportExcel exports data as XLSX
	ExportExcel
	// ExportJSON exports data as a JSON array of objects
	ExportJSON
	// ExportNDJSON exports data as newline-delimited JSON
	ExportNDJSON
	// ExportParquet exports data as Apache Parquet
	ExportParquet
)

// String returns the string representation of the ExportFormat.
func (e ExportFormat) String() string {
	switch e {
	case ExportCSV:
		return formatCSV
	case ExportTSV:
		return formatTSV
	case ExportLTSV:
		return formatLTSV
	case ExportMarkdown:
		return formatMarkdown
	case ExportExcel:
		return formatExcel
	case ExportJSON:
		return formatJSON
	case ExportNDJSON:
		return formatNDJSON
	case ExportParquet:
		return formatParquet
	}
	return formatCSV
}

// Extension returns the file extension for the ExportFormat.
func (e ExportFormat) Extension() string {
	switch e {
	case ExportCSV:
		return ExtCSV
	case ExportTSV:
		return ExtTSV
	case ExportLTSV:
		return ExtLTSV
	case ExportMarkdown:
		return ExtMarkdown
	case ExportExcel:
		return ExtExcel
	case ExportJSON:
		return ExtJSON
	case ExportNDJSON:
		return ExtNDJSON
	case ExportParquet:
		return ExtParquet
	}
	return ExtCSV
}

// ExportFormatFromPrintMode converts a PrintMode to an ExportFormat.
// PrintModeTable falls back to ExportCSV since table format is display-only.
func ExportFormatFromPrintMode(m PrintMode) ExportFormat {
	switch m {
	case PrintModeCSV:
		return ExportCSV
	case PrintModeTSV:
		return ExportTSV
	case PrintModeLTSV:
		return ExportLTSV
	case PrintModeMarkdownTable:
		return ExportMarkdown
	case PrintModeExcel:
		return ExportExcel
	case PrintModeJSON:
		return ExportJSON
	case PrintModeNDJSON:
		return ExportNDJSON
	case PrintModeParquet:
		return ExportParquet
	default:
		return ExportCSV
	}
}
