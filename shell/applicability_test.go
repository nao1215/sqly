package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOptionApplicability covers the rule that an import option the user typed
// must be able to apply to something. A flag that is accepted and then has no
// effect is indistinguishable from one that worked, which is exactly the failure
// a script cannot detect.
func TestOptionApplicability(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "rows.csv", "a,b\n1,2\n")
	tsv := writeCSV(t, dir, "rows.tsv", "a\tb\n1\t2\n")
	jsonl := writeCSV(t, dir, "docs.jsonl", "{\"a\":1}\n")
	parquet := writeCSV(t, dir, "data.parquet", "not really parquet")
	gzipped := writeCSV(t, dir, "rows.csv.gz", "unused")
	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "row-mismatch with a csv input is accepted",
			args: []string{"--row-mismatch", "skip", "--sql", "SELECT 1", csv},
		},
		{
			name: "row-mismatch with a tsv input is accepted",
			args: []string{"--row-mismatch", "skip", "--sql", "SELECT 1", tsv},
		},
		{
			name:    "row-mismatch with only a jsonl input is rejected",
			args:    []string{"--row-mismatch", "skip", "--sql", "SELECT 1", jsonl},
			wantErr: "--row-mismatch",
		},
		{
			name: "row-mismatch is accepted when one of several inputs is csv",
			args: []string{"--row-mismatch", "skip", "--sql", "SELECT 1", jsonl, csv},
		},
		{
			name: "row-mismatch is accepted for a compressed csv",
			args: []string{"--row-mismatch", "skip", "--sql", "SELECT 1", gzipped},
		},
		{
			name: "row-mismatch is accepted for a directory, whose contents are unknown",
			args: []string{"--row-mismatch", "skip", "--sql", "SELECT 1", sub},
		},
		{
			name: "encoding with a text input is accepted",
			args: []string{"--encoding", "shift-jis", "--sql", "SELECT 1", jsonl},
		},
		{
			name:    "encoding with only a parquet input is rejected",
			args:    []string{"--encoding", "shift-jis", "--sql", "SELECT 1", parquet},
			wantErr: "--encoding",
		},
		{
			name: "encoding is accepted for a remote input, whose format is unknown",
			args: []string{"--encoding", "shift-jis", "--sql", "SELECT 1", "https://example.test/data"},
		},
		{
			name: "an option left at its default is never rejected",
			args: []string{"--sql", "SELECT 1", parquet},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			shell, cleanup, err := newShell(t, append([]string{"sqly"}, tt.args...))
			if err != nil {
				t.Fatalf("newShell: %v", err)
			}
			defer cleanup()

			gotErr := shell.validateOptionApplicability()
			if tt.wantErr == "" {
				if gotErr != nil {
					t.Fatalf("validateOptionApplicability() = %v, want nil", gotErr)
				}
				return
			}
			if gotErr == nil {
				t.Fatal("validateOptionApplicability() = nil, want a rejection")
			}
			if !strings.Contains(gotErr.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to name %s", gotErr, tt.wantErr)
			}
		})
	}
}

// TestOptionApplicability_StdinDatasetCounts checks that the piped dataset is
// counted as an input. Its format is named by --stdin-format, so an option that
// applies to that format applies to the run even with no file arguments.
func TestOptionApplicability_StdinDatasetCounts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "row-mismatch applies to a piped csv dataset",
			args: []string{"--stdin-format", "csv", "--row-mismatch", "skip", "--sql", "SELECT 1"},
		},
		{
			name:    "row-mismatch does not apply to a piped jsonl dataset",
			args:    []string{"--stdin-format", "jsonl", "--row-mismatch", "skip", "--sql", "SELECT 1"},
			wantErr: true,
		},
		{
			name: "encoding applies to a piped jsonl dataset",
			args: []string{"--stdin-format", "jsonl", "--encoding", "euc-jp", "--sql", "SELECT 1"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			shell, cleanup, err := newShell(t, append([]string{"sqly"}, tt.args...))
			if err != nil {
				t.Fatalf("newShell: %v", err)
			}
			defer cleanup()

			gotErr := shell.validateOptionApplicability()
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("validateOptionApplicability() = %v, wantErr %v", gotErr, tt.wantErr)
			}
		})
	}
}

// TestOptionApplicability_RejectionHappensBeforeTheImport pins that the check
// runs early: a rejected run must not have read an input or written anything.
func TestOptionApplicability_RejectionHappensBeforeTheImport(t *testing.T) {
	dir := t.TempDir()
	jsonl := writeCSV(t, dir, "docs.jsonl", "{\"a\":1}\n")
	out := filepath.Join(dir, "out.csv")

	shell, cleanup, err := newShell(t, []string{"sqly", "--row-mismatch", "skip", "--output", out, "--sql", "SELECT 1", jsonl})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return false }
	silenceStderr(t)

	if runErr := shell.Run(context.Background()); runErr == nil {
		t.Fatal("Run succeeded although --row-mismatch applies to no input")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("the rejected run created its output file")
	}
	if extra := leftoverFiles(t, dir, "docs.jsonl"); len(extra) > 0 {
		t.Errorf("the rejected run left files behind: %v", extra)
	}
}
