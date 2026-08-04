package shell

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// TestEnsureNotDirectory covers rejecting directory-like output destinations:
// an existing directory and a path ending with a path separator.
func TestEnsureNotDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	t.Run("existing directory is rejected", func(t *testing.T) {
		t.Parallel()
		if err := ensureWritableDestination(dir); err == nil {
			t.Errorf("want error for existing directory, got nil")
		}
	})

	t.Run("path ending with a separator is rejected", func(t *testing.T) {
		t.Parallel()
		if err := ensureWritableDestination(filepath.Join(dir, "outdir") + "/"); err == nil {
			t.Errorf("want error for trailing-separator path, got nil")
		}
	})

	t.Run("plain non-existent file path is accepted", func(t *testing.T) {
		t.Parallel()
		if err := ensureWritableDestination(filepath.Join(dir, "out.csv")); err != nil {
			t.Errorf("want nil for plain file path, got %v", err)
		}
	})
}

// TestResolveOutputTarget_ClassifiesAFormatConflictAsUsage pins which of
// ResolveOutputTarget's refusals is a command-line problem. A mode that
// contradicts the destination extension is two things the user typed
// disagreeing, so it exits 2; a destination that cannot carry the compression it
// was given describes the file, and keeps the class it had.
func TestResolveOutputTarget_ClassifiesAFormatConflictAsUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		explicit    model.ExportFormat
		explicitSet bool
		want        int
	}{
		{
			name:        "a mode that contradicts the extension is a usage error",
			path:        "m.json",
			explicit:    model.ExportCSV,
			explicitSet: true,
			want:        ExitUsage,
		},
		{
			name:        "compression a format cannot carry keeps its own class",
			path:        "out.parquet.gz",
			explicit:    model.ExportParquet,
			explicitSet: true,
			want:        ExitFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := resolveOutputTarget(tt.path, tt.explicit, tt.explicitSet)
			if err == nil {
				t.Fatalf("resolveOutputTarget(%q) = nil error, want one", tt.path)
			}
			if got := ExitCode(err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", err, got, tt.want)
			}
		})
	}
}

// TestPrintResultTable_ReportsAnUnrepresentableValueAsOutput checks the class of
// a result stdout cannot carry. The statement ran and produced its rows, so the
// fix is another --output-format, not another query.
func TestPrintResultTable_ReportsAnUnrepresentableValueAsOutput(t *testing.T) {
	table := model.NewTable(
		"t",
		model.Header{"x"},
		[]model.Record{{"a\tb"}},
	)

	var buf bytes.Buffer
	stdout := config.Stdout
	config.Stdout = &buf
	t.Cleanup(func() { config.Stdout = stdout })

	err := printResultTable(table, model.PrintModeLTSV)
	if err == nil {
		t.Fatal("want an error for a tab inside an LTSV value, got nil")
	}
	if got := ExitCode(err); got != ExitOutput {
		t.Errorf("ExitCode(%v) = %d, want %d", err, got, ExitOutput)
	}
	var pathErr *outputPathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("want an *outputPathError, got %T", err)
	}
	if pathErr.Path != stdoutDestination {
		t.Errorf("Path = %q, want %q", pathErr.Path, stdoutDestination)
	}
}

// TestSameFilePathSymlink verifies that a symlink alias to a file is recognized
// as the same file, so the overwrite guard cannot be bypassed.
func TestSameFilePathSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "user.csv")
	if err := os.WriteFile(src, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.csv")
	if err := os.Symlink(src, alias); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if !sameFilePath(alias, src) {
		t.Errorf("sameFilePath(symlink, target) = false, want true")
	}
	other := filepath.Join(dir, "other.csv")
	if err := os.WriteFile(other, []byte("b\n2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if sameFilePath(other, src) {
		t.Errorf("sameFilePath(unrelated, target) = true, want false")
	}
}
