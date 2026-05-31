package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

func TestWritableExportTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     string
		wantOK     bool
		wantFormat model.ExportFormat
		wantComp   model.Compression
	}{
		{name: "csv is writable", source: "data.csv", wantOK: true, wantFormat: model.ExportCSV, wantComp: model.CompressionNone},
		{name: "tsv is writable", source: "data.tsv", wantOK: true, wantFormat: model.ExportTSV, wantComp: model.CompressionNone},
		{name: "ltsv is writable", source: "data.ltsv", wantOK: true, wantFormat: model.ExportLTSV, wantComp: model.CompressionNone},
		{name: "parquet is writable", source: "data.parquet", wantOK: true, wantFormat: model.ExportParquet, wantComp: model.CompressionNone},
		{name: "csv.gz keeps gzip", source: "data.csv.gz", wantOK: true, wantFormat: model.ExportCSV, wantComp: model.CompressionGzip},
		{name: "tsv.zst keeps zstd", source: "data.tsv.zst", wantOK: true, wantFormat: model.ExportTSV, wantComp: model.CompressionZstd},
		{name: "json is not writable", source: "data.json", wantOK: false},
		{name: "jsonl is not writable", source: "data.jsonl", wantOK: false},
		{name: "xlsx is not writable", source: "data.xlsx", wantOK: false},
		{name: "ach is not writable", source: "data.ach", wantOK: false},
		{name: "fed is not writable", source: "data.fed", wantOK: false},
		{name: "compressed parquet is not writable", source: "data.parquet.gz", wantOK: false},
		{name: "unknown extension is not writable", source: "data.bin", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			format, comp, ok := writableExportTarget(tt.source)
			if ok != tt.wantOK {
				t.Fatalf("writableExportTarget(%q) ok = %v, want %v", tt.source, ok, tt.wantOK)
			}
			if ok && (format != tt.wantFormat || comp != tt.wantComp) {
				t.Errorf("writableExportTarget(%q) = (%v, %v), want (%v, %v)", tt.source, format, comp, tt.wantFormat, tt.wantComp)
			}
		})
	}
}

func TestValidateSaveFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		save    bool
		saveDir string
		force   bool
		query   string
		tty     bool
		wantErr bool
	}{
		{name: "no save flags is allowed", wantErr: false},
		{name: "save with force and query is allowed", save: true, force: true, query: "SELECT 1", wantErr: false},
		{name: "save without force is rejected", save: true, query: "SELECT 1", wantErr: true},
		{name: "save and save-dir together is rejected", save: true, force: true, saveDir: "out", query: "SELECT 1", wantErr: true},
		{name: "save-dir on an interactive session is rejected", saveDir: "out", tty: true, wantErr: true},
		{name: "save-dir with query is allowed", saveDir: "out", query: "SELECT 1", wantErr: false},
		{name: "save-dir in batch (non-tty) is allowed", saveDir: "out", tty: false, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Shell{
				argument: &config.Arg{SaveInPlace: tt.save, SaveDir: tt.saveDir, Force: tt.force, Query: tt.query},
				isTTY:    func() bool { return tt.tty },
			}
			err := s.validateSaveFlags()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSaveFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteBack_SaveDirIsNonDestructive(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "people.csv", "name,age\nAlice,30\nBob,25\n")
	outDir := filepath.Join(dir, "out")

	runWithArgs(t, []string{"sqly", "--sql", "UPDATE people SET age = '99' WHERE name = 'Alice'", "--save-dir", outDir, src})

	orig, err := os.ReadFile(src) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(orig), "99") {
		t.Errorf("source file was modified by --save-dir:\n%s", orig)
	}

	saved, err := os.ReadFile(filepath.Join(outDir, "people.csv")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("saved file not written: %v", err)
	}
	if !strings.Contains(string(saved), "99") {
		t.Errorf("saved file missing the update:\n%s", saved)
	}
}

func TestWriteBack_SaveInPlaceWithForce(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "nums.csv", "id\n1\n2\n3\n")

	runWithArgs(t, []string{"sqly", "--sql", "DELETE FROM nums WHERE id > 1", "--save", "--force", src})

	got, err := os.ReadFile(src) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	// Header plus one remaining row; the deleted rows must be gone (O_TRUNC).
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 {
		t.Errorf("in-place save did not truncate; got %d lines:\n%s", len(lines), got)
	}
}

func TestWriteBack_UnsupportedSourceErrors(t *testing.T) {
	dir := t.TempDir()
	// JSON loads into a single data column and does not round-trip, so write-back
	// must refuse it rather than corrupt the file.
	jsonPath := filepath.Join(dir, "data.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"a":1}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "SELECT 1", "--save", "--force", jsonPath})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if err := shell.Run(context.Background()); err == nil {
		t.Fatal("expected an error saving back to a JSON source, got nil")
	}
}

// runWithArgs builds a shell from args and runs it, failing the test on error.
func runWithArgs(t *testing.T, args []string) {
	t.Helper()
	shell, cleanup, err := newShell(t, args)
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()
	_ = captureStdout(t, func() {
		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}
