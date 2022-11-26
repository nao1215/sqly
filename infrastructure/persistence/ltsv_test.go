package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/golden"
)

func Test_ltsvRepository_labelAndData(t *testing.T) {
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

func Test_ltsvRepository_List(t *testing.T) {
	t.Run("list and dump ltsv data", func(t *testing.T) {
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

		file := filepath.Join(t.TempDir(), "dump.ltsv")
		f2, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0664)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		if err := r.Dump(f2, ltsv.ToTable()); err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "sample_ltsv", got)
	})
}
