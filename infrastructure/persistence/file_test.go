package persistence

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/sqly/config"
)

func TestFileRepositoryOpen(t *testing.T) {
	t.Parallel()

	t.Run("file open succeeded", func(t *testing.T) {
		t.Parallel()

		fr := NewFileRepository()
		f, err := fr.Open(filepath.Join("testdata", "sample.txt"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		if f == nil {
			t.Fatal("file is nil")
		}
		got := f.Name()
		want := filepath.Join("testdata", "sample.txt")
		if got != want {
			t.Errorf("mismatch got=%s, want=%s", got, want)
		}
	})

	t.Run("because file does not exist, file open failed", func(t *testing.T) {
		t.Parallel()

		fr := NewFileRepository()
		_, err := fr.Open(filepath.Join("testdata", "not_exist.txt"))
		if err == nil {
			t.Fatal("error is nil")
		}
	})
}

func TestFileRepositoryCreate(t *testing.T) {
	if runtime.GOOS == config.Windows {
		t.Skip("skip on windows")
	}

	t.Parallel()

	t.Run("file create succeeded", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		fr := NewFileRepository()
		f, err := fr.Create(filepath.Join(tmpDir, "create.txt"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		if f == nil {
			t.Fatal("file is nil")
		}
		got := f.Name()
		want := filepath.Join(tmpDir, "create.txt")
		if got != want {
			t.Errorf("mismatch got=%s, want=%s", got, want)
		}

		info, err := os.Stat(want)
		if err != nil {
			t.Fatal(err)
		}

		wantMode := os.FileMode(0600)
		gotMode := info.Mode()
		if gotMode != wantMode {
			t.Errorf("mismatch got=%s, want=%s", gotMode, wantMode)
		}
	})

	t.Run("because no permission, file create failed", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		if err := os.Chmod(tmpDir, 0); err != nil {
			t.Fatal(err)
		}

		fr := NewFileRepository()
		_, err := fr.Create(filepath.Join(tmpDir, "create.txt"))
		if err == nil {
			t.Fatal("error is nil")
		}
	})
}
