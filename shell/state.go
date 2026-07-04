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
	// importMode is the current malformed-row policy for CSV/TSV imports. It is
	// seeded from the --import-mode flag and changed by the .import-mode command.
	importMode model.MalformedRowPolicy
}

// newState return *state.
func newState(arg *config.Arg) (*state, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &state{
		cwd:        dir,
		mode:       newMode(config.Stdout, arg.Output.Mode, arg.Output.JSONTyped),
		importMode: arg.ImportMode,
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

// Typed JSON output mode names accepted by .mode and shown in the prompt. They
// mirror the --json-typed/--ndjson-typed CLI flags.
const (
	outputModeJSONTyped   = "json-typed"
	outputModeNDJSONTyped = "ndjson-typed"
)

// mode is output mode.
type mode struct {
	w io.Writer
	model.PrintMode
	// jsonTyped, when true and PrintMode is JSON or NDJSON, opts query JSON/NDJSON
	// output into the typed contract (native numbers, booleans, and nulls). It is
	// ignored for every other PrintMode.
	jsonTyped bool
}

// newMode returns mode.
func newMode(w io.Writer, m model.PrintMode, jsonTyped bool) *mode {
	return &mode{
		w:         w,
		PrintMode: m,
		jsonTyped: jsonTyped,
	}
}

// displayName returns the user-facing name of the current mode, distinguishing
// the typed JSON variants ("json-typed"/"ndjson-typed") from the default
// string-valued JSON modes. It backs the prompt label and the .mode banner so a
// typed session is visible to the user.
func (m *mode) displayName() string {
	if m.jsonTyped {
		switch m.PrintMode {
		case model.PrintModeJSON:
			return outputModeJSONTyped
		case model.PrintModeNDJSON:
			return outputModeNDJSONTyped
		}
	}
	return m.String()
}

// changeOutputModeIfNeeded change output mode.
// modeName is new output mode (e.g. table).
//
// The mode-change banner is written to stderr, not stdout. In batch mode a
// `.mode json`/`.mode ndjson` switch is followed by machine-readable output on
// stdout, so a banner there would corrupt it; keeping the status message on
// stderr preserves stdout purity for every mode.
func (m *mode) changeOutputModeIfNeeded(modeName string) error {
	if modeName == m.displayName() {
		return fmt.Errorf("already %s mode", modeName)
	}

	// Resolve the requested name to a target PrintMode and typed flag before
	// mutating anything, so an invalid name leaves the current mode untouched. The
	// banner suffix flags the dump-only formats whose on-screen output is CSV.
	var (
		target model.PrintMode
		typed  bool
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
	case model.PrintModeNDJSON.String():
		target = model.PrintModeNDJSON
	case outputModeJSONTyped:
		target, typed = model.PrintModeJSON, true
	case outputModeNDJSONTyped:
		target, typed = model.PrintModeNDJSON, true
	case model.PrintModeExcel.String():
		target = model.PrintModeExcel
		suffix = " (active only when executing .dump, otherwise same as csv mode)"
	case model.PrintModeParquet.String():
		target = model.PrintModeParquet
		suffix = " (active only when executing .dump, otherwise same as csv mode)"
	default:
		return fmt.Errorf("invalid output mode: %s", modeName)
	}

	fmt.Fprintf(config.Stderr, "Change output mode from %s to %s%s\n", m.displayName(), modeName, suffix)
	m.PrintMode = target
	m.jsonTyped = typed
	return nil
}
