package model

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrOutputFormatConflict is returned when an explicit output mode and the
// destination path extension name different formats.
var ErrOutputFormatConflict = errors.New("output format conflicts with destination path")

// ErrCompressionUnsupported is returned when a requested compression cannot be
// written for the chosen format (binary formats, or bzip2 which has no writer).
var ErrCompressionUnsupported = errors.New("compression is not supported for this output")

// ErrNestedCompression is returned when a destination path stacks more than one
// trailing compression suffix (e.g. ".csv.gz.zst"). sqly applies only the
// outermost codec, so accepting such a path would leave the inner suffix lying
// about the bytes on disk.
var ErrNestedCompression = errors.New("nested compression suffixes are not supported")

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

// SupportsCompression reports whether output of this format can be wrapped in a
// compression codec. Binary container formats (Parquet, Excel) carry their own
// encoding and are not wrapped, so they return false.
func (e ExportFormat) SupportsCompression() bool {
	switch e {
	case ExportParquet, ExportExcel:
		return false
	default:
		return true
	}
}

// ExportFormatFromExtension maps a base file extension (e.g. ".csv") to an
// ExportFormat. The bool is false when the extension is not a known export
// format, so callers can fall back instead of guessing. Matching is
// case-insensitive. ".jsonl" maps to NDJSON since JSON Lines is newline-delimited.
func ExportFormatFromExtension(ext string) (ExportFormat, bool) {
	switch strings.ToLower(ext) {
	case ExtCSV:
		return ExportCSV, true
	case ExtTSV:
		return ExportTSV, true
	case ExtLTSV:
		return ExportLTSV, true
	case ExtMarkdown:
		return ExportMarkdown, true
	case ExtExcel:
		return ExportExcel, true
	case ExtJSON:
		return ExportJSON, true
	case ExtNDJSON, ExtJSONL:
		return ExportNDJSON, true
	case ExtParquet:
		return ExportParquet, true
	default:
		return ExportCSV, false
	}
}

// Compression represents an output compression codec wrapped around a text or
// JSON export. It is kept separate from filesql's compression type so the
// domain layer stays free of infrastructure dependencies; the filesql adapter
// maps these values when creating the writer.
type Compression uint

const (
	// CompressionNone writes the format uncompressed.
	CompressionNone Compression = iota
	// CompressionGzip writes gzip (.gz).
	CompressionGzip
	// CompressionBzip2 names bzip2 (.bz2). It is read-only; writing is rejected.
	CompressionBzip2
	// CompressionXz writes xz (.xz).
	CompressionXz
	// CompressionZstd writes zstd (.zst).
	CompressionZstd
	// CompressionZlib writes zlib (.z).
	CompressionZlib
	// CompressionSnappy writes snappy (.snappy).
	CompressionSnappy
	// CompressionS2 writes s2 (.s2).
	CompressionS2
	// CompressionLz4 writes lz4 (.lz4).
	CompressionLz4
)

// Compression file extension constants.
const (
	ExtGzip   = ".gz"
	ExtBzip2  = ".bz2"
	ExtXz     = ".xz"
	ExtZstd   = ".zst"
	ExtZlib   = ".z"
	ExtSnappy = ".snappy"
	ExtS2     = ".s2"
	ExtLz4    = ".lz4"
)

// Extension returns the file extension for the compression, or "" for none.
func (c Compression) Extension() string {
	switch c {
	case CompressionGzip:
		return ExtGzip
	case CompressionBzip2:
		return ExtBzip2
	case CompressionXz:
		return ExtXz
	case CompressionZstd:
		return ExtZstd
	case CompressionZlib:
		return ExtZlib
	case CompressionSnappy:
		return ExtSnappy
	case CompressionS2:
		return ExtS2
	case CompressionLz4:
		return ExtLz4
	default:
		return ""
	}
}

