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

	t.Run("--sql-file sets the SQL file path (#281)", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--sql-file", "query.sql", "testdata/user.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.SQLFilePath != "query.sql" {
			t.Errorf("SQLFilePath = %q, want %q", arg.SQLFilePath, "query.sql")
		}
		if diff := cmp.Diff([]string{"testdata/user.csv"}, arg.FilePaths); diff != "" {
			t.Errorf("FilePaths mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("sql file path defaults to empty (#281)", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "testdata/user.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.SQLFilePath != "" {
			t.Errorf("SQLFilePath = %q, want empty", arg.SQLFilePath)
		}
	})

	t.Run("invalid --stdin-name values are rejected (#305)", func(t *testing.T) {
		for _, name := range []string{"", ".", "..", "a/b", "../escaped", `a\b`} {
			if _, err := NewArg([]string{"sqly", "--stdin", "csv", "--stdin-name", name}); err == nil {
				t.Errorf("NewArg accepted invalid --stdin-name %q, want error", name)
			}
		}
	})

	t.Run("non-identifier --stdin-name values are rejected (#289)", func(t *testing.T) {
		// These would be sanitized by filesql, leaving the advertised name
		// unqueryable, so they are rejected up front.
		for _, name := range []string{"my data", "2023-data", "a-b", "weird!"} {
			if _, err := NewArg([]string{"sqly", "--stdin", "csv", "--stdin-name", name}); err == nil {
				t.Errorf("NewArg accepted non-identifier --stdin-name %q, want error", name)
			}
		}
	})

	t.Run("a normal --stdin-name is accepted (#305)", func(t *testing.T) {
		if _, err := NewArg([]string{"sqly", "--stdin", "csv", "--stdin-name", "people"}); err != nil {
			t.Errorf("NewArg rejected a valid --stdin-name: %v", err)
		}
	})

	t.Run("explicit empty --sheet is rejected (#313)", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sheet", "", "testdata/user.csv"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --sheet, got nil")
		}
	})

	t.Run("explicit empty --output is rejected (#349)", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql", "SELECT 1 AS x", "--output", ""})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --output, got nil")
		}
	})

	t.Run("explicit empty --sql-file is rejected (#350)", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql-file", ""})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --sql-file, got nil")
		}
	})

	t.Run("explicit empty --save-dir is rejected (#352)", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql", "SELECT 1", "--save-dir", "", "testdata/user.csv"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --save-dir, got nil")
		}
	})

	t.Run("explicit empty --stdin is rejected (#353)", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--stdin", "", "--sql", "SELECT 1 AS x"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --stdin, got nil")
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
