package config

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/golden"
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

	t.Run("user set --excel option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--excel"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeExcel {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeExcel)
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

	t.Run("user set --excel option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--excel"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeExcel {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeExcel)
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

	t.Run("user set --ndjson option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--ndjson"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeNDJSON {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeNDJSON)
		}
	})

	t.Run("user set --parquet option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--parquet"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeParquet {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeParquet)
		}
	})

	t.Run("user set --stdin and --stdin-name options", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--stdin", "csv", "--stdin-name", "piped"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.StdinFormat != "csv" {
			t.Errorf("StdinFormat = %q, want csv", arg.StdinFormat)
		}
		if arg.StdinTableName != "piped" {
			t.Errorf("StdinTableName = %q, want piped", arg.StdinTableName)
		}
	})

	t.Run("stdin table name defaults to stdin", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--stdin", "csv"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.StdinTableName != "stdin" {
			t.Errorf("StdinTableName = %q, want stdin", arg.StdinTableName)
		}
	})

	t.Run("--output after file path sets output destination, not an import path (#264)", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "result.csv")
		arg, err := NewArg([]string{"sqly", "--sql", "SELECT * FROM user", "testdata/user.csv", "--output", testFile})
		if err != nil {
			t.Fatal(err)
		}
		if arg.Output.FilePath != testFile {
			t.Errorf("Output.FilePath = %q, want %q", arg.Output.FilePath, testFile)
		}
		if diff := cmp.Diff([]string{"testdata/user.csv"}, arg.FilePaths); diff != "" {
			t.Errorf("FilePaths mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("output-mode flag after file path sets mode, not an import path (#264)", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--sql", "SELECT * FROM user", "testdata/user.csv", "--csv"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.Output.Mode != model.PrintModeCSV {
			t.Errorf("Output.Mode = %v, want %v", arg.Output.Mode, model.PrintModeCSV)
		}
		if diff := cmp.Diff([]string{"testdata/user.csv"}, arg.FilePaths); diff != "" {
			t.Errorf("FilePaths mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("flags interspersed among file paths are not imported as paths (#264)", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "testdata/user.csv", "--json", "testdata/identifier.csv", "--sql", "SELECT 1"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.Output.Mode != model.PrintModeJSON {
			t.Errorf("Output.Mode = %v, want %v", arg.Output.Mode, model.PrintModeJSON)
		}
		if arg.Query != "SELECT 1" {
			t.Errorf("Query = %q, want %q", arg.Query, "SELECT 1")
		}
		if diff := cmp.Diff([]string{"testdata/user.csv", "testdata/identifier.csv"}, arg.FilePaths); diff != "" {
			t.Errorf("FilePaths mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("unknown flag after file path returns a parse error (#264)", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "testdata/user.csv", "--nope"})
		if err == nil {
			t.Fatal("expected a parse error for an unknown flag, got nil")
		}
	})

	t.Run("--inspect sets the inspect flag (#259)", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--inspect", "testdata/user.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if !arg.InspectFlag {
			t.Errorf("InspectFlag = false, want true")
		}
		if diff := cmp.Diff([]string{"testdata/user.csv"}, arg.FilePaths); diff != "" {
			t.Errorf("FilePaths mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("inspect flag defaults to false (#259)", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "testdata/user.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.InspectFlag {
			t.Errorf("InspectFlag = true, want false")
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
			want:    "(devel)",
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

func TestVersion(t *testing.T) {
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

	var buffer bytes.Buffer
	Stdout = &buffer

	f()

	s := buffer.String()
	return s[:len(s)-1]
}
