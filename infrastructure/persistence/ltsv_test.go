package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/sqly/config"
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
}

func TestLtsvRepositoryList(t *testing.T) {
	t.Parallel()

	t.Run("list and dump ltsv data", func(t *testing.T) {
		t.Parallel()

		r := NewLTSVRepository()
		f, err := os.Open(filepath.Join("testdata", "sample.ltsv"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		ltsv, err := r.List(f)
		if err != nil {
			t.Fatal(err)
		}

		var tmpFile *os.File
		var e error
		if runtime.GOOS != config.Windows {
			tmpFile, e = os.CreateTemp(t.TempDir(), "dump.ltsv")
		} else {
			// See https://github.com/golang/go/issues/51442
			tmpFile, e = os.CreateTemp(os.TempDir(), "dump.ltsv")
		}
		if e != nil {
			t.Fatal(err)
		}

		if err := r.Dump(tmpFile, ltsv.ToTable()); err != nil {
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

	t.Run("failed to get label data", func(t *testing.T) {
		t.Parallel()

		r := NewLTSVRepository()
		f, err := os.Open(filepath.Join("testdata", "sample_bad_label.ltsv"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = r.List(f)
		if !errors.Is(err, infrastructure.ErrNoLabel) {
			t.Errorf("error is not ErrNoLabel: %v", err)
		}
	})
}
