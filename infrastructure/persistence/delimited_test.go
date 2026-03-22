package persistence

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/golden"
)

func TestCSVRepositoryDump(t *testing.T) {
	t.Parallel()

	t.Run("dump csv data", func(t *testing.T) {
		t.Parallel()

		cr := NewCSVRepository()

		table := readDelimitedAsTable(t, filepath.Join("testdata", "sample.csv"), ',')

		var tmpFile *os.File
		var err error
		if runtime.GOOS != config.Windows {
			tmpFile, err = os.CreateTemp(t.TempDir(), "dump.csv")
		} else {
			tmpFile, err = os.CreateTemp(os.TempDir(), "dump.csv")
		}
		if err != nil {
			t.Fatal(err)
		}

		if err := cr.Dump(tmpFile, table); err != nil {
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

func TestTSVRepositoryDump(t *testing.T) {
	t.Parallel()

	t.Run("dump tsv data", func(t *testing.T) {
		t.Parallel()

		r := NewTSVRepository()

		table := readDelimitedAsTable(t, filepath.Join("testdata", "sample.tsv"), '\t')

		var tmpFile *os.File
		var err error
		if runtime.GOOS != config.Windows {
			tmpFile, err = os.CreateTemp(t.TempDir(), "dump.tsv")
		} else {
			tmpFile, err = os.CreateTemp(os.TempDir(), "dump.tsv")
		}
		if err != nil {
			t.Fatal(err)
		}

		if err := r.Dump(tmpFile, table); err != nil {
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

// readDelimitedAsTable reads a delimiter-separated file and returns a model.Table for testing Dump.
func readDelimitedAsTable(t *testing.T, path string, delimiter rune) *model.Table {
	t.Helper()

	f, err := os.Open(path) // #nosec G304 - test helper with controlled input
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = delimiter
	var header model.Header
	var records []model.Record
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header == nil {
			header = row
			continue
		}
		records = append(records, model.NewRecord(row))
	}
	return model.NewTable(filepath.Base(path), model.NewHeader(header), records)
}
