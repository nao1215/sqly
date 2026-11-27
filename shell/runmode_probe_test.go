package shell

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProbeStdinClassifiesARedirectedFile checks what a redirected stdin is
// taken for, which decides whether sqly reads a script from it: a file with
// bytes in it, an empty file (nothing to run), and a descriptor that can no
// longer be examined. The answer is kept for the rest of the run.
func TestProbeStdinClassifiesARedirectedFile(t *testing.T) {
	t.Parallel()

	open := func(t *testing.T, content string) *os.File {
		t.Helper()
		path := filepath.Join(t.TempDir(), "stdin.sql")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path) //nolint:gosec // the test's own temp file
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}

	t.Run("full", func(t *testing.T) {
		t.Parallel()
		s := &Shell{isTTY: func() bool { return false }, stdin: open(t, "SELECT 1;\n")}
		if got := s.stdinKind(); got != stdinFile {
			t.Errorf("stdinKind() = %v, want stdinFile", got)
		}
		s.stdin = nil // the cached answer stands
		if got := s.stdinKind(); got != stdinFile {
			t.Errorf("second stdinKind() = %v, want the cached stdinFile", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		s := &Shell{isTTY: func() bool { return false }, stdin: open(t, "")}
		if got := s.stdinKind(); got != stdinEmpty {
			t.Errorf("stdinKind() = %v, want stdinEmpty", got)
		}
	})
	t.Run("closed", func(t *testing.T) {
		t.Parallel()
		f := open(t, "SELECT 1;\n")
		_ = f.Close()
		s := &Shell{isTTY: func() bool { return false }, stdin: f}
		if got := s.stdinKind(); got != stdinUnknown {
			t.Errorf("stdinKind() = %v, want stdinUnknown", got)
		}
	})
}

// TestRunModeNamesItself checks the words an error message uses for each mode
// and for where its work comes from.
func TestRunModeNamesItself(t *testing.T) {
	t.Parallel()

	for mode, want := range map[runMode][2]string{
		modeInteractiveShell: {"interactive shell", "the statement on the command line"},
		modeStdinScript:      {"stdin script", "the statement on the command line"},
		modeInlineSQL:        {flagSQL, "the statement on the command line"},
		modeSQLFile:          {flagSQLFile, "the statements in the file"},
		modeScriptFile:       {flagScript, "the script in the file"},
		modeInspect:          {flagInspect, "the files named on the command line"},
		runMode(99):          {"unknown", "the statement on the command line"},
	} {
		if got := [2]string{mode.String(), mode.source()}; got != want {
			t.Errorf("mode %d names itself %q, want %q", mode, got, want)
		}
	}
}
