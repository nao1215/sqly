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
)

// String returns the string representation of the ExportFormat.
func (e ExportFormat) String() string {
	switch e {
	case ExportCSV:
		return "csv"
	case ExportTSV:
		return "tsv"
	case ExportLTSV:
		return "ltsv"
	case ExportMarkdown:
		return "markdown"
	case ExportExcel:
		return "excel"
	}
	return "csv"
}

// Extension returns the file extension for the ExportFormat.
func (e ExportFormat) Extension() string {
	switch e {
	case ExportCSV:
		return ".csv"
	case ExportTSV:
		return ".tsv"
	case ExportLTSV:
		return ".ltsv"
	case ExportMarkdown:
		return ".md"
	case ExportExcel:
		return ".xlsx"
	}
	return ".csv"
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
	default:
		return ExportCSV
	}
}
