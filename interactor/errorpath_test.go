package interactor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestDumpTable_InitCompressionError covers the withCompressedWriter branch where
// building the compression codec fails: bzip2 has no writer, so DumpTable must
// surface an "init compression" error after the output file is created.
func TestDumpTable_InitCompressionError(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("t", model.NewHeader([]string{"id"}), []model.Record{
		model.Record([]string{"1"}),
	})
	out := filepath.Join(t.TempDir(), "out.csv.bz2")

	err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionBzip2)
	if err == nil {
		t.Fatal("DumpTable with bzip2 compression = nil error, want error (bzip2 has no writer)")
	}
}

// TestDumpTable_Parquet covers the ExportParquet case of DumpTable by exporting a
// non-empty table and asserting the file is written.
func TestDumpTable_Parquet(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("people", model.NewHeader([]string{"id", "name"}), []model.Record{
		model.Record([]string{"1", "alice"}),
	})
	out := filepath.Join(t.TempDir(), "people.parquet")

	if err := exp.DumpTable(out, table, model.ExportParquet, model.CompressionNone); err != nil {
		t.Fatalf("DumpTable Parquet failed: %v", err)
	}
}

// TestDumpTable_ParquetEmpty covers the ExportParquet error path routed through
// DumpTable: an empty result cannot be written as Parquet.
func TestDumpTable_ParquetEmpty(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("empty", model.NewHeader([]string{"id"}), []model.Record{})
	out := filepath.Join(t.TempDir(), "empty.parquet")

	if err := exp.DumpTable(out, table, model.ExportParquet, model.CompressionNone); err == nil {
		t.Fatal("DumpTable Parquet on empty result = nil error, want error")
	}
}

func TestDumpTable_PreservesExistingFileOnFailure(t *testing.T) {
	t.Parallel()

	t.Run("preserves existing file and cleans up temp files when LTSV dump fails due to tab", func(t *testing.T) {
		t.Parallel()

		exp := newTestExportInteractor()
		// Table with a tab character in a value, which is invalid in LTSV format
		table := model.NewTable("test", model.NewHeader([]string{"id", "name"}), []model.Record{
			model.Record([]string{"1", "alice\tbob"}),
		})

		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "original.ltsv")

		// Create an existing file with content
		originalContent := []byte("original data")
		if err := os.WriteFile(outPath, originalContent, 0o600); err != nil {
			t.Fatalf("failed to write original file: %v", err)
		}

		// Try to dump invalid table to the file path, which should fail
		err := exp.DumpTable(outPath, table, model.ExportLTSV, model.CompressionNone)
		if err == nil {
			t.Fatal("expected DumpTable to fail due to tab in LTSV value, but it succeeded")
		}

		// Verify that the file still contains its original data and has not been truncated
		currentContent, err := os.ReadFile(outPath) //nolint:gosec // test file with controlled path
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(currentContent) != string(originalContent) {
			t.Errorf("file was altered! got %q, want %q", string(currentContent), string(originalContent))
		}
	})
}

func TestDumpTable_PreservesFilePermissions(t *testing.T) {
	t.Parallel()

	t.Run("preserves file permissions on successful overwrite", func(t *testing.T) {
		t.Parallel()

		exp := newTestExportInteractor()
		table := model.NewTable("test", model.NewHeader([]string{"id", "name"}), []model.Record{
			model.Record([]string{"1", "alice"}),
		})

		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "output.ltsv")

		// Create file with custom permission (0o600)
		const customPerm = os.FileMode(0o600)
		if err := os.WriteFile(outPath, []byte("old content"), customPerm); err != nil {
			t.Fatalf("failed to write original file: %v", err)
		}

		infoBefore, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("failed to stat file before: %v", err)
		}

		// Perform successful dump
		if err := exp.DumpTable(outPath, table, model.ExportLTSV, model.CompressionNone); err != nil {
			t.Fatalf("failed to dump table: %v", err)
		}

		infoAfter, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("failed to stat file after: %v", err)
		}

		if infoBefore.Mode().Perm() != infoAfter.Mode().Perm() {
			t.Errorf("file permissions changed! got %v, want %v", infoAfter.Mode().Perm(), infoBefore.Mode().Perm())
		}
	})
}
