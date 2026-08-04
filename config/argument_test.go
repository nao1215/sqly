package config

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/testutil"
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

	t.Run("user set --output-format csv option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "csv"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeCSV {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeCSV)
		}
	})

	t.Run("user set --output-format excel option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "excel"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeExcel {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeExcel)
		}
	})

	t.Run("user set --output-format tsv option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "tsv"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeTSV {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeTSV)
		}
	})

	t.Run("user set --output-format ltsv option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "ltsv"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeLTSV {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeLTSV)
		}
	})

	t.Run("user set --output-format markdown option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "markdown"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeMarkdownTable {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeMarkdownTable)
		}
	})

	t.Run("user set --output-format excel option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "excel"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeExcel {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeExcel)
		}
	})

	t.Run("user set --output-format json option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "json"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeJSON {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeJSON)
		}
	})

	t.Run("user set --output-format jsonl option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "jsonl"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeJSONL {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeJSONL)
		}
	})

	t.Run("user set --output-format parquet option", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--output-format", "parquet"})
		if err != nil {
			t.Fatal(err)
		}

		if arg.Output.Mode != model.PrintModeParquet {
			t.Errorf("mismatch got=%v, want=%v", arg.Output.Mode, model.PrintModeParquet)
		}
	})

	t.Run("user set --stdin-format and --stdin-table options", func(t *testing.T) {
		arg, err := NewArg([]string{"sqly", "--stdin-format", "csv", "--stdin-table", "piped"})
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
		arg, err := NewArg([]string{"sqly", "--stdin-format", "csv"})
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
		arg, err := NewArg([]string{"sqly", "--sql", "SELECT * FROM user", "testdata/user.csv", "--output-format", "csv"})
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
		arg, err := NewArg([]string{"sqly", "testdata/user.csv", "--output-format", "json", "testdata/identifier.csv", "--sql", "SELECT 1"})
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

	t.Run("invalid --stdin-table values are rejected", func(t *testing.T) {
		for _, name := range []string{"", ".", "..", "a/b", "../escaped", `a\b`} {
			if _, err := NewArg([]string{"sqly", "--stdin-format", "csv", "--stdin-table", name}); err == nil {
				t.Errorf("NewArg accepted invalid --stdin-table %q, want error", name)
			}
		}
	})

	t.Run("non-identifier --stdin-table values are rejected", func(t *testing.T) {
		// These would be sanitized by filesql, leaving the advertised name
		// unqueryable, so they are rejected up front.
		for _, name := range []string{"my data", "2023-data", "a-b", "weird!"} {
			if _, err := NewArg([]string{"sqly", "--stdin-format", "csv", "--stdin-table", name}); err == nil {
				t.Errorf("NewArg accepted non-identifier --stdin-table %q, want error", name)
			}
		}
	})

	t.Run("a normal --stdin-table is accepted", func(t *testing.T) {
		if _, err := NewArg([]string{"sqly", "--stdin-format", "csv", "--stdin-table", "people"}); err != nil {
			t.Errorf("NewArg rejected a valid --stdin-table: %v", err)
		}
	})

	t.Run("explicit empty --sheet is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sheet", "", "testdata/user.csv"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --sheet, got nil")
		}
	})

	t.Run("explicit empty --sql is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql", "", "testdata/user.csv"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --sql, got nil")
		}
		if !errors.Is(err, errEmptyQuery) {
			t.Errorf("mismatch error got=%v, want=%v", err, errEmptyQuery)
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

	t.Run("explicit empty --save-tables is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--sql", "SELECT 1", "--save-tables", "", "testdata/user.csv"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --save-tables, got nil")
		}
	})

	t.Run("explicit empty --stdin-format is rejected", func(t *testing.T) {
		_, err := NewArg([]string{"sqly", "--stdin-format", "", "--sql", "SELECT 1 AS x"})
		if err == nil {
			t.Fatal("expected an error for an explicit empty --stdin, got nil")
		}
	})

	t.Run("invalid output format is rejected", func(t *testing.T) {
		if _, err := NewArg([]string{"sqly", "--output-format", "yaml"}); err == nil {
			t.Fatal("NewArg accepted unsupported output format yaml")
		}
	})

	t.Run("legacy individual output flags are rejected", func(t *testing.T) {
		legacyFlags := []string{
			"--csv", "--tsv", "--ltsv", "--json", "--ndjson", "--excel", "--markdown", "--parquet", "--vertical",
			"-c", "-t", "-l", "-j", "-n", "-e", "-m", "-p",
		}
		for _, legacyFlag := range legacyFlags {
			t.Run(legacyFlag, func(t *testing.T) {
				_, err := NewArg([]string{"sqly", legacyFlag})
				if err == nil {
					t.Fatalf("NewArg accepted removed output flag %s", legacyFlag)
				}
				if !strings.Contains(err.Error(), "unknown") {
					t.Errorf("error for %s = %v, want unknown flag error", legacyFlag, err)
				}
			})
		}

		arg, err := NewArg([]string{"sqly", "--output-format", "csv"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(arg.Usage, "--output-format FORMAT") {
			t.Errorf("usage does not explain the replacement --output-format option")
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

	t.Run("parse and validation errors are tagged as *ArgError so the caller can tell a bad invocation from a shell-start failure", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			args []string
		}{
			{"empty argument", []string{}},
			{"unknown flag", []string{"sqly", "--no-such-flag"}},
			{"invalid output format", []string{"sqly", "--output-format", "yaml"}},
			{"explicit empty --sheet value", []string{"sqly", "--sheet", ""}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := NewArg(tc.args)
				if err == nil {
					t.Fatalf("NewArg(%v) returned nil error, want an error", tc.args)
				}
				var argErr *ArgError
				if !errors.As(err, &argErr) {
					t.Errorf("NewArg(%v) error is not *ArgError: %v", tc.args, err)
				}
			})
		}
	})

	t.Run("--row-mismatch defaults to error and parses error/skip/pad", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			args []string
			want model.RowMismatchPolicy
		}{
			{[]string{"sqly"}, model.RowMismatchError},
			{[]string{"sqly", "--row-mismatch", "error"}, model.RowMismatchError},
			{[]string{"sqly", "--row-mismatch", "skip"}, model.RowMismatchSkip},
			{[]string{"sqly", "--row-mismatch", "pad"}, model.RowMismatchPad},
		}
		for _, tc := range cases {
			arg, err := NewArg(tc.args)
			if err != nil {
				t.Fatalf("NewArg(%v) unexpected error: %v", tc.args, err)
			}
			if arg.RowMismatch != tc.want {
				t.Errorf("NewArg(%v).RowMismatch = %v, want %v", tc.args, arg.RowMismatch, tc.want)
			}
		}
	})

	t.Run("an invalid --row-mismatch is rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := NewArg([]string{"sqly", "--row-mismatch", "keep"}); err == nil {
			t.Fatal("NewArg with --row-mismatch keep returned nil error, want an error")
		}
	})

	t.Run("--encoding defaults to utf-8 and parses known aliases", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			args []string
			want model.TextEncoding
		}{
			{[]string{"sqly"}, model.TextEncodingUTF8},
			{[]string{"sqly", "--encoding", "utf-8"}, model.TextEncodingUTF8},
			{[]string{"sqly", "--encoding", "cp932"}, model.TextEncodingShiftJIS},
			{[]string{"sqly", "--encoding", "euc-jp"}, model.TextEncodingEUCJP},
			{[]string{"sqly", "--encoding", "iso-2022-jp"}, model.TextEncodingISO2022JP},
			{[]string{"sqly", "--encoding", "utf-16le"}, model.TextEncodingUTF16LE},
		}
		for _, tc := range cases {
			arg, err := NewArg(tc.args)
			if err != nil {
				t.Fatalf("NewArg(%v) unexpected error: %v", tc.args, err)
			}
			if arg.Encoding != tc.want {
				t.Errorf("NewArg(%v).Encoding = %v, want %v", tc.args, arg.Encoding, tc.want)
			}
		}
	})

	t.Run("an invalid --encoding is rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := NewArg([]string{"sqly", "--encoding", "latin1"}); err == nil {
			t.Fatal("NewArg with --encoding latin1 returned nil error, want an error")
		}
	})

	t.Run("--dialect defaults to sqlite and parses names and aliases", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			args []string
			want dialect.Dialect
		}{
			{[]string{"sqly"}, dialect.SQLite},
			{[]string{"sqly", "--dialect", "sqlite"}, dialect.SQLite},
			{[]string{"sqly", "--dialect", "mysql"}, dialect.MySQL},
			{[]string{"sqly", "--dialect", "postgresql"}, dialect.PostgreSQL},
			{[]string{"sqly", "--dialect", "postgres"}, dialect.PostgreSQL},
			{[]string{"sqly", "--dialect", "bigquery"}, dialect.GoogleSQL},
		}
		for _, tc := range cases {
			arg, err := NewArg(tc.args)
			if err != nil {
				t.Fatalf("NewArg(%v) unexpected error: %v", tc.args, err)
			}
			if arg.Dialect != tc.want {
				t.Errorf("NewArg(%v).Dialect = %q, want %q", tc.args, arg.Dialect, tc.want)
			}
		}
	})

	t.Run("an invalid --dialect is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := NewArg([]string{"sqly", "--dialect", "oracle"})
		if err == nil {
			t.Fatal("NewArg with --dialect oracle returned nil error, want an error")
		}
		if !strings.Contains(err.Error(), "unknown SQL dialect") {
			t.Fatalf("error = %v, want it to mention the unknown dialect", err)
		}
	})
}

