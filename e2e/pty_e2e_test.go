//go:build smoke && !windows

// This file adds PTY-backed end-to-end coverage for the REAL interactive shell.
// The other tests in this package and the ShellSpec suite drive sqly only in
// batch / non-interactive mode (piped stdin, --sql flags). They never exercise
// the interactive prompt, which only starts when stdin is a TTY and which reads
// keystrokes through the terminal backend (go-tty / /dev/tty) rather than a pipe.
//
// To cover that path we allocate a pseudo-terminal, spawn the built sqly binary
// with its stdio attached to the PTY slave, type into the PTY master like a real
// user, and assert on what the shell renders back. PTYs are Unix-only, so this
// file is gated behind "!windows" (and the shared "smoke" build tag); on Windows
// the batch smoke tests still provide binary-level coverage.
//
// Failure messages here intentionally say "interactive shell" so a regression in
// the prompt path is never mistaken for a batch-mode regression.
package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// ansiEscape matches CSI / OSC style ANSI escape sequences and stray control
// bytes so assertions can be made against the human-visible text the interactive
// shell renders. The prompt emits color, cursor-movement, and redraw sequences
// that would otherwise break naive substring matching.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[()][0-9A-Za-z]|[\x00-\x08\x0b\x0c\x0e-\x1f]`)

// stripANSI removes ANSI escape sequences and other control bytes from s,
// leaving the plain text the user would perceive on screen. Matching on this is
// tolerant of color, prompt redraw, and cursor movement emitted by the prompt.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// ptySession drives the built sqly binary under a pseudo-terminal. It owns the
// PTY master, the spawned process, and a background drain that accumulates all
// terminal output so tests can wait for substrings without risking a blocked
// child (a full PTY buffer would otherwise stall the shell).
type ptySession struct {
	t    *testing.T
	cmd  *exec.Cmd
	ptmx *os.File

	mu  sync.Mutex
	buf strings.Builder
}

// startPTYSession launches sqly under a PTY with the given arguments and a
// hermetic HOME / history DB so the interactive run never touches real config
// state. It begins draining output immediately and returns once the process is
// running.
func startPTYSession(t *testing.T, args ...string) *ptySession {
	t.Helper()

	home := t.TempDir()
	cmd := exec.Command(sqlyBin, args...)
	cmd.Dir = repoRoot()
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"USERPROFILE="+home,
		"SQLY_HISTORY_DB_PATH="+filepath.Join(home, "history.db"),
		"TERM=xterm-256color",
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("interactive shell: failed to start sqly under a PTY: %v", err)
	}
	// A fixed window size keeps prompt rendering deterministic across machines.
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Logf("interactive shell: could not set PTY size (continuing): %v", err)
	}

	s := &ptySession{t: t, cmd: cmd, ptmx: ptmx}
	go s.drain()
	return s
}

