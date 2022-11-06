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
	cursor       *cursor
}

// NewInteractive return *Interactive
func NewInteractive(h *History) *Interactive {
	return &Interactive{
		promptPrefix: "sqly> ",
		history:      h,
		cursor:       newCursor(),
	}
}

// initialize for Interactive.
func (i *Interactive) initialize(ctx context.Context) error {
	return i.history.initialize(ctx)
}

// recordUserRequest store user input
func (i *Interactive) recordUserRequest(ctx context.Context) error {
	if err := i.history.recordAndRefreshCache(ctx); err != nil {
		return err
	}
	i.cursor.moveHead() // reset
	return nil
}

// printPrompt print "sqly>>" prompt
func (i *Interactive) printPrompt() {
	fmt.Fprintf(Stdout, "\r%s%s", color.GreenString(i.promptPrefix), i.history.currentInput())
}

// deleteChar delete one character.
func (i *Interactive) deleteChar() {
	if i.history.currentInputLen() == 0 {
		//beep
		return
	}

	if i.cursor.position() < 0 {
		i.cursor.moveHead()
		// beep
		return
	}

	r := []rune(i.history.currentInput())
	if i.cursor.position() == i.history.currentInputLen() {
		i.history.replace(string(r[:(len(r) - 1)]))
	} else if i.cursor.position() == 0 {
		// beep
		return
	} else {
		i.history.replace(string(r[:i.cursor.position()-1]) + string(r[i.cursor.position():]))
	}
	i.cursor.moveLeft()
}

func (i *Interactive) clearLine() {
	fmt.Fprintf(Stdout, "\r%s", strings.Repeat(" ", len(i.promptPrefix)+i.history.maxLength))
}

func (i *Interactive) olderInput() {
	i.history.older()
	i.cursorEnd()
}

func (i *Interactive) newerInput() {
	i.history.newer()
	i.cursorEnd()
}

// request returns the string entered by the time the user presses Enter.
// Spaces before and after the string are stripped.
func (i *Interactive) request() string {
	return strings.TrimSpace(i.history.currentInput())
}

// append append character.
func (i *Interactive) append(r rune) {
	i.history.appendChar(r, i.cursor.position())
	if r == runeBackSpace {
		return
	}
	i.cursor.moveRight()
}

func (i *Interactive) cursorLeft() {
	if i.cursor.position() <= 0 {
		i.cursor.moveHead()
		// beep
		return
	}
	i.append(runeBackSpace)
	i.cursor.moveLeft()
}

func (i *Interactive) cursorRight() {
	if i.cursor.position() >= i.history.currentInputLen() {
		// beep
		return
	}
	i.cursor.moveRight()
	i.history.replace(strings.Replace(i.history.currentInput(), "\b", "", 1))
}

func (i *Interactive) cursorEnd() {
	i.cursor.set(i.history.currentInputLen())
}
