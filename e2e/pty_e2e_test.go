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
	"bytes"
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
	return startPTYSessionInDir(t, repoRoot(), args...)
}

func startPTYSessionInDir(t *testing.T, dir string, args ...string) *ptySession {
	t.Helper()

	home := t.TempDir()
	cmd := exec.Command(sqlyBin, args...)
	cmd.Dir = dir
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

// sendEOF delivers Ctrl-D to the prompt. Under the PTY-backed interactive
// tests this is the most reliable way to end a settled session because it is a
// single control byte rather than a multi-character command plus Enter.
func (s *ptySession) sendEOF() {
	s.t.Helper()
	s.write("\x04")
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

// TestInteractivePTY_QueryRoundTripAndExitCleanly covers the core interactive
// contract: start the real prompt with imported CSV data, run a SELECT, see the
// result rendered to the terminal, and then terminate the settled prompt
// cleanly.
//
// This is the primary regression guard for the interactive prompt path that the
// batch smoke tests cannot reach.
//
// The PTY-only atago suite exits the prompt with EOF instead of typing ".exit":
// after a query has just finished redrawing the prompt, a multi-byte helper
// command plus Enter can race the prompt's read-readiness and become flaky,
// while a single Ctrl-D byte is stable. The ".exit" helper itself is still
// covered by unit tests; this smoke test focuses on the real PTY query
// round-trip.
func TestInteractivePTY_QueryRoundTripAndExitCleanly(t *testing.T) {
	s := startPTYSession(t, filepath.Join("testdata", "user.csv"))
	t.Cleanup(s.close)

	// Wait until the prompt is in raw mode and ready for keystrokes.
	s.waitReady(startupTimeout)

	// A real keystroke round-trip: type a SELECT and press Enter. The value
	// "Rachel" comes from testdata/user.csv (booker12 / Rachel Booker).
	s.submitLine("SELECT first_name FROM user WHERE user_name = 'booker12';")
	s.waitFor("Rachel", ioTimeout)

	// Let the prompt finish redrawing the fresh empty line; EOF ends the session
	// only when the input buffer is empty.
	time.Sleep(500 * time.Millisecond)
	s.sendEOF()
	if code := s.waitExit(exitTimeout); code != 0 {
		t.Fatalf("interactive shell: EOF produced exit code %d, want 0", code)
	}
}

// TestInteractivePTY_ExitOnCtrlD covers the documented EOF exit path at an
// otherwise idle prompt. Ctrl-D on an empty line must terminate the session
// cleanly with status 0.
func TestInteractivePTY_ExitOnCtrlD(t *testing.T) {
	s := startPTYSession(t, filepath.Join("testdata", "user.csv"))
	t.Cleanup(s.close)

	s.waitReady(startupTimeout)

	s.sendEOF()
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
// All queries plus a trailing EOF are written as a single burst so the whole
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
	burst.WriteByte('\x04')

	// One write, no inter-line pauses: the whole script arrives at once.
	s.write(burst.String())

	// Every marker must appear; a dropped line would never echo or execute.
	for i := range lines {
		s.waitFor(fmt.Sprintf("sqlymark%d", i), ioTimeout)
	}

	// The trailing EOF must still be reached, proving the session processed the
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

// TestInteractivePTY_FinancialImportRollback drives the built sqly binary
// through the real shell so the process-global ACH/Fedwire metadata is present
// before a later multi-file import fails. It verifies both the SQLite tables and
// the subsequent native .save output, which cannot be covered by an in-process
// adapter test alone.
func TestInteractivePTY_FinancialImportRollback(t *testing.T) {
	dir := t.TempDir()
	ach := filepath.Join(dir, "original.ach")
	fed := filepath.Join(dir, "original.fed")
	bad := filepath.Join(dir, "broken.json")
	copySmokeFixture(t, filepath.Join(repoRoot(), "testdata", "ppd-debit.ach"), ach)
	copySmokeFixture(t, filepath.Join(repoRoot(), "testdata", "customer-transfer.fed"), fed)
	if err := os.WriteFile(bad, []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	verifyFinancialRollback(t, ach, bad, "original_entries", "ACH")
	verifyFinancialRollback(t, fed, bad, "original_message", "Fedwire")
}

// TestInteractivePTY_FinancialImportRollbackWithoutExistingState proves that a
// failed import does not leave a financial registry behind when its SQLite
// transaction created no durable tables at all. The final .save is important:
// a stale registry would make the command try to reconstruct a file whose
// tables were rolled back.
func TestInteractivePTY_FinancialImportRollbackWithoutExistingState(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(bad, []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		file  string
		table string
		label string
	}{
		{name: "ACH", file: "ppd-debit.ach", table: "original_entries", label: "ACH_EMPTY"},
		{name: "Fedwire", file: "customer-transfer.fed", table: "original_message", label: "FEDWIRE_EMPTY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			valid := filepath.Join(dir, tc.file)
			copySmokeFixture(t, filepath.Join(repoRoot(), "testdata", tc.file), valid)
			s := startPTYSessionInDir(t, dir)
			t.Cleanup(s.close)
			s.waitReady(startupTimeout)
			s.write(strings.Join([]string{
				".import " + tc.file + " " + filepath.Base(bad),
				"SELECT '" + tc.label + "' AS state, COUNT(*) AS rows FROM " + tc.table + ";",
				".save --in-place",
			}, "\r") + "\r\x04")
			s.waitFor("failed", ioTimeout)
			s.waitFor(tc.label, ioTimeout)
			if code := s.waitExit(exitTimeout); code != 0 {
				t.Fatalf("%s empty-state rollback session exit code = %d; output=%s", tc.name, code, s.output())
			}
			output := s.output()
			if !strings.Contains(output, "no such table") {
				t.Fatalf("%s rollback unexpectedly left a table; output=%s", tc.name, output)
			}
			if strings.Contains(output, "no "+tc.name+" TableSet found") {
				t.Fatalf("%s registry was published before the failed transaction; output=%s", tc.name, output)
			}
		})
	}
}

// TestInteractivePTY_PadRollbackKeepsExistingTable verifies the row-mismatch
// through the real shell after a prior table already exists. The failed
// multi-file import must not replace or remove that table.
func TestInteractivePTY_PadRollbackKeepsExistingTable(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.csv")
	valid := filepath.Join(dir, "valid.csv")
	long := filepath.Join(dir, "long.csv")
	for path, data := range map[string]string{
		existing: "id,name\n7,original\n",
		valid:    "id,name\n1,new\n",
		long:     "id,name\n1,new,discard-me\n",
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := startPTYSessionInDir(t, dir, filepath.Base(existing))
	t.Cleanup(s.close)
	s.waitReady(startupTimeout)
	s.write(strings.Join([]string{
		".row-mismatch pad",
		".import " + filepath.Base(valid) + " " + filepath.Base(long),
		"SELECT 'PAD_EXISTING' AS state, COUNT(*) AS rows FROM existing;",
	}, "\r") + "\r\x04")
	s.waitFor("refuses to truncate", ioTimeout)
	s.waitFor("PAD_EXISTING", ioTimeout)
	s.waitFor("|    1 |", ioTimeout)
	if code := s.waitExit(exitTimeout); code == 0 {
		// Batch command errors are surfaced as a non-zero process status; the
		// interactive shell may still terminate cleanly after the query, so the
		// assertion below is intentionally about the preserved row and not this
		// shell-specific exit convention.
		t.Logf("interactive pad rollback ended cleanly after reporting the import error")
	}
}

func verifyFinancialRollback(t *testing.T, source, bad, table, label string) {
	t.Helper()
	before, err := os.ReadFile(source) //nolint:gosec // test fixture created in TempDir
	if err != nil {
		t.Fatal(err)
	}
	s := startPTYSessionInDir(t, filepath.Dir(source), filepath.Base(source))
	t.Cleanup(s.close)
	s.waitReady(startupTimeout)
	// Send the whole scenario as one burst. This mirrors a user pasting a
	// script into the interactive terminal and, importantly, lets the shell's
	// persistent raw-mode reader consume the next line only after the previous
	// command has completed. Waiting a guessed amount of time after .save is
	// inherently racy: a slow filesystem can still lose the next keystrokes.
	s.write(strings.Join([]string{
		"SELECT '" + label + "_BEFORE' AS state, COUNT(*) AS rows FROM " + table + ";",
		".save --in-place",
		"SELECT '" + label + "_FIRST_SAVE_DONE';",
		".import " + filepath.Base(source) + " " + filepath.Base(bad),
		"SELECT '" + label + "_AFTER' AS state, COUNT(*) AS rows FROM " + table + ";",
		".save --in-place",
		"SELECT '" + label + "_SAVE_DONE';",
	}, "\r") + "\r\x04")
	s.waitFor(label+"_BEFORE", ioTimeout)
	s.waitFor(label+"_FIRST_SAVE_DONE", ioTimeout)
	s.waitFor("failed", ioTimeout)
	s.waitFor(label+"_AFTER", ioTimeout)
	s.waitFor(label+"_SAVE_DONE", ioTimeout)
	if code := s.waitExit(exitTimeout); code != 0 {
		t.Fatalf("%s rollback session exit code = %d; output=%s", label, code, s.output())
	}
	if got, err := os.ReadFile(source); err != nil || !bytes.Equal(got, before) {
		t.Fatalf("%s after rollback/save changed: err=%v equal=%v", label, err, err == nil && bytes.Equal(got, before))
	}
	if output := s.output(); strings.Contains(output, "no "+label+" TableSet found") {
		t.Fatalf("rollback removed the existing %s registry: %s", label, output)
	}
}

func copySmokeFixture(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source) //nolint:gosec // source is a repository test fixture
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
