package model

import (
	"errors"
	"strings"
	"testing"
	"testing/quick"
)

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
		{name: "json", ef: ExportJSON, want: "json"},
		{name: "ndjson", ef: ExportNDJSON, want: "ndjson"},
		{name: "parquet", ef: ExportParquet, want: "parquet"},
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
		{name: "json", ef: ExportJSON, want: ".json"},
		{name: "ndjson", ef: ExportNDJSON, want: ".ndjson"},
		{name: "parquet", ef: ExportParquet, want: ".parquet"},
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

func TestExportFormatFromExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ext    string
		want   ExportFormat
		wantOK bool
	}{
		{name: ".csv maps to csv", ext: ".csv", want: ExportCSV, wantOK: true},
		{name: ".tsv maps to tsv", ext: ".tsv", want: ExportTSV, wantOK: true},
		{name: ".ltsv maps to ltsv", ext: ".ltsv", want: ExportLTSV, wantOK: true},
		{name: ".md maps to markdown", ext: ".md", want: ExportMarkdown, wantOK: true},
		{name: ".xlsx maps to excel", ext: ".xlsx", want: ExportExcel, wantOK: true},
		{name: ".json maps to json", ext: ".json", want: ExportJSON, wantOK: true},
		{name: ".ndjson maps to ndjson", ext: ".ndjson", want: ExportNDJSON, wantOK: true},
		{name: ".jsonl maps to ndjson", ext: ".jsonl", want: ExportNDJSON, wantOK: true},
		{name: ".parquet maps to parquet", ext: ".parquet", want: ExportParquet, wantOK: true},
		{name: "uppercase .CSV maps to csv", ext: ".CSV", want: ExportCSV, wantOK: true},
		{name: "unknown extension is not recognized", ext: ".txt", want: ExportCSV, wantOK: false},
		{name: "empty extension is not recognized", ext: "", want: ExportCSV, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExportFormatFromExtension(tt.ext)
			if ok != tt.wantOK {
				t.Fatalf("ExportFormatFromExtension(%q) ok = %v, want %v", tt.ext, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("ExportFormatFromExtension(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestCompressionFromExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ext    string
		want   Compression
		wantOK bool
	}{
		{name: ".gz maps to gzip", ext: ".gz", want: CompressionGzip, wantOK: true},
		{name: ".bz2 maps to bzip2", ext: ".bz2", want: CompressionBzip2, wantOK: true},
		{name: ".xz maps to xz", ext: ".xz", want: CompressionXz, wantOK: true},
		{name: ".zst maps to zstd", ext: ".zst", want: CompressionZstd, wantOK: true},
		{name: ".z maps to zlib", ext: ".z", want: CompressionZlib, wantOK: true},
		{name: ".snappy maps to snappy", ext: ".snappy", want: CompressionSnappy, wantOK: true},
		{name: ".s2 maps to s2", ext: ".s2", want: CompressionS2, wantOK: true},
		{name: ".lz4 maps to lz4", ext: ".lz4", want: CompressionLz4, wantOK: true},
		{name: "uppercase .GZ maps to gzip", ext: ".GZ", want: CompressionGzip, wantOK: true},
		{name: ".csv is not a compression", ext: ".csv", want: CompressionNone, wantOK: false},
		{name: "empty is not a compression", ext: "", want: CompressionNone, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := CompressionFromExtension(tt.ext)
			if ok != tt.wantOK {
				t.Fatalf("CompressionFromExtension(%q) ok = %v, want %v", tt.ext, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("CompressionFromExtension(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestExportFormat_SupportsCompression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ef   ExportFormat
		want bool
	}{
		{name: "csv supports compression", ef: ExportCSV, want: true},
		{name: "tsv supports compression", ef: ExportTSV, want: true},
		{name: "ltsv supports compression", ef: ExportLTSV, want: true},
		{name: "json supports compression", ef: ExportJSON, want: true},
		{name: "ndjson supports compression", ef: ExportNDJSON, want: true},
		{name: "markdown supports compression", ef: ExportMarkdown, want: true},
		{name: "parquet does not support compression", ef: ExportParquet, want: false},
		{name: "excel does not support compression", ef: ExportExcel, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.ef.SupportsCompression(); got != tt.want {
				t.Errorf("%v.SupportsCompression() = %v, want %v", tt.ef, got, tt.want)
			}
		})
	}
}

func TestResolveOutputTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		explicit    ExportFormat
		explicitSet bool
		wantFormat  ExportFormat
		wantComp    Compression
		wantErr     error
	}{
		{
			name:       "no flag infers parquet from path",
			path:       "result.parquet",
			wantFormat: ExportParquet,
			wantComp:   CompressionNone,
		},
		{
			name:       "no flag infers csv with gzip from path",
			path:       "result.csv.gz",
			wantFormat: ExportCSV,
			wantComp:   CompressionGzip,
		},
		{
			name:       "no flag infers ndjson with zstd from path",
			path:       "out.ndjson.zst",
			wantFormat: ExportNDJSON,
			wantComp:   CompressionZstd,
		},
		{
			name:        "explicit json matching .json path",
			path:        "result.json",
			explicit:    ExportJSON,
			explicitSet: true,
			wantFormat:  ExportJSON,
			wantComp:    CompressionNone,
		},
		{
			name:        "explicit json conflicts with .csv path",
			path:        "result.csv",
			explicit:    ExportJSON,
			explicitSet: true,
			wantErr:     ErrOutputFormatConflict,
		},
		{
			name:       "no extension and no flag defaults to csv",
			path:       "result",
			wantFormat: ExportCSV,
			wantComp:   CompressionNone,
		},
		{
			name:        "explicit format with no extension is kept",
			path:        "result",
			explicit:    ExportNDJSON,
			explicitSet: true,
			wantFormat:  ExportNDJSON,
			wantComp:    CompressionNone,
		},
		{
			name:       "unknown extension with no flag falls back to csv",
			path:       "result.txt",
			wantFormat: ExportCSV,
			wantComp:   CompressionNone,
		},
		{
			name:        "unknown extension with an explicit format is kept",
			path:        "result.txt",
			explicit:    ExportCSV,
			explicitSet: true,
			wantFormat:  ExportCSV,
			wantComp:    CompressionNone,
		},
		{
			name:    "compression on parquet is rejected",
			path:    "result.parquet.gz",
			wantErr: ErrCompressionUnsupported,
		},
		{
			name:    "bzip2 output is rejected",
			path:    "result.csv.bz2",
			wantErr: ErrCompressionUnsupported,
		},
		{
			name:        "explicit csv with gzip path is consistent",
			path:        "result.csv.gz",
			explicit:    ExportCSV,
			explicitSet: true,
			wantFormat:  ExportCSV,
			wantComp:    CompressionGzip,
		},
		{
			name:    "nested csv compression suffixes are rejected",
			path:    "double.csv.gz.zst",
			wantErr: ErrNestedCompression,
		},
		{
			name:    "nested parquet compression suffixes are rejected",
			path:    "fake.parquet.gz.zst",
			wantErr: ErrNestedCompression,
		},
		{
			name:    "nested xlsx compression suffixes are rejected",
			path:    "fake.xlsx.gz.zst",
			wantErr: ErrNestedCompression,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotFmt, gotComp, err := ResolveOutputTarget(tt.path, tt.explicit, tt.explicitSet)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ResolveOutputTarget(%q) error = %v, want %v", tt.path, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveOutputTarget(%q) unexpected error: %v", tt.path, err)
			}
			if gotFmt != tt.wantFormat {
				t.Errorf("ResolveOutputTarget(%q) format = %v, want %v", tt.path, gotFmt, tt.wantFormat)
			}
			if gotComp != tt.wantComp {
				t.Errorf("ResolveOutputTarget(%q) compression = %v, want %v", tt.path, gotComp, tt.wantComp)
			}
		})
	}
}

func TestBuildOutputPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		format ExportFormat
		comp   Compression
		want   string
	}{
		{name: "keeps matching parquet extension", path: "result.parquet", format: ExportParquet, comp: CompressionNone, want: "result.parquet"},
		{name: "adds csv extension when missing", path: "result", format: ExportCSV, comp: CompressionNone, want: "result.csv"},
		{name: "keeps csv.gz path", path: "result.csv.gz", format: ExportCSV, comp: CompressionGzip, want: "result.csv.gz"},
		{name: "rebuilds base and appends gzip", path: "result.gz", format: ExportCSV, comp: CompressionGzip, want: "result.csv.gz"},
		{name: "replaces mismatched extension", path: "data.json", format: ExportNDJSON, comp: CompressionNone, want: "data.ndjson"},
		{name: "keeps ndjson.zst path", path: "out.ndjson.zst", format: ExportNDJSON, comp: CompressionZstd, want: "out.ndjson.zst"},
		{name: "honors an unknown extension", path: "out.txt", format: ExportCSV, comp: CompressionNone, want: "out.txt"},
		{name: "honors an unknown extension with compression", path: "out.txt.gz", format: ExportCSV, comp: CompressionGzip, want: "out.txt.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := BuildOutputPath(tt.path, tt.format, tt.comp); got != tt.want {
				t.Errorf("BuildOutputPath(%q, %v, %v) = %q, want %q", tt.path, tt.format, tt.comp, got, tt.want)
			}
		})
	}
}

