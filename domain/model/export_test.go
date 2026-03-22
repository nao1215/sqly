package model

import "testing"

func TestExportFormat_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ef   ExportFormat
		want string
	}{
		{name: "csv", ef: ExportCSV, want: "csv"},
		{name: "tsv", ef: ExportTSV, want: "tsv"},
		{name: "ltsv", ef: ExportLTSV, want: "ltsv"},
		{name: "markdown", ef: ExportMarkdown, want: "markdown"},
		{name: "excel", ef: ExportExcel, want: "excel"},
		{name: "unknown defaults to csv", ef: ExportFormat(99), want: "csv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.ef.String(); got != tt.want {
				t.Errorf("ExportFormat.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExportFormat_Extension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ef   ExportFormat
		want string
	}{
		{name: "csv", ef: ExportCSV, want: ".csv"},
		{name: "tsv", ef: ExportTSV, want: ".tsv"},
		{name: "ltsv", ef: ExportLTSV, want: ".ltsv"},
		{name: "markdown", ef: ExportMarkdown, want: ".md"},
		{name: "excel", ef: ExportExcel, want: ".xlsx"},
		{name: "unknown defaults to .csv", ef: ExportFormat(99), want: ".csv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.ef.Extension(); got != tt.want {
				t.Errorf("ExportFormat.Extension() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExportFormatFromPrintMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode PrintMode
		want ExportFormat
	}{
		{name: "table falls back to csv", mode: PrintModeTable, want: ExportCSV},
		{name: "csv", mode: PrintModeCSV, want: ExportCSV},
		{name: "tsv", mode: PrintModeTSV, want: ExportTSV},
		{name: "ltsv", mode: PrintModeLTSV, want: ExportLTSV},
		{name: "markdown", mode: PrintModeMarkdownTable, want: ExportMarkdown},
		{name: "excel", mode: PrintModeExcel, want: ExportExcel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ExportFormatFromPrintMode(tt.mode); got != tt.want {
				t.Errorf("ExportFormatFromPrintMode(%v) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}
