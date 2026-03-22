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

func TestCsvRepositoryDump(t *testing.T) {
	t.Run("dump csv data", func(t *testing.T) {
		cr := NewCSVRepository()

		table := readCSVAsTable(t, filepath.Join("testdata", "sample.csv"))

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

// readCSVAsTable reads a CSV file and returns a model.Table for testing Dump.
func readCSVAsTable(t *testing.T, path string) *model.Table {
	t.Helper()

	f, err := os.Open(path) // #nosec G304 - test helper with controlled input
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
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
