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

	t.Run("user set --json-typed option selects json mode with the typed contract", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--json-typed"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.Output.Mode != model.PrintModeJSON {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeJSON)
		}
		if !arg.Output.JSONTyped {
			t.Error("expected Output.JSONTyped to be true for --json-typed")
		}
	})

	t.Run("user set --ndjson-typed option selects ndjson mode with the typed contract", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--ndjson-typed"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.Output.Mode != model.PrintModeNDJSON {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeNDJSON)
		}
		if !arg.Output.JSONTyped {
			t.Error("expected Output.JSONTyped to be true for --ndjson-typed")
		}
	})

	t.Run("--json and --json-typed together are rejected as conflicting", func(t *testing.T) {
		if _, err := NewArg([]string{"sqly", "--json", "--json-typed"}); err == nil {
			t.Error("expected conflicting output mode flags error, got nil")
		}
	})

	t.Run("--compare sets the flag and accepts its sub-flags", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--compare", "--compare-key", "id", "--compare-tables", "a,b", "--compare-format", "text", "a.csv", "b.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if !arg.CompareFlag {
			t.Error("expected CompareFlag to be true")
		}
		if arg.CompareKey != "id" || arg.CompareTables != "a,b" || arg.CompareFormat != "text" {
			t.Errorf("compare flags = %q/%q/%q, want id/a,b/text", arg.CompareKey, arg.CompareTables, arg.CompareFormat)
		}
	})

	t.Run("compare sub-flags without --compare are rejected", func(t *testing.T) {
		for _, a := range [][]string{
			{"sqly", "--compare-key", "id"},
			{"sqly", "--compare-tables", "a,b"},
			{"sqly", "--compare-format", "text"},
		} {
			if _, err := NewArg(a); err == nil {
				t.Errorf("expected error for %v without --compare", a)
			}
		}
	})

	t.Run("an invalid --compare-format is rejected", func(t *testing.T) {
		if _, err := NewArg([]string{"sqly", "--compare", "--compare-format", "yaml"}); err == nil {
			t.Error("expected an error for an invalid --compare-format")
		}
	})

	t.Run("--compare-format defaults to json", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--compare", "a.csv", "b.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.CompareFormat != "json" {
			t.Errorf("CompareFormat = %q, want json", arg.CompareFormat)
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

	t.Run("--output after file path sets output destination, not an import path", func(t *testing.T) {
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

	t.Run("output-mode flag after file path sets mode, not an import path", func(t *testing.T) {
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

	t.Run("flags interspersed among file paths are not imported as paths", func(t *testing.T) {
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

	t.Run("unknown flag after file path returns a parse error", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "testdata/user.csv", "--nope"})
		if err == nil {
			t.Fatal("expected a parse error for an unknown flag, got nil")
		}
	})

	t.Run("--sql-file sets the SQL file path", func(t *testing.T) {
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

	t.Run("sql file path defaults to empty", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "testdata/user.csv"})
		if err != nil {
			t.Fatal(err)
		}
		if arg.SQLFilePath != "" {
			t.Errorf("SQLFilePath = %q, want empty", arg.SQLFilePath)
		}
	})

	t.Run("invalid --stdin-name values are rejected", func(t *testing.T) {
		for _, name := range []string{"", ".", "..", "a/b", "../escaped", `a\b`} {
			if _, err := NewArg([]string{"sqly", "--stdin", "csv", "--stdin-name", name}); err == nil {
				t.Errorf("NewArg accepted invalid --stdin-name %q, want error", name)
			}
		}
	})

	t.Run("non-identifier --stdin-name values are rejected", func(t *testing.T) {
		// These would be sanitized by filesql, leaving the advertised name
		// unqueryable, so they are rejected up front.
		for _, name := range []string{"my data", "2023-data", "a-b", "weird!"} {
			if _, err := NewArg([]string{"sqly", "--stdin", "csv", "--stdin-name", name}); err == nil {
				t.Errorf("NewArg accepted non-identifier --stdin-name %q, want error", name)
			}
		}
	})

	t.Run("a normal --stdin-name is accepted", func(t *testing.T) {
		if _, err := NewArg([]string{"sqly", "--stdin", "csv", "--stdin-name", "people"}); err != nil {
			t.Errorf("NewArg rejected a valid --stdin-name: %v", err)
		}
	})

	t.Run("explicit empty --sheet is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sheet", "", "testdata/user.csv"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --sheet, got nil")
		}
	})

	t.Run("explicit empty --output is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql", "SELECT 1 AS x", "--output", ""})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --output, got nil")
		}
	})

	t.Run("explicit empty --sql-file is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql-file", ""})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --sql-file, got nil")
		}
	})

	t.Run("explicit empty --save-dir is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql", "SELECT 1", "--save-dir", "", "testdata/user.csv"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --save-dir, got nil")
		}
	})

	t.Run("explicit empty --stdin is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--stdin", "", "--sql", "SELECT 1 AS x"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --stdin, got nil")
		}
	})

	t.Run("conflicting output mode flags are rejected", func(t *testing.T) {
		for _, args := range [][]string{
			{"sqly", "--csv", "--json", "--sql", "SELECT 1 AS x"},
			{"sqly", "--tsv", "--json", "--sql", "SELECT 1 AS x"},
			{"sqly", "--csv", "--tsv", "--ltsv"},
		} {
			if _, err := NewArg(args); err == nil {
				t.Errorf("NewArg(%v) = nil error, want a conflict error", args[1:])
			}
		}
	})

	t.Run("a single output mode flag is accepted", func(t *testing.T) {
		if _, err := NewArg([]string{"sqly", "--json", "--sql", "SELECT 1 AS x"}); err != nil {
			t.Errorf("NewArg with a single output mode flag returned an error: %v", err)
		}
	})

	t.Run("--inspect sets the inspect flag", func(t *testing.T) {
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

	t.Run("inspect flag defaults to false", func(t *testing.T) {
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

// TestNewArgDependentFlagValidation covers the v0.19.0 flag-dependency bugs:
// --stdin-name without --stdin, --inspect-sample without --inspect,
// --force without --save/--save-dir, and a SQLite-keyword --stdin-name
func TestNewArgDependentFlagValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name:    "stdin-name without stdin is rejected",
			args:    []string{"sqly", "--stdin-name", "weird", "--sql", "SELECT 1"},
			wantErr: errStdinNameWithoutStdin,
		},
		{
			name:    "inspect-sample without inspect is rejected",
			args:    []string{"sqly", "--inspect-sample", "0", "--sql", "SELECT 1"},
			wantErr: errInspectSampleWithoutInspect,
		},
		{
			name:    "negative inspect-sample without inspect is rejected",
			args:    []string{"sqly", "--inspect-sample", "-1", "--sql", "SELECT 1"},
			wantErr: errInspectSampleWithoutInspect,
		},
		{
			name:    "force without save is rejected",
			args:    []string{"sqly", "--force", "--sql", "SELECT 1"},
			wantErr: errForceWithoutSave,
		},
		{
			name:    "stdin-name that is a SQLite keyword is rejected",
			args:    []string{"sqly", "--stdin", "csv", "--stdin-name", "select", "--sql", "SELECT 1"},
			wantErr: errStdinNameReserved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewArg(tt.args)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewArg(%v) error = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
	}

	// Sanity: the dependent flags are accepted when their parent flag is present.
	t.Run("dependent flags accepted with their parent flag", func(t *testing.T) {
		t.Parallel()
		ok := [][]string{
			{"sqly", "--stdin", "csv", "--stdin-name", "data", "--sql", "SELECT 1"},
			{"sqly", "--inspect", "--inspect-sample", "0"},
			{"sqly", "--save", "--force", "--sql", "SELECT 1"},
		}
		for _, args := range ok {
			if _, err := NewArg(args); err != nil {
				t.Errorf("NewArg(%v) unexpected error: %v", args, err)
			}
		}
	})
}
