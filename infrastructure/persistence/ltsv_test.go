package persistence

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/golden"
	"github.com/nao1215/sqly/infrastructure"
)

func TestLtsvRepositoryLabelAndData(t *testing.T) {
	t.Parallel()

	type args struct {
		s string
	}
	tests := []struct {
		name    string
		lr      *ltsvRepository
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name: "get label and data",
			lr:   &ltsvRepository{},
			args: args{
				s: "label:data",
			},
			want:    "label",
			want1:   "data",
			wantErr: false,
		},
		{
			name: "error happen because data with out label",
			lr:   &ltsvRepository{},
			args: args{
				s: "",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "error happen because string has only delimiter':'",
			lr:   &ltsvRepository{},
			args: args{
				s: ":",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lr := &ltsvRepository{}
			got, got1, err := lr.labelAndData(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ltsvRepository.labelAndField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ltsvRepository.labelAndField() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ltsvRepository.labelAndField() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}

	t.Run("failed to get label data returns ErrNoLabel", func(t *testing.T) {
		t.Parallel()

		lr := &ltsvRepository{}
		_, _, err := lr.labelAndData("")
		if !errors.Is(err, infrastructure.ErrNoLabel) {
			t.Errorf("expected ErrNoLabel, got: %v", err)
		}
	})
}

func TestLtsvRepositoryDump(t *testing.T) {
	t.Parallel()

	t.Run("dump ltsv data", func(t *testing.T) {
		t.Parallel()

		r := NewLTSVRepository()

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

		if err := r.Dump(tmpFile, table); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "sample_ltsv", got)
	})
}

// readLTSVAsTable reads an LTSV file and returns a model.Table for testing Dump.
func readLTSVAsTable(t *testing.T, path string) *model.Table {
	t.Helper()

	f, err := os.Open(path) // #nosec G304 - test helper with controlled input
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

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
			idx := strings.Index(v, ":")
			if idx >= 0 {
				record = append(record, v[idx+1:])
			} else {
				record = append(record, v)
			}
		}
		records = append(records, record)
	}
	return model.NewTable(filepath.Base(path), header, records)
}
