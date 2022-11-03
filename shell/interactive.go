package shell

import (
	"fmt"
	"strings"
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
	currentInput string
	promptPrefix string
}

// NewInteractive return *Interactive
func NewInteractive() *Interactive {
	return &Interactive{
		promptPrefix: "sqly>>",
	}
}

// deleteLastInput delete last character.
func (i *Interactive) deleteLastInput() {
	if len(i.currentInput) == 0 {
		return
	}
	fmt.Fprintf(Stdout, "\r%s", strings.Repeat(" ", len(i.promptPrefix+i.currentInput)))
	r := []rune(i.currentInput)
	i.currentInput = string(r[:(len(r) - 1)])
}

// request returns the string entered by the time the user presses Enter.
// Spaces before and after the string are stripped.
func (i *Interactive) request() string {
	return strings.TrimSpace(i.currentInput)
}

// append append character.
func (i *Interactive) append(r rune) {
	i.currentInput += string(r)
}

// reset delete all user input.
func (i *Interactive) resetUserInput() {
	i.currentInput = ""
}
