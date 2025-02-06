package persistence

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/golden"
)

func TestTsvRepositoryList(t *testing.T) {
	t.Parallel()

	t.Run("list and dump tsv data", func(t *testing.T) {
		t.Parallel()

		r := NewTSVRepository()
		f, err := os.Open(filepath.Join("testdata", "sample.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		tsv, err := r.List(f)
		if err != nil {
			t.Fatal(err)
		}

		var tmpFile *os.File
		var e error
		if runtime.GOOS != config.Windows {
			tmpFile, e = os.CreateTemp(t.TempDir(), "dump.tsv")
		} else {
			// See https://github.com/golang/go/issues/51442
			tmpFile, e = os.CreateTemp(os.TempDir(), "dump.tsv")
		}
		if e != nil {
			t.Fatal(err)
		}

		if err := r.Dump(tmpFile, tsv.ToTable()); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "sample_tsv", got)
	})
}
