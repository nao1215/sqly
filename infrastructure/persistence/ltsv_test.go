package persistence

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/testutil"
)

func TestLtsvRepositoryDump(t *testing.T) {
	t.Parallel()

	t.Run("dump ltsv data", func(t *testing.T) {
		t.Parallel()

		table := readLTSVAsTable(t, filepath.Join("testdata", "sample.ltsv"))

		var tmpFile *os.File
		var err error
		if runtime.GOOS != config.Windows {
			tmpFile, err = os.CreateTemp(t.TempDir(), "dump.ltsv")
		} else {
			tmpFile, err = os.CreateTemp(os.TempDir(), "dump.ltsv")
		}
		if err != nil {
			t.Fatal(err)
		}

		if err := DumpLTSV(tmpFile, table); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		testutil.AssertFileEquals(t, filepath.Join("testdata", "golden", "sample_ltsv.golden"), got)
	})
}

// readLTSVAsTable reads an LTSV file and returns a model.Table for testing Dump.
func readLTSVAsTable(t *testing.T, path string) *model.Table {
	t.Helper()

	f, err := os.Open(path) // #nosec G304 - test helper with controlled input
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.Comma = '\t'
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
			for _, v := range row {
				idx := strings.Index(v, ":")
				if idx > 0 {
					header = append(header, v[:idx])
				}
			}
		}
		var record model.Record
		for _, v := range row {
			_, after, ok := strings.Cut(v, ":")
			if ok {
				record = append(record, after)
			} else {
				record = append(record, v)
			}
		}
		records = append(records, record)
	}
	return model.NewTable(filepath.Base(path), header, records)
}