func TestNewArgOutputFormatChoices(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mode model.PrintMode
	}{
		{"table", model.PrintModeTable},
		{"csv", model.PrintModeCSV},
		{"tsv", model.PrintModeTSV},
		{"ltsv", model.PrintModeLTSV},
		{"excel", model.PrintModeExcel},
		{"markdown", model.PrintModeMarkdownTable},
		{"json", model.PrintModeJSON},
		{"jsonl", model.PrintModeJSONL},
		{"parquet", model.PrintModeParquet},
		{"vertical", model.PrintModeVertical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			arg, err := NewArg([]string{"sqly", "--output-format", tc.name})
			if err != nil {
				t.Fatal(err)
			}
			if arg.Output.Mode != tc.mode {
				t.Errorf("output mode = %v, want %v", arg.Output.Mode, tc.mode)
			}
		})
	}
}

func TestUsage(t *testing.T) {
	t.Run("check usage contents", func(t *testing.T) {
		Version = "test-version"
		arg, err := NewArg([]string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		testutil.AssertFileEquals(t, filepath.Join("testdata", "usage.golden"), []byte(arg.Usage))
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

// TestNewArgDependentFlagValidation covers the flag dependencies the parser
// enforces: --stdin-table without --stdin-format, --inspect-sample without
// --inspect, the two write-back destinations together, and a SQLite-keyword
// --stdin-table.
func TestNewArgDependentFlagValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name:    "stdin-table without stdin-format is rejected",
			args:    []string{"sqly", "--stdin-table", "weird", "--sql", "SELECT 1"},
			wantErr: errStdinTableWithoutFormat,
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
			name:    "stdin-table that is a SQLite keyword is rejected",
			args:    []string{"sqly", "--stdin-format", "csv", "--stdin-table", "select", "--sql", "SELECT 1"},
			wantErr: errStdinTableReserved,
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
			{"sqly", "--stdin-format", "csv", "--stdin-table", "data", "--sql", "SELECT 1"},
			{"sqly", "--inspect", "--inspect-sample", "0"},
		}
		for _, args := range ok {
			if _, err := NewArg(args); err != nil {
				t.Errorf("NewArg(%v) unexpected error: %v", args, err)
			}
		}
	})
}

