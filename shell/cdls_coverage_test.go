package shell

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
)

// captureLsStdout runs f with config.Stdout redirected to a buffer and returns
// what it wrote. Kept local to the cd/ls tests so it does not collide with other
// capture helpers in the package.
func captureLsStdout(t *testing.T, f func() error) (string, error) {
	t.Helper()
	old := config.Stdout
	buf := bytes.NewBuffer(nil)
	config.Stdout = buf
	t.Cleanup(func() { config.Stdout = old })
	err := f()
	return buf.String(), err
}

func TestCommandList_lsCommandCoverage(t *testing.T) {
	// These subtests read the filesystem and one changes cwd, so they stay serial.
	c := CommandList{}

	t.Run("rejects more than one argument", func(t *testing.T) {
		err := c.lsCommand(context.Background(), nil, []string{"a", "b"})
		if err == nil || !strings.Contains(err.Error(), "too many arguments") {
			t.Fatalf("want too many arguments error, got %v", err)
		}
	})

	t.Run("reports a missing path as no such file or directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope")
		_, err := captureLsStdout(t, func() error {
			return c.lsCommand(context.Background(), nil, []string{missing})
		})
		if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
			t.Fatalf("want no such file or directory error, got %v", err)
		}
	})

	t.Run("lists a single file as its base name", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "only.csv")
		if err := os.WriteFile(file, []byte("a\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := captureLsStdout(t, func() error {
			return c.lsCommand(context.Background(), nil, []string{file})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(out) != "only.csv" {
			t.Fatalf("want %q, got %q", "only.csv", out)
		}
	})

	t.Run("lists directory entries sorted with a trailing slash on directories", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.csv"), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := captureLsStdout(t, func() error {
			return c.lsCommand(context.Background(), nil, []string{dir})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.Fields(out)
		want := []string{"a.csv", "sub/"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("want %v, got %v", want, got)
		}
	})

	t.Run("lists the current directory when given no argument", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "here.tsv"), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		out, err := captureLsStdout(t, func() error {
			return c.lsCommand(context.Background(), nil, []string{})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(out) != "here.tsv" {
			t.Fatalf("want %q, got %q", "here.tsv", out)
		}
	})
}

func TestCommandList_cdCommandCoverage(t *testing.T) {
	// cdCommand mutates the process working directory, so these stay serial and
	// rely on t.Chdir to restore the original cwd after each subtest.
	c := CommandList{}

	t.Run("rejects more than one argument", func(t *testing.T) {
		s := &Shell{state: &state{}}
		err := c.cdCommand(context.Background(), s, []string{"a", "b"})
		if err == nil || !strings.Contains(err.Error(), "too many arguments") {
			t.Fatalf("want too many arguments error, got %v", err)
		}
	})

	t.Run("changes to the given directory and records its absolute path", func(t *testing.T) {
		start := t.TempDir()
		t.Chdir(start)
		dest := t.TempDir()
		s := &Shell{state: &state{}}
		if err := c.cdCommand(context.Background(), s, []string{dest}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantResolved, err := filepath.EvalSymlinks(dest)
		if err != nil {
			t.Fatal(err)
		}
		gotResolved, err := filepath.EvalSymlinks(s.state.cwd)
		if err != nil {
			t.Fatal(err)
		}
		if gotResolved != wantResolved {
			t.Fatalf("want cwd %q, got %q", wantResolved, gotResolved)
		}
	})

	t.Run("returns an error for a directory that does not exist", func(t *testing.T) {
		t.Chdir(t.TempDir())
		s := &Shell{state: &state{}}
		missing := filepath.Join(t.TempDir(), "absent")
		if err := c.cdCommand(context.Background(), s, []string{missing}); err == nil {
			t.Fatal("want error for missing directory, got nil")
		}
	})

	t.Run("changes to the home directory when given no argument", func(t *testing.T) {
		t.Chdir(t.TempDir())
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			t.Skip("home directory is not resolvable in this environment")
		}
		s := &Shell{state: &state{}}
		if err := c.cdCommand(context.Background(), s, []string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantResolved, err := filepath.EvalSymlinks(home)
		if err != nil {
			t.Skipf("cannot resolve home symlinks: %v", err)
		}
		gotResolved, err := filepath.EvalSymlinks(s.state.cwd)
		if err != nil {
			t.Fatal(err)
		}
		if gotResolved != wantResolved {
			t.Fatalf("want cwd %q, got %q", wantResolved, gotResolved)
		}
	})
}
