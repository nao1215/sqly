package persistence

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/golden"
)

func Test_csvRepository_List(t *testing.T) {
	t.Run("list and dump csv data", func(t *testing.T) {
		cr := NewCSVRepository()
		f, err := os.Open(filepath.Join("testdata", "sample.csv"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		csv, err := cr.List(f)
		if err != nil {
			t.Fatal(err)
		}

		var tmpFile *os.File
		var e error
		if runtime.GOOS != "windows" {
			tmpFile, e = os.CreateTemp(t.TempDir(), "dump.csv")
		} else {
			// See https://github.com/golang/go/issues/51442
			tmpFile, e = os.CreateTemp(os.TempDir(), "dump.csv")
		}
		if e != nil {
			t.Fatal(err)
		}

		if err := cr.Dump(tmpFile, csv.ToTable()); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "sample_csv", got)
	})
}
