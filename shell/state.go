package shell

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// state is shell state.
type state struct {
	cwd  string // cwd is current working directory.
	mode *mode  // mode is output mode.
	// rowMismatch is the current policy for a CSV/TSV row whose field count
	// differs from the header. It is seeded from the --row-mismatch flag and
	// changed by the .row-mismatch command.
	rowMismatch model.RowMismatchPolicy
	// importEncoding is the current text-import decoding for CSV, TSV, LTSV,
	// JSON, and JSONL inputs. It is seeded from --encoding.
	importEncoding model.TextEncoding
	// includeHiddenSheets is the session's Excel sheet policy: false imports
	// only the sheets a workbook shows, true imports the hidden ones too. It is
	// seeded from --include-hidden-sheets and does not change during a session,
	// so every .import reads workbooks the same way the initial import did.
	includeHiddenSheets bool
}

// newState return *state.
func newState(arg *config.Arg) (*state, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	importEncoding := arg.Encoding
	if importEncoding == "" {
		importEncoding = model.TextEncodingUTF8
	}
	return &state{
		cwd:                 dir,
		mode:                newMode(config.Stdout, arg.Output.Mode),
		rowMismatch:         arg.RowMismatch,
		importEncoding:      importEncoding,
		includeHiddenSheets: arg.IncludeHiddenSheets,
	}, nil
}

// shortCWD return short current working directory.
// If current working directory is home directory, return "~".
func (s *state) shortCWD() string {
	// Resolve home cross-platform (os.UserHomeDir uses %USERPROFILE% on Windows,
	// where $HOME is usually unset). Skip abbreviation if it cannot be resolved.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return s.cwd
	}
	return abbreviateHome(s.cwd, home)
}

// abbreviateHome replaces a leading home-directory prefix in cwd with "~". The
// prefix is only replaced when cwd equals home or is a real descendant of home
// at a path-separator boundary. Why not a plain string replace: that rewrites a
// sibling such as "/home/nao2" into "~2" when home is "/home/nao". Both "/" and
// "\\" are accepted as boundaries so the check is correct for Unix and Windows
// paths regardless of the host OS.
func abbreviateHome(cwd, home string) string {
	home = strings.TrimRight(home, `/\`)
	if home == "" {
		return cwd
	}
	if cwd == home {
		return "~"
	}
	if strings.HasPrefix(cwd, home) {
		if rest := cwd[len(home):]; rest[0] == '/' || rest[0] == '\\' {
			return "~" + rest
		}
	}
	return cwd
}

// mode is output mode.
type mode struct {
	w io.Writer
	model.PrintMode
}

// newMode returns mode.
func newMode(w io.Writer, m model.PrintMode) *mode {
	return &mode{
		w:         w,
		PrintMode: m,
	}
}

// AllowsMultipleResults reports whether the current output format can carry more
// than one result set in one stream.
func (m *mode) AllowsMultipleResults() bool {
	return m.PrintMode.AllowsMultipleResults()
}

// displayName returns the user-facing name of the current mode.
func (m *mode) displayName() string {
	return m.String()
}

// changeOutputModeIfNeeded change output mode.
// modeName is new output mode (e.g. table).
//
// The mode-change banner is written to stderr, not stdout. In batch mode a
// `.mode json`/`.mode jsonl` switch is followed by machine-readable output on
// stdout, so a banner there would corrupt it; keeping the status message on
// stderr preserves stdout purity for every mode.
func (m *mode) changeOutputModeIfNeeded(modeName string) error {
	// Selecting the mode that is already in effect is what the caller asked for,
	// so it succeeds silently. Reporting it as an error made a batch script fatal
	// on a line that changed nothing: `sqly --output-format csv data.csv` fed a script opening
	// with `.mode csv` died before running a single query, and so did any script
	// that set the mode defensively. The banner is skipped because nothing
	// changed; an unknown name still fails below.
	if modeName == m.displayName() {
		return nil
	}

	// Resolve the requested name to a target PrintMode before
	// mutating anything, so an invalid name leaves the current mode untouched. The
	// banner suffix flags the dump-only formats whose on-screen output is CSV.
	var (
		target model.PrintMode
		suffix string
	)
	switch modeName {
	case model.PrintModeTable.String():
		target = model.PrintModeTable
	case model.PrintModeMarkdownTable.String():
		target = model.PrintModeMarkdownTable
		suffix = " table"
	case model.PrintModeCSV.String():
		target = model.PrintModeCSV
	case model.PrintModeTSV.String():
		target = model.PrintModeTSV
	case model.PrintModeLTSV.String():
		target = model.PrintModeLTSV
	case model.PrintModeJSON.String():
		target = model.PrintModeJSON
	case model.PrintModeJSONL.String():
		target = model.PrintModeJSONL
	case model.PrintModeExcel.String():
		target = model.PrintModeExcel
		suffix = " (active only when executing .dump, otherwise same as csv mode)"
	case model.PrintModeParquet.String():
		target = model.PrintModeParquet
		suffix = " (active only when executing .dump, otherwise same as csv mode)"
	case model.PrintModeVertical.String():
		target = model.PrintModeVertical
	default:
		return fmt.Errorf("invalid output mode %q: want table, vertical, csv, tsv, ltsv, json, jsonl, markdown, excel, or parquet", modeName)
	}

	fmt.Fprintf(config.Stderr, "Change output mode from %s to %s%s\n", m.displayName(), modeName, suffix)
	m.PrintMode = target
	return nil
}
