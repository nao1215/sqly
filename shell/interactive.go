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

// initialize for Interactive.
func (i *Interactive) initialize(ctx context.Context) error {
	return i.history.initialize(ctx)
}

// recordUserRequest store user input
func (i *Interactive) recordUserRequest(ctx context.Context) error {
	return i.history.recordAndRefreshCache(ctx)
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