// TestNewArg_NormalizesStdinFormat pins the fix for a value that passed
// validation and then failed anyway. --stdin-format is validated trimmed and
// lowercased, so " CSV " is accepted; storing the raw string meant the staging
// step looked it up in a map keyed by the canonical names, missed, and failed
// mid-run. What is validated is what is stored.
func TestNewArg_NormalizesStdinFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		given string
		want  string
	}{
		{name: "an already canonical value is unchanged", given: "csv", want: "csv"},
		{name: "surrounding whitespace is trimmed", given: " csv ", want: "csv"},
		{name: "an uppercase value is lowered", given: "CSV", want: "csv"},
		{name: "both at once", given: "  JSONL\t", want: "jsonl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			arg, err := NewArg([]string{"sqly", "--stdin-format", tt.given, "--sql", "SELECT 1"})
			if err != nil {
				t.Fatalf("NewArg(--stdin-format %q): %v", tt.given, err)
			}
			if arg.StdinFormat != tt.want {
				t.Errorf("StdinFormat = %q, want %q", arg.StdinFormat, tt.want)
			}
		})
	}
}

// TestNewArg_RejectsAnUnknownStdinFormat is the other half: a value that is not
// one of the five is refused while parsing, so it exits as a usage error rather
// than after the run has started.
func TestNewArg_RejectsAnUnknownStdinFormat(t *testing.T) {
	t.Parallel()

	_, err := NewArg([]string{"sqly", "--stdin-format", "xml", "--sql", "SELECT 1"})
	if err == nil {
		t.Fatal("an unknown --stdin-format value was accepted")
	}
	var argErr *ArgError
	if !errors.As(err, &argErr) {
		t.Errorf("error is %T, want a *ArgError so it exits as a usage error", err)
	}
	if !strings.Contains(err.Error(), "want csv, tsv, ltsv, json, jsonl") {
		t.Errorf("error should list the values that exist, got: %v", err)
	}
}
