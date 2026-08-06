package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
)

// runBatchShell builds a shell from args and feeds it batch input on stdin (no
// TTY), returning the run error. It drives the import path and any helper
// commands the same way `printf ... | sqly ARGS` does.
func runBatchShell(t *testing.T, args []string, stdin string) error {
	t.Helper()
	shell, cleanup, err := newShell(t, args)
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()
	shell.stdin = strings.NewReader(stdin)
	shell.isTTY = func() bool { return false }
	var runErr error
	_ = captureStdout(t, func() {
		runErr = shell.Run(context.Background())
	})
	return runErr
}

// TestImportCollisionRegressions covers the v0.19.0 directory-import collision
// bugs: re-importing a directory-sourced file directly
// should clear the directory marker, a standalone file should be able to replace
// a directory-imported table, a same-source symlink alias is not a collision, and
// directory re-imports must not mis-detect basename-prefix tables as collisions.
func TestImportCollisionRegressions(t *testing.T) {
	const csv = "id,name\n1,a\n"

	t.Run("re-importing a directory-sourced file directly makes it saveable", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		if err := os.Mkdir(sub, 0o750); err != nil {
			t.Fatal(err)
		}
		file := writeCSV(t, sub, "user.csv", csv)

		script := ".import " + file + "\n.save --in-place\n"
		if err := runBatchShell(t, []string{"sqly", sub}, script); err != nil {
			t.Errorf(".save --in-place after re-import failed: %v", err)
		}
	})

	t.Run("standalone file replaces a directory-imported table", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		if err := os.Mkdir(sub, 0o750); err != nil {
			t.Fatal(err)
		}
		writeCSV(t, sub, "user.csv", csv)
		standalone := writeCSV(t, dir, "user.csv", csv)

		script := ".import " + standalone + "\n"
		if err := runBatchShell(t, []string{"sqly", sub}, script); err != nil {
			t.Errorf("standalone .import over a directory table failed: %v", err)
		}
	})

	t.Run("replacing a table with another file says so", func(t *testing.T) {
		// Allowed on purpose (a directory-imported table becomes a named one), but
		// it rebinds the name and drops the session's edits, so it has to say so.
		dir := t.TempDir()
		first := filepath.Join(dir, "one")
		second := filepath.Join(dir, "two")
		for _, d := range []string{first, second} {
			if err := os.Mkdir(d, 0o750); err != nil {
				t.Fatal(err)
			}
		}
		original := writeCSV(t, first, "user.csv", csv)
		replacement := writeCSV(t, second, "user.csv", "id,name\n2,b\n")

		backup := config.Stderr
		var stderr strings.Builder
		config.Stderr = &stderr
		defer func() { config.Stderr = backup }()

		script := ".import " + replacement + "\n"
		if err := runBatchShell(t, []string{"sqly", original}, script); err != nil {
			t.Fatalf(".import over an existing table failed: %v", err)
		}
		got := stderr.String()
		for _, want := range []string{`table "user" now reads`, replacement, original, "is gone"} {
			if !strings.Contains(got, want) {
				t.Errorf("stderr does not mention %q, got %q", want, got)
			}
		}
	})

	t.Run("re-importing the same file says nothing", func(t *testing.T) {
		// A reload is not a replacement: the name still means the same file.
		dir := t.TempDir()
		src := writeCSV(t, dir, "user.csv", csv)

		backup := config.Stderr
		var stderr strings.Builder
		config.Stderr = &stderr
		defer func() { config.Stderr = backup }()

		if err := runBatchShell(t, []string{"sqly", src}, ".import "+src+"\n"); err != nil {
			t.Fatalf(".import of the same file failed: %v", err)
		}
		if strings.Contains(stderr.String(), "now reads") {
			t.Errorf("a reload was reported as a replacement: %q", stderr.String())
		}
	})

	t.Run("same-source symlink alias is not a collision", func(t *testing.T) {
		dir := t.TempDir()
		src := writeCSV(t, dir, "user.csv", csv)
		aliasDir := filepath.Join(dir, "alias")
		if err := os.Mkdir(aliasDir, 0o750); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(aliasDir, "user.csv")
		if err := os.Symlink(src, alias); err != nil {
			t.Skipf("symlink not supported: %v", err)
		}

		script := ".import " + alias + "\n"
		if err := runBatchShell(t, []string{"sqly", src}, script); err != nil {
			t.Errorf("re-import through a symlink alias was treated as a collision: %v", err)
		}
	})

	t.Run("directory re-import does not mis-detect basename-prefix tables", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "d")
		if err := os.Mkdir(sub, 0o750); err != nil {
			t.Fatal(err)
		}
		a := writeCSV(t, sub, "a.csv", "id,name\n1,A\n")
		ab := writeCSV(t, sub, "a_b.csv", "id,name\n2,B\n")

		script := ".import " + sub + "\n"
		if err := runBatchShell(t, []string{"sqly", a, ab}, script); err != nil {
			t.Errorf("directory re-import reported a false prefix collision: %v", err)
		}
	})
}