// CompressionFromExtension maps a compression extension (e.g. ".gz") to a
// Compression. The bool is false when the extension is not a known compression.
// Matching is case-insensitive.
func CompressionFromExtension(ext string) (Compression, bool) {
	switch strings.ToLower(ext) {
	case ExtGzip:
		return CompressionGzip, true
	case ExtBzip2:
		return CompressionBzip2, true
	case ExtXz:
		return CompressionXz, true
	case ExtZstd:
		return CompressionZstd, true
	case ExtZlib:
		return CompressionZlib, true
	case ExtSnappy:
		return CompressionSnappy, true
	case ExtS2:
		return CompressionS2, true
	case ExtLz4:
		return CompressionLz4, true
	default:
		return CompressionNone, false
	}
}

// ResolveOutputTarget determines the export format and compression for a
// destination path. explicit is the format chosen by a mode flag or .mode, and
// explicitSet is false when the user gave no format (table/default), in which
// case the format is inferred from the path so requests like "result.parquet"
// or "out.ndjson.gz" do the obvious thing.
//
// When an explicit format and the path extension disagree, it returns
// ErrOutputFormatConflict rather than silently writing a surprising format.
// Compression on a binary format, or bzip2 (no writer), returns
// ErrCompressionUnsupported.
func ResolveOutputTarget(path string, explicit ExportFormat, explicitSet bool) (ExportFormat, Compression, error) {
	// Reject stacked compression suffixes up front. Only the outermost codec is
	// applied, so a path like "out.csv.gz.zst" or "fake.parquet.gz.zst" would write
	// a single-codec file under a name that advertises a different (or binary)
	// format. Fail instead of producing a mislabeled artifact.
	if hasNestedCompressionSuffix(path) {
		return 0, CompressionNone, fmt.Errorf("%w: %q stacks multiple codecs; use a single compression suffix", ErrNestedCompression, path)
	}

	comp := CompressionNone
	base := path
	if c, ok := CompressionFromExtension(filepath.Ext(path)); ok {
		comp = c
		base = strings.TrimSuffix(path, filepath.Ext(path))
	}

	baseExt := filepath.Ext(base)
	inferred, hasInferred := ExportFormatFromExtension(baseExt)

	var format ExportFormat
	switch {
	case explicitSet && hasInferred && explicit != inferred:
		return 0, CompressionNone, fmt.Errorf("%w: output mode %q does not match destination extension %q",
			ErrOutputFormatConflict, explicit.String(), baseExt)
	case explicitSet:
		format = explicit
	case hasInferred:
		format = inferred
	default:
		// No extension, or an unknown one: fall back to CSV. The path itself is
		// preserved by BuildOutputPath (an unknown extension is not rewritten),
		// so the documented CSV fallback writes to the exact destination given.
		format = ExportCSV
	}

	if comp != CompressionNone {
		if !format.SupportsCompression() {
			return 0, CompressionNone, fmt.Errorf("%w: %s output cannot be compressed",
				ErrCompressionUnsupported, format.String())
		}
		if comp == CompressionBzip2 {
			return 0, CompressionNone, fmt.Errorf("%w: bzip2 output cannot be written", ErrCompressionUnsupported)
		}
	}
	return format, comp, nil
}

// BuildOutputPath returns the destination path with the format's extension and
// any compression extension applied, so the written file name matches what was
// actually produced (e.g. "result" with NDJSON+gzip becomes "result.ndjson.gz").
func BuildOutputPath(path string, format ExportFormat, comp Compression) string {
	base := path
	if _, ok := CompressionFromExtension(filepath.Ext(path)); ok {
		base = strings.TrimSuffix(path, filepath.Ext(path))
	}
	// Append the format extension only when the path has none. Rewrite an
	// existing extension only when it is a known export extension that differs
	// from the chosen format; leave an unknown extension untouched so an
	// explicitly chosen destination path is honored rather than silently changed.
	baseExt := filepath.Ext(base)
	_, knownExt := ExportFormatFromExtension(baseExt)
	switch {
	case baseExt == "":
		base += format.Extension()
	case knownExt && !strings.EqualFold(baseExt, format.Extension()):
		base = strings.TrimSuffix(base, baseExt) + format.Extension()
	}
	return base + comp.Extension()
}

