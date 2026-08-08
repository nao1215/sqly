package interactor

import (
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestWithCompressedWriterErrors covers the error branches of withCompressedWriter:
// a failing file creation, a rejected compression codec, and a write function that
// returns an error.
func TestWithCompressedWriterErrors(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("t", model.Header{"a"}, []model.Record{
		model.Record([]string{"1"}),
	})

	t.Run("create failure on nonexistent directory", func(t *testing.T) {
		t.Parallel()
		badPath := filepath.Join(t.TempDir(), "no-such-dir", "out.csv")
		if err := exp.DumpTable(badPath, table, model.ExportCSV, model.CompressionNone, model.TextEncodingUTF8); err == nil {
			t.Fatal("DumpTable() = nil error, want error when output directory does not exist")
		}
	})

	t.Run("compression init failure for write-only-unsupported codec", func(t *testing.T) {
		t.Parallel()
		out := filepath.Join(t.TempDir(), "out.csv.bz2")
		if err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionBzip2, model.TextEncodingUTF8); err == nil {
			t.Fatal("DumpTable() = nil error, want error when compression codec rejects writing")
		}
	})

	t.Run("write failure from duplicate JSON columns", func(t *testing.T) {
		t.Parallel()
		dup := model.NewTable("t", model.Header{"a", "a"}, []model.Record{
			model.Record([]string{"1", "2"}),
		})
		out := filepath.Join(t.TempDir(), "out.json")
		if err := exp.DumpTable(out, dup, model.ExportJSON, model.CompressionNone, model.TextEncodingUTF8); err == nil {
			t.Fatal("DumpTable() = nil error, want error when JSON columns are not unique")
		}
	})
}
