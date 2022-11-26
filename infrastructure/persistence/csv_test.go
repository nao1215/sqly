package persistence

import (
	"os"
	"path/filepath"
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

		file := filepath.Join(t.TempDir(), "dump.csv")
		f2, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0664)
		if err != nil {
			t.Fatal(err)
		}
		defer f2.Close()

		if err := cr.Dump(f2, csv.ToTable()); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "sample_csv", got)
	})
}
