package persistence

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/golden"
)

func TestJsonRepositoryList(t *testing.T) {
	t.Parallel()
	t.Run("list and dump json data", func(t *testing.T) {
		t.Parallel()

		r := NewJSONRepository()
		csv, err := r.List(filepath.Join("testdata", "sample.json"))
		if err != nil {
			t.Fatal(err)
		}

		var tmpFile *os.File
		var e error
		if runtime.GOOS != config.Windows {
			tmpFile, e = os.CreateTemp(t.TempDir(), "dump.json")
		} else {
			// See https://github.com/golang/go/issues/51442
			tmpFile, e = os.CreateTemp(os.TempDir(), "dump.json")
		}
		if e != nil {
			t.Fatal(err)
		}

		if err := r.Dump(tmpFile, csv.ToTable()); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "sample_json", got)
	})
}
