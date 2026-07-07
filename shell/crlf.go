package shell

import (
	"bytes"
	"io"
	"sync"

	"github.com/nao1215/sqly/config"
)

// crlfWriter translates a lone LF ("\n") into CRLF ("\r\n") on the way to the
// underlying writer, leaving an existing CRLF untouched.
//
// The interactive shell keeps the terminal in raw mode across prompts
// (prompt.WithPersistentRawMode) so keystrokes typed between one result and the
// next prompt are never dropped. Raw mode disables the terminal's own output
// post-processing (OPOST/ONLCR), so command output written with "\n" would
// staircase down the screen instead of returning to the left margin. Wrapping the
// shell's output writers restores the alignment without touching every print
// site.
type crlfWriter struct {
	mu     sync.Mutex
	w      io.Writer
	lastCR bool // the previous byte written was CR, so a following LF already forms CRLF
}

// Write translates lone LFs to CRLF and forwards the result to the wrapped
// writer. It reports len(p) consumed on success so callers see their whole input
// accepted regardless of the extra CR bytes added downstream.
func (c *crlfWriter) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var buf bytes.Buffer
	buf.Grow(len(p) + bytes.Count(p, []byte{'\n'}))
	for _, b := range p {
		if b == '\n' && !c.lastCR {
			buf.WriteByte('\r')
		}
		buf.WriteByte(b)
		c.lastCR = b == '\r'
	}
	if _, err := c.w.Write(buf.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}

// installCRLFTranslation routes the shell's command output (config.Stdout and
// config.Stderr) through crlfWriter for the duration of an interactive session
// and returns a function that restores the original writers. It is a no-op when
// stdout is not a terminal (piped or redirected, and in tests), because raw mode
// only affects a real terminal; there the caller gets back an unchanged restore
// function so batch output keeps plain "\n".
func installCRLFTranslation() func() {
	if !config.IsOutputToTTY() {
		return func() {}
	}
	prevOut, prevErr := config.Stdout, config.Stderr
	config.Stdout = &crlfWriter{w: prevOut}
	config.Stderr = &crlfWriter{w: prevErr}
	return func() {
		config.Stdout = prevOut
		config.Stderr = prevErr
	}
}
