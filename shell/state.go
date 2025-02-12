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
}

// newState return *state.
func newState(arg *config.Arg) (*state, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &state{
		cwd:  dir,
		mode: newMode(config.Stdout, arg.Output.Mode),
	}, nil
}

// shortCWD return short current working directory.
// If current working directory is home directory, return "~".
func (s *state) shortCWD() string {
	home := os.Getenv("HOME")
	if s.cwd == home {
		return "~"
	}
	return strings.Replace(s.cwd, home, "~", 1)
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

// changeOutputModeIfNeeded change output mode.
// modeName is new output mode (e.g. table).
func (m *mode) changeOutputModeIfNeeded(modeName string) error {
	if modeName == m.String() {
		return fmt.Errorf("already %s mode", modeName)
	}

	switch modeName {
	case model.PrintModeTable.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to %s\n", m.String(), model.PrintModeTable.String())
		m.PrintMode = model.PrintModeTable
	case model.PrintModeMarkdownTable.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to %s table\n", m.String(), model.PrintModeMarkdownTable.String())
		m.PrintMode = model.PrintModeMarkdownTable
	case model.PrintModeCSV.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to %s\n", m.String(), model.PrintModeCSV.String())
		m.PrintMode = model.PrintModeCSV
	case model.PrintModeTSV.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to %s\n", m.String(), model.PrintModeTSV.String())
		m.PrintMode = model.PrintModeTSV
	case model.PrintModeLTSV.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to %s\n", m.String(), model.PrintModeLTSV.String())
		m.PrintMode = model.PrintModeLTSV
	case model.PrintModeJSON.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to %s\n", m.String(), model.PrintModeJSON.String())
		m.PrintMode = model.PrintModeJSON
	case model.PrintModeExcel.String():
		fmt.Fprintf(config.Stdout, "Change output mode from %s to %s (active only when executing .dump, otherwise same as csv mode)\n",
			m.String(), model.PrintModeExcel.String())
		m.PrintMode = model.PrintModeExcel
	default:
		return fmt.Errorf("invalid output mode: %s", modeName)
	}
	return nil
}
