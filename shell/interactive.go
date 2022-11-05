package shell

import (
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

const (
	runeEnter     rune = '\r'
	runeTabKey    rune = '\t'
	runeEscapeKey rune = '\u001B'
	runeBackSpace rune = '\b'
	runeDelete    rune = '\u007f'
)

// Interactive is user interface that provide command prompt.
type Interactive struct {
	promptPrefix string
	history      *History
}

// NewInteractive return *Interactive
func NewInteractive(h *History) *Interactive {
	return &Interactive{
		promptPrefix: "sqly> ",
		history:      h,
	}
}

// initHistory create history table in DB.
// If there is no directory to store DB files, create it.
func (i *Interactive) initHistory(ctx context.Context) error {
	return i.history.interactor.CreateTable(ctx)
}

// printPrompt print "sqly>>" prompt
func (i *Interactive) printPrompt() {
	fmt.Fprintf(Stdout, "\r%s%s", color.GreenString(i.promptPrefix), i.history.currentInput())
}

// deleteLastInput delete last character.
func (i *Interactive) deleteLastInput() {
	if len(i.history.currentInput()) == 0 {
		return
	}
	r := []rune(i.history.currentInput())
	i.history.replace(string(r[:(len(r) - 1)]))
}

func (i *Interactive) clearLine() {
	fmt.Fprintf(Stdout, "\r%s", strings.Repeat(" ", len(i.promptPrefix)+i.history.maxLength))
}

func (i *Interactive) olderInput() {
	i.history.older()
}

func (i *Interactive) newerInput() {
	i.history.newer()
}

// request returns the string entered by the time the user presses Enter.
// Spaces before and after the string are stripped.
func (i *Interactive) request() string {
	return strings.TrimSpace(i.history.currentInput())
}

// append append character.
func (i *Interactive) append(r rune) {
	i.history.appendChar(r)
}