// IsInputOnlyExtension reports whether a destination path targets a format the
// single-table export path cannot produce: ACH (.ach) and Fedwire (.fed). These
// are reconstructed from a complete, related table set, not from one table, so
// `--output` and `.dump` (which write a single table) reject them; the whole-set
// `--save`/`--save-dir`/`.save` write-back path handles them instead. All
// trailing compression suffixes are stripped first, so a path that hides the
// extension behind several codecs (".ach.gz.zst", ".fed.gz.zst") is detected too.
func IsInputOnlyExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(stripCompressionSuffixes(path))) {
	case ".ach", ".fed":
		return true
	default:
		return false
	}
}

// Native financial write-back format names returned by FinancialWriteFormat.
const (
	// FinancialFormatACH identifies a NACHA ACH (.ach) destination.
	FinancialFormatACH = "ach"
	// FinancialFormatFedWire identifies a Fedwire (.fed) destination.
	FinancialFormatFedWire = "fed"
)

// FinancialWriteFormat classifies a write-back destination path as the native
// financial format sqly can reconstruct from a complete table set: ACH (.ach) or
// Fedwire (.fed). It returns FinancialFormatACH, FinancialFormatFedWire, or "".
// A compressed path (for example "payment.ach.gz") returns "" because the native
// ACH/Fedwire writers emit a plain, uncompressed file; sqly does not round-trip a
// compressed financial source.
func FinancialWriteFormat(path string) string {
	if _, ok := CompressionFromExtension(filepath.Ext(path)); ok {
		return ""
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ach":
		return FinancialFormatACH
	case ".fed":
		return FinancialFormatFedWire
	default:
		return ""
	}
}

// compressionSuffixAliases maps a long-form compression extension to its canonical
// short form. sqly writes only the short forms, and filesql reads only those, so an
// alias is never a writable codec. It is recognized only when detecting
// stacked suffixes or seeing through compression to the underlying format, so a
// destination like "out.parquet.gzip.zst" cannot smuggle a masked format past
// validation by spelling one codec out in full.
var compressionSuffixAliases = map[string]string{
	".gzip":  ExtGzip,
	".zstd":  ExtZstd,
	".bzip2": ExtBzip2,
}

// isCompressionSuffix reports whether ext is a compression suffix sqly recognizes
// for detection, including the long-form aliases (".gzip", ".zstd", ".bzip2"). It
// is broader than CompressionFromExtension, which admits only writable codecs.
func isCompressionSuffix(ext string) bool {
	if _, ok := CompressionFromExtension(ext); ok {
		return true
	}
	_, ok := compressionSuffixAliases[strings.ToLower(ext)]
	return ok
}

// hasNestedCompressionSuffix reports whether path ends in two or more stacked
// compression suffixes (e.g. "out.csv.gz.zst" or "fake.parquet.gzip.zst"). A
// long-form alias such as ".gzip" counts, so spelling a codec out in full cannot
// slip a stacked destination past validation.
func hasNestedCompressionSuffix(path string) bool {
	if !isCompressionSuffix(filepath.Ext(path)) {
		return false
	}
	inner := strings.TrimSuffix(path, filepath.Ext(path))
	return isCompressionSuffix(filepath.Ext(inner))
}

// stripCompressionSuffixes removes every trailing compression extension from a
// path (e.g. "out.ach.gzip.zst" -> "out.ach"), so a check on the remaining base
// extension is not fooled by stacked codecs or a long-form alias.
func stripCompressionSuffixes(path string) string {
	for {
		if !isCompressionSuffix(filepath.Ext(path)) {
			return path
		}
		path = strings.TrimSuffix(path, filepath.Ext(path))
	}
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
