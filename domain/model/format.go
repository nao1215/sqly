package model

import "strings"

// This file is the one place a format name is written down.
//
// A name like "csv" used to appear in four unrelated tables: the parser behind
// --output-format, the parser behind .mode, PrintMode.String, and the shell's
// completion list. Adding a format meant finding all four, and the compiler had
// nothing to say when one was missed — the format simply did not exist for
// whichever question read the table that lacked it. --output-format accepted a
// name that .mode rejected, or completion offered one that neither took.
//
// So the name, the mode, and everything derived from them live in the registries
// below, and each question is a function over one of them.

// Format name constants shared between PrintMode and ExportFormat.
const (
	formatTable    = "table"
	formatCSV      = "csv"
	formatTSV      = "tsv"
	formatLTSV     = "ltsv"
	formatMarkdown = "markdown"
	formatExcel    = "excel"
	formatJSON     = "json"
	formatJSONL    = "jsonl"
	formatParquet  = "parquet"
	formatVertical = "vertical"
)

// Extension name constants.
const (
	ExtCSV      = ".csv"
	ExtTSV      = ".tsv"
	ExtLTSV     = ".ltsv"
	ExtMarkdown = ".md"
	ExtExcel    = ".xlsx"
	ExtJSON     = ".json"
	ExtJSONL    = ".jsonl"
	ExtNDJSON   = ".ndjson"
	ExtParquet  = ".parquet"
)

// PrintMode is enum to specify output method
type PrintMode uint

const (
	// PrintModeTable print data in table format
	PrintModeTable PrintMode = iota
	// PrintModeMarkdownTable print data in markdown table format
	PrintModeMarkdownTable
	// PrintModeCSV print data in csv format
	PrintModeCSV
	// PrintModeTSV print data in tsv format
	PrintModeTSV
	// PrintModeLTSV print data in ltsv format
	PrintModeLTSV
	// PrintModeExcel print data in excel format
	PrintModeExcel
	// PrintModeJSON print data as a JSON array of objects
	PrintModeJSON
	// PrintModeJSONL print data as newline-delimited JSON (one object per line)
	PrintModeJSONL
	// PrintModeParquet is an export-only mode; on screen it renders like CSV and
	// only writes a Parquet file via .dump or --output (same pattern as Excel).
	PrintModeParquet
	// PrintModeVertical prints one column per line, in a block per record. It is
	// display-only, like PrintModeTable: a row wider than the terminal is what the
	// table, csv, tsv, and ltsv modes all fail at, and a 300-column row is the case
	// sqly exists for.
	PrintModeVertical
)

// printModes pairs each mode with the name a user types for it, in the order
// --help, .mode's error, and completion all list them: the formats a person
// reads first, then the two that only make sense written to a file.
//
// The order is part of the registry rather than each caller's business, so the
// three lists a user might compare cannot disagree about it either.
//
// selectable says whether .mode can choose the format. Excel and Parquet cannot:
// neither can be rendered to a terminal, so selecting one used to leave the
// session printing CSV while calling itself something else — a banner had to
// admit it was "same as csv mode", and a script scraping that output depended on
// excel meaning csv forever. They stay in the registry because --output-format
// still names them: there the name picks a file format for --output, which is a
// different question from what the screen shows.
var printModes = []struct {
	mode       PrintMode
	name       string
	selectable bool
}{
	{PrintModeTable, formatTable, true},
	{PrintModeVertical, formatVertical, true},
	{PrintModeCSV, formatCSV, true},
	{PrintModeTSV, formatTSV, true},
	{PrintModeLTSV, formatLTSV, true},
	{PrintModeJSON, formatJSON, true},
	{PrintModeJSONL, formatJSONL, true},
	{PrintModeMarkdownTable, formatMarkdown, true},
	{PrintModeExcel, formatExcel, false},
	{PrintModeParquet, formatParquet, false},
}

// PrintModes returns every output mode, in the order they are listed to a user.
// A caller building a list of the formats — help text, completion — iterates
// this rather than writing the names out, so a format cannot be offered by one
// and missing from another.
func PrintModes() []PrintMode {
	modes := make([]PrintMode, 0, len(printModes))
	for _, m := range printModes {
		modes = append(modes, m.mode)
	}
	return modes
}

// unknownPrintModeName is what String reports for a value that is not one of the
// declared modes. Nothing sqly does can produce one — the modes come from the
// two parsers, and both reject a name they do not know — so it marks a
// programming error rather than a user's, and the tests use it to catch a mode
// declared without a registry entry.
const unknownPrintModeName = "unknown"

// String return string of PrintMode.
func (p PrintMode) String() string {
	for _, m := range printModes {
		if m.mode == p {
			return m.name
		}
	}
	return unknownPrintModeName
}

