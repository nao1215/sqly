package persistence

import (
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/testutil"
)

func TestCSVRepositoryDump(t *testing.T) {
	t.Parallel()

	t.Run("dump csv data", func(t *testing.T) {
		t.Parallel()

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

		if err := DumpCSV(tmpFile, table); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		testutil.AssertFileEquals(t, filepath.Join("testdata", "golden", "sample_csv.golden"), got)
	})
}

func TestTSVRepositoryDump(t *testing.T) {
	t.Parallel()

	t.Run("dump tsv data", func(t *testing.T) {
		t.Parallel()

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

		if err := DumpTSV(tmpFile, table); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		testutil.AssertFileEquals(t, filepath.Join("testdata", "golden", "sample_tsv.golden"), got)
	})
}

// readDelimitedAsTable reads a delimiter-separated file and returns a model.Table for testing Dump.
func readDelimitedAsTable(t *testing.T, path string, delimiter rune) *model.Table {
	t.Helper()

	f, err := os.Open(path) // #nosec G304 - test helper with controlled input
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

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
		records = append(records, model.Record(row))
	}
	return model.NewTable(filepath.Base(path), header, records)
}

// TestDelimitedRepositoryDumpLoneEmptyField pins that a one-column row whose
// only value is empty is written as "".
//
// Written plainly it is a blank line, and a blank line is not a record: a reader
// skips it. `SELECT v FROM t` over alice, "", bob wrote three rows and read back
// as two, and the export reported success.
func TestDelimitedRepositoryDumpLoneEmptyField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dump func(io.Writer, *model.Table) error
		want string
	}{
		{name: "csv", dump: DumpCSV, want: "v\nalice\n\"\"\nbob\n"},
		{name: "tsv", dump: DumpTSV, want: "v\nalice\n\"\"\nbob\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			table := model.NewTable("t", model.Header{"v"}, []model.Record{
				{"alice"}, {""}, {"bob"},
			})

			var buf bytes.Buffer
			if err := tt.dump(&buf, table); err != nil {
				t.Fatalf("Dump() error = %v, want nil", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("Dump() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("a multi-column row of empty values keeps its delimiters", func(t *testing.T) {
		t.Parallel()

		table := model.NewTable("t", model.Header{"a", "b"}, []model.Record{{"", ""}})

		var buf bytes.Buffer
		if err := DumpCSV(&buf, table); err != nil {
			t.Fatalf("Dump() error = %v, want nil", err)
		}
		if got, want := buf.String(), "a,b\n,\n"; got != want {
			t.Errorf("Dump() = %q, want %q", got, want)
		}
	})

	t.Run("a lone empty header is written the same way", func(t *testing.T) {
		t.Parallel()

		table := model.NewTable("t", model.Header{""}, []model.Record{{"x"}})

		var buf bytes.Buffer
		if err := DumpCSV(&buf, table); err != nil {
			t.Fatalf("Dump() error = %v, want nil", err)
		}
		if got, want := buf.String(), "\"\"\nx\n"; got != want {
			t.Errorf("Dump() = %q, want %q", got, want)
		}
	})
}
