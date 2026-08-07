package interactor

import (
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestSQLite3InteractorMalformedRowPolicy verifies that a policy set with
// SetRowMismatchPolicy is reported back by RowMismatchPolicy through the
// filesql adapter.
func TestSQLite3InteractorMalformedRowPolicy(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	policies := []model.RowMismatchPolicy{
		model.RowMismatchError,
		model.RowMismatchSkip,
		model.RowMismatchPad,
	}
	for _, want := range policies {
		si.SetRowMismatchPolicy(want)
		if got := si.RowMismatchPolicy(); got != want {
			t.Errorf("RowMismatchPolicy() = %v, want %v", got, want)
		}
	}
}

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
		if err := exp.DumpTable(badPath, table, model.ExportCSV, model.CompressionNone); err == nil {
			t.Fatal("DumpTable() = nil error, want error when output directory does not exist")
		}
	})

	t.Run("compression init failure for write-only-unsupported codec", func(t *testing.T) {
		t.Parallel()
		out := filepath.Join(t.TempDir(), "out.csv.bz2")
		if err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionBzip2); err == nil {
			t.Fatal("DumpTable() = nil error, want error when compression codec rejects writing")
		}
	})

	t.Run("write failure from duplicate JSON columns", func(t *testing.T) {
		t.Parallel()
		dup := model.NewTable("t", model.Header{"a", "a"}, []model.Record{
			model.Record([]string{"1", "2"}),
		})
		out := filepath.Join(t.TempDir(), "out.json")
		if err := exp.DumpTable(out, dup, model.ExportJSON, model.CompressionNone); err == nil {
			t.Fatal("DumpTable() = nil error, want error when JSON columns are not unique")
		}
	})
}