// ParsePrintMode returns the mode a user named, and whether the name is one.
// Surrounding whitespace and letter case are ignored, so "--output-format CSV "
// and ".mode csv" reach the same mode.
func ParsePrintMode(name string) (PrintMode, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, m := range printModes {
		if m.name == normalized {
			return m.mode, true
		}
	}
	return PrintModeTable, false
}

// PrintModeNames lists every output format's name, comma-separated in the order
// a user is shown them. The separator is ", " rather than "|" so a terminal can
// wrap the list; ten names joined by pipes is one unbreakable 58-column token.
func PrintModeNames() string {
	names := make([]string, 0, len(printModes))
	for _, m := range printModes {
		names = append(names, m.name)
	}
	return strings.Join(names, ", ")
}

// SelectableModes returns the formats .mode can choose, in the order they are
// listed to a user. Completion iterates this so it cannot offer a name .mode
// would then reject.
func SelectableModes() []PrintMode {
	modes := make([]PrintMode, 0, len(printModes))
	for _, m := range printModes {
		if m.selectable {
			modes = append(modes, m.mode)
		}
	}
	return modes
}

// ParseSelectableMode returns the mode a user named to .mode, and whether the
// name is one .mode can select. A format that only names a file (excel,
// parquet) is not: it reports false, like an unknown name, so the caller says
// what .mode takes rather than accepting a mode the screen cannot show.
func ParseSelectableMode(name string) (PrintMode, bool) {
	mode, ok := ParsePrintMode(name)
	if !ok {
		return PrintModeTable, false
	}
	for _, m := range printModes {
		if m.mode == mode {
			return mode, m.selectable
		}
	}
	return PrintModeTable, false
}

// SelectableModeNames lists the formats .mode can choose, comma-separated in the
// order a user is shown them.
func SelectableModeNames() string {
	names := make([]string, 0, len(printModes))
	for _, m := range printModes {
		if m.selectable {
			names = append(names, m.name)
		}
	}
	return strings.Join(names, ", ")
}

// IsDisplayOnly reports whether the mode only decides what the screen looks like,
// so it names no export format.
//
// The export path asks this to decide whether the mode chose the destination's
// format or the destination's extension did: `.dump out.tsv` while the session is
// in a display-only mode writes a TSV, where the same call in csv mode is a
// conflict the caller has to resolve. Table and vertical are the two — vertical is
// a way of reading a wide row, not a file format anything else can parse back.
func (p PrintMode) IsDisplayOnly() bool {
	return p == PrintModeTable || p == PrintModeVertical
}

// AllowsMultipleResults reports whether this format can carry more than one
// result set in a single stream.
//
// A format a person reads can: two tables, two vertical blocks, or two Markdown
// tables separated by a blank line are still exactly what they look like. A
// format a program parses cannot. Two CSV bodies concatenated are one CSV whose
// third line is a second header row; two JSON arrays back to back are not a JSON
// document; and JSONL has no way to say "a new result starts here". Emitting
// those anyway produces a file that parses — into the wrong thing, or not at
// all — which is worse than refusing, so a run that would need one is rejected.
func (p PrintMode) AllowsMultipleResults() bool {
	switch p {
	case PrintModeTable, PrintModeVertical, PrintModeMarkdownTable:
		return true
	default:
		return false
	}
}

// stdinFormats pairs each --stdin-format name with the extension the staged file
// gets, in the order help lists them.
//
// A piped dataset has no file name, so nothing can be inferred from it: the
// format has to be named, and the name has to become an extension, because the
// loader works by path. The two used to be separate tables in separate packages
// — the set the flag validated against, and the map that turned a name into an
// extension — with a comment on one asking the reader to keep it in step with
// the other. A name in the first and not the second parsed fine and then staged
// a file with no extension, which failed to import for a reason that named
// neither the flag nor the format.
var stdinFormats = []struct {
	name string
	ext  string
}{
	{formatCSV, ExtCSV},
	{formatTSV, ExtTSV},
	{formatLTSV, ExtLTSV},
	{formatJSON, ExtJSON},
	{formatJSONL, ExtJSONL},
}

// StdinFormatExtension returns the file extension a dataset piped as name is
// staged with, and whether name is a format stdin can be read as.
func StdinFormatExtension(name string) (string, bool) {
	for _, f := range stdinFormats {
		if f.name == name {
			return f.ext, true
		}
	}
	return "", false
}

// StdinFormatNames lists the formats stdin can be read as, comma-separated in
// the order a user is shown them.
func StdinFormatNames() string {
	names := make([]string, 0, len(stdinFormats))
	for _, f := range stdinFormats {
		names = append(names, f.name)
	}
	return strings.Join(names, ", ")
}
