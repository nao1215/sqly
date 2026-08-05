package shell

import (
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
)

func featureQuickConfig() *quick.Config {
	return &quick.Config{
		MaxCount: 300,
		Rand:     rand.New(rand.NewSource(7)), //nolint:gosec // deterministic test seed
	}
}

// TestSplitCompletionPrefixProperties checks the invariants splitPathPrefix
// must hold for any typed prefix: the kept base concatenated with the partial
// reconstructs the input exactly, the partial never spans a directory separator,
// and the base is either empty or ends on a separator. These guarantee that
// suggestions built as base+name preserve precisely what the user typed.
func TestSplitCompletionPrefixProperties(t *testing.T) {
	t.Parallel()

	property := func(prefix string) bool {
		readDir, base, partial := splitPathPrefix(prefix, lastUnescapedSeparator)

		if base+partial != prefix {
			return false
		}
		if strings.ContainsAny(partial, `/\`) {
			return false
		}
		if base != "" && !strings.HasSuffix(base, "/") && !strings.HasSuffix(base, `\`) {
			return false
		}
		// readDir is the base when present, otherwise the current directory or
		// the filesystem root for a lone leading separator; it is never empty.
		return readDir != ""
	}
	if err := quick.Check(property, featureQuickConfig()); err != nil {
		t.Error(err)
	}
}

// TestSQLInputCompleteProperties checks the invariants of the interactive
// multiline submit predicate: a ";"-terminated statement is always complete, a
// blank continuation line always force-submits, and a bare single-line query
// (no ";", not a dot-command) is never treated as complete on its own.
func TestSQLInputCompleteProperties(t *testing.T) {
	t.Parallel()

	terminated := func(body string, trailing uint8) bool {
		in := body + ";" + strings.Repeat(" ", int(trailing%4))
		return sqlInputComplete(in)
	}
	if err := quick.Check(terminated, featureQuickConfig()); err != nil {
		t.Errorf("semicolon-terminated input must be complete: %v", err)
	}

	blankLineSubmits := func(body string) bool {
		return sqlInputComplete(body + "\n")
	}
	if err := quick.Check(blankLineSubmits, featureQuickConfig()); err != nil {
		t.Errorf("blank continuation line must force submit: %v", err)
	}

	bareSingleLineContinues := func(word string) bool {
		w := strings.TrimSpace(strings.NewReplacer("\n", "", ";", "").Replace(word))
		if w == "" || strings.HasPrefix(w, ".") {
			return true // empty and dot-commands are complete by design
		}
		return !sqlInputComplete(w)
	}
	if err := quick.Check(bareSingleLineContinues, featureQuickConfig()); err != nil {
		t.Errorf("bare single-line query must continue, not submit: %v", err)
	}
}
