// Package config manage sqly configuration
package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gorky/golden"
	"github.com/nao1215/sqly/domain/model"
)

func TestNewArg(t *testing.T) {
	t.Run("user want to output result to file", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "output.txt")
		arg, err := NewArg([]string{"sqly", "--sql", "SELECT * FROM test", "-o", testFile, "testdata/no_exist.csv"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.FilePath != testFile {
			t.Errorf("mismatch got=%s, want=%s", arg.Output.FilePath, testFile)
		}

		want2 := true
		if arg.NeedsOutputToFile() != want2 {
			t.Errorf("mismatch got=%v, want=%v", arg.NeedsOutputToFile(), want2)
		}
	})

	t.Run("user set --csv option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--csv"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeCSV {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeCSV)
		}
	})

	t.Run("user set --json option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--json"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeJSON {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeJSON)
		}
	})

	t.Run("user set --tsv option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--tsv"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeTSV {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeTSV)
		}
	})

	t.Run("user set --ltsv option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--ltsv"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeLTSV {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeLTSV)
		}
	})

	t.Run("user set --markdown option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--markdown"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeMarkdownTable {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeMarkdownTable)
		}
	})

	t.Run("default print mode", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeTable {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeTable)
		}
	})

	t.Run("no argument", func(t *testing.T) {
		_, got := NewArg([]string{})
		if got == nil {
			t.Fatal("expect error happen, however NewArg() return nil")
		}

		if !errors.Is(got, ErrEmptyArg) {
			t.Errorf("mismatch got=%v, want=%v", got, ErrEmptyArg)
		}
	})
}

func TestUsage(t *testing.T) {
	t.Run("check usage contents", func(t *testing.T) {
		Version = "test-version"
		arg, err := NewArg([]string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		g := golden.New(t)
		g.Assert(t, "usage", []byte(arg.Usage))
	})
}

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "get the version embedded by ldflags",
			version: "test-ver",
			want:    "test-ver",
		},
		{
			name:    "not set version",
			version: "",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			if got := GetVersion(); got != tt.want {
				t.Errorf("GetVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_version(t *testing.T) {
	t.Run("get version with escape sequence", func(t *testing.T) {
		Version = "test-version"
		got := getStdout(t, version)

		want := "sqly test-version"
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}

func getStdout(t *testing.T, f func()) string {
	t.Helper()
	backupColorStdout := Stdout
	defer func() {
		Stdout = backupColorStdout
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	Stdout = w

	f()
	w.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}

	s := buffer.String()
	return s[:len(s)-1]
}