// TestBuildOutputPath_Property asserts the built path always ends with the
// format extension followed by the compression extension, and that building is
// idempotent.
func TestBuildOutputPath_Property(t *testing.T) {
	formats := []ExportFormat{
		ExportCSV, ExportTSV, ExportLTSV, ExportMarkdown,
		ExportExcel, ExportJSON, ExportNDJSON, ExportParquet,
	}
	comps := []Compression{
		CompressionNone, CompressionGzip, CompressionXz, CompressionZstd,
		CompressionZlib, CompressionSnappy, CompressionS2, CompressionLz4,
	}
	property := func(base string, fIdx, cIdx uint8) bool {
		ef := formats[int(fIdx)%len(formats)]
		comp := comps[int(cIdx)%len(comps)]
		// Binary formats never carry a compression wrapper; mirror the caller contract.
		if !ef.SupportsCompression() {
			comp = CompressionNone
		}
		// Use an extensionless path so the format extension is appended. Unknown
		// extensions are instead honored as-is, which the table tests cover.
		path := strings.ReplaceAll(base, ".", "_")
		got := BuildOutputPath(path, ef, comp)
		if !strings.HasSuffix(got, ef.Extension()+comp.Extension()) {
			return false
		}
		return BuildOutputPath(got, ef, comp) == got
	}
	if err := quick.Check(property, quickConfig()); err != nil {
		t.Error(err)
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
		{name: "json", mode: PrintModeJSON, want: ExportJSON},
		{name: "ndjson", mode: PrintModeNDJSON, want: ExportNDJSON},
		{name: "parquet", mode: PrintModeParquet, want: ExportParquet},
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

// TestIsInputOnlyExtension covers rejecting ACH/Fedwire export destinations,
// including compressed variants.
func TestIsInputOnlyExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"out.ach", true},
		{"out.fed", true},
		{"out.ACH", true},
		{"out.ach.gz", true},
		{"out.fed.zst", true},
		// Multiple stacked compression suffixes must still be seen through.
		{"out.ach.gz.zst", true},
		{"out.fed.gz.zst", true},
		{"out.ach.gz.gz.gz", true},
		{"out.csv", false},
		{"out.csv.gz", false},
		{"out.csv.gz.zst", false},
		{"out.parquet", false},
		{"out", false},
	}
	for _, tt := range tests {
		if got := IsInputOnlyExtension(tt.path); got != tt.want {
			t.Errorf("IsInputOnlyExtension(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
