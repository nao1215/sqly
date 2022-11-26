package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/golden"
)

func Test_jsonRepository_List(t *testing.T) {
	t.Run("list and dump json data", func(t *testing.T) {
		r := NewJSONRepository()

		csv, err := r.List(filepath.Join("testdata", "sample.json"))
		if err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.json")
		f2, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0664)
		if err != nil {
			t.Fatal(err)
		}
		defer f2.Close()

		if err := r.Dump(f2, csv.ToTable()); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "sample_json", got)
	})
}