// drain continuously copies PTY output into the session buffer until the master
// is closed or the child exits. Reads from a closed/exited PTY return an error
// (often EIO on Linux); that simply ends the drain.
func (s *ptySession) drain() {
	chunk := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(chunk)
		if n > 0 {
			s.mu.Lock()
			s.buf.Write(chunk[:n])
			s.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// output returns the plain-text (ANSI-stripped) terminal output captured so far.
func (s *ptySession) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return stripANSI(s.buf.String())
}

// rawOutput returns the captured output without stripping, for diagnostics.
func (s *ptySession) rawOutput() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// write types the given bytes into the PTY as if entered at the keyboard.
func (s *ptySession) write(b string) {
	s.t.Helper()
	if _, err := s.ptmx.Write([]byte(b)); err != nil {
		s.t.Fatalf("interactive shell: failed to write %q to the prompt: %v", b, err)
	}
}

// submitLine enters a single SQL/command line and presses Enter. The line and
// its terminating carriage return are written as one burst: delivering all the
// bytes together (rather than dribbling characters in, or sending Enter as a
// separate write) keeps the interactive completer from materializing a popup
// that then swallows or rewrites the Enter as an accept-completion. A small pause
// first lets any pending prompt redraw settle.
func (s *ptySession) submitLine(line string) {
	s.t.Helper()
	time.Sleep(100 * time.Millisecond)
	s.write(line + "\r")
}

// waitForRaw polls the unstripped captured output until want appears or the
// timeout elapses. It is used to detect terminal control sequences (e.g. the
// bracketed-paste-enable the prompt emits once it is in raw mode) that stripANSI
// would otherwise remove.
func (s *ptySession) waitForRaw(want string, timeout time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.rawOutput(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("interactive shell: timed out after %s waiting for control sequence %q.\n--- raw output ---\n%q",
		timeout, want, s.rawOutput())
}

// waitReady blocks until the interactive prompt is fully initialized and ready
// for keystrokes. The welcome banner prints before the prompt session enters raw
// mode and enables bracketed paste; typing during that window drops leading
// bytes. Waiting for the bracketed-paste-enable sequence (and a short settle)
// guarantees the terminal is in raw mode before we type.
func (s *ptySession) waitReady(timeout time.Duration) {
	s.t.Helper()
	s.waitFor("sqly", timeout)           // welcome banner
	s.waitForRaw("\x1b[?2004h", timeout) // prompt entered raw mode + bracketed paste
	time.Sleep(200 * time.Millisecond)   // settle so the first keystrokes are not raced
}

// waitFor polls the captured output until want appears or the timeout elapses.
// A timeout fails the test with the accumulated output so a hang surfaces as a
// clear interactive-shell failure rather than blocking CI until the job dies.
func (s *ptySession) waitFor(want string, timeout time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.output(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("interactive shell: timed out after %s waiting for %q.\n--- visible output ---\n%s\n--- raw output ---\n%q",
		timeout, want, s.output(), s.rawOutput())
}

// waitExit waits for the child process to exit and returns its exit code. A
// timeout is enforced by killing the process so a stuck shell fails fast.
func (s *ptySession) waitExit(timeout time.Duration) int {
	s.t.Helper()

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case err := <-done:
		_ = s.ptmx.Close()
		if err == nil {
			return 0
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		s.t.Fatalf("interactive shell: process ended with an unexpected error: %v\n--- visible output ---\n%s", err, s.output())
		return -1
	case <-time.After(timeout):
		_ = s.cmd.Process.Kill()
		_ = s.ptmx.Close()
		s.t.Fatalf("interactive shell: process did not exit within %s (expected EOF/.exit to terminate it).\n--- visible output ---\n%s", timeout, s.output())
		return -1
	}
}

// close best-effort tears down a session that a test did not exit cleanly.
func (s *ptySession) close() {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.ptmx.Close()
}

const (
	// startupTimeout is generous because the first read also pays for process
	// start, file import, and prompt-session initialization.
	startupTimeout = 30 * time.Second
	// ioTimeout bounds an individual command round-trip.
	ioTimeout = 15 * time.Second
	// exitTimeout bounds clean shutdown after EOF / .exit.
	exitTimeout = 15 * time.Second
)

// TestInteractivePTY_QueryRoundTripAndExitOnExit covers the core interactive
// contract: start the real prompt with imported CSV data, run a SELECT, see the
// result rendered to the terminal, and quit with the ".exit" command (exit 0).
//
// This is the primary regression guard for the interactive prompt path that the
// batch smoke tests cannot reach.
func TestInteractivePTY_QueryRoundTripAndExitOnExit(t *testing.T) {
	s := startPTYSession(t, filepath.Join("testdata", "user.csv"))
	t.Cleanup(s.close)

	// Wait until the prompt is in raw mode and ready for keystrokes.
	s.waitReady(startupTimeout)

	// A real keystroke round-trip: type a SELECT and press Enter. The value
	// "Rachel" comes from testdata/user.csv (booker12 / Rachel Booker).
	s.submitLine("SELECT first_name FROM user WHERE user_name = 'booker12';")
	s.waitFor("Rachel", ioTimeout)

	// Quit via the ".exit" helper command and confirm a clean exit.
	s.submitLine(".exit")
	if code := s.waitExit(exitTimeout); code != 0 {
		t.Fatalf("interactive shell: .exit produced exit code %d, want 0", code)
	}
}

// TestInteractivePTY_ExitOnCtrlD covers the second documented way to leave the
// interactive shell: Ctrl-D (EOF, byte 0x04) on an empty line. The prompt treats
// this as a clean termination, so the process must exit 0.
func TestInteractivePTY_ExitOnCtrlD(t *testing.T) {
	s := startPTYSession(t, filepath.Join("testdata", "user.csv"))
	t.Cleanup(s.close)

	s.waitReady(startupTimeout)

	// Run one query so the round-trip is exercised before the EOF exit, then send
	// Ctrl-D on an empty line to end the session.
	s.submitLine("SELECT last_name FROM user WHERE user_name = 'smith79';")
	s.waitFor("Smith", ioTimeout)

	// Let the prompt finish redrawing the fresh empty line; Ctrl-D ends the
	// session only when the input buffer is empty, so the redraw must settle first.
	time.Sleep(500 * time.Millisecond)
	s.write("\x04") // Ctrl-D / EOF
	if code := s.waitExit(exitTimeout); code != 0 {
		t.Fatalf("interactive shell: Ctrl-D produced exit code %d, want 0", code)
	}
}

// rawCount returns how many times want appears in the unstripped captured output.
// It is used to assert on the raw terminal control bytes (the bracketed-paste
// enable the prompt emits once when it enters raw mode) that stripANSI removes.
func (s *ptySession) rawCount(want string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Count(s.buf.String(), want)
}

// bracketedPasteEnable is the control sequence the prompt writes each time it
// enters raw mode. sqly holds the terminal in raw mode for the whole session
// (prompt.WithPersistentRawMode), so it appears exactly once; if the shell
// regressed to re-acquiring raw mode on every line it would appear once per
// prompt, which is the toggling that lets input be dropped between lines.
const bracketedPasteEnable = "\x1b[?2004h"

// TestInteractivePTY_RapidConsecutiveLinesNotLost reproduces the interactive
// input-loss bug (prompt issue #10): a driver that dumps several lines back to
// back — as a pipe or pty driver does — with no delay between them must have
// every line consumed. Before the fix the shell restored and re-acquired raw mode
// around every command, so a line already buffered when the next prompt was
// rendered could be dropped in the mode-switch window and the session would hang.
//
// All queries plus a trailing ".exit" are written as a single burst so the whole
// script is buffered before the shell reads it, exercising the many-lines-at-once
// path deterministically. Each query selects a distinct marker, so a lost line
// surfaces as a missing marker or as the process failing to exit. The test also
// asserts raw mode was entered exactly once, pinning the persistent-raw-mode
// contract that closes the loss window.
func TestInteractivePTY_RapidConsecutiveLinesNotLost(t *testing.T) {
	s := startPTYSession(t, filepath.Join("testdata", "user.csv"))
	t.Cleanup(s.close)

	s.waitReady(startupTimeout)

	const lines = 10
	var burst strings.Builder
	for i := range lines {
		// A distinct string literal per line; "sqlymark<i>" is unlikely to collide
		// with any other terminal text, so its presence proves the line reached the
		// prompt's read loop rather than being dropped in a mode-switch window.
		fmt.Fprintf(&burst, "SELECT 'sqlymark%d';\r", i)
	}
	burst.WriteString(".exit\r")

	// One write, no inter-line pauses: the whole script arrives at once.
	s.write(burst.String())

	// Every marker must appear; a dropped line would never echo or execute.
	for i := range lines {
		s.waitFor(fmt.Sprintf("sqlymark%d", i), ioTimeout)
	}

	// The trailing ".exit" must still be reached, proving the session processed the
	// entire burst without hanging on a lost line.
	if code := s.waitExit(exitTimeout); code != 0 {
		t.Fatalf("interactive shell: rapid burst produced exit code %d, want 0 (a lost line would hang the session)", code)
	}

	// Raw mode must have been entered once for the whole session, not per line.
	// Per-line re-acquisition is the toggling that opens the input-loss window.
	if got := s.rawCount(bracketedPasteEnable); got != 1 {
		t.Fatalf("interactive shell: raw mode entered %d times across the session, want 1 (persistent raw mode)", got)
	}
}
