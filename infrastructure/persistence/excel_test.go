package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/domain/model"
	"github.com/xuri/excelize/v2"
)

func TestExcelRepositoryDump(t *testing.T) {
	t.Parallel()

	t.Run("dump excel data and verify round-trip", func(t *testing.T) {
		t.Parallel()

		r := NewExcelRepository()

		table := model.NewTable(
			"test_sheet",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
				{"3", "Vika"},
			},
		)
		tempFilePath := filepath.Join(os.TempDir(), "dump.xlsx")
		defer os.Remove(tempFilePath)
		if err := r.Dump(tempFilePath, table); err != nil {
			t.Fatal(err)
		}

		// Verify by reading the dumped file directly with excelize
		f, err := excelize.OpenFile(tempFilePath)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		rows, err := f.GetRows("test_sheet")
		if err != nil {
			t.Fatal(err)
		}

		wantRows := [][]string{
			{"id", "name"},
			{"1", "Gina"},
			{"2", "Yulia"},
			{"3", "Vika"},
		}
		if diff := cmp.Diff(rows, wantRows); diff != "" {
			t.Fatalf("differs: (-got +want)\n%s", diff)
		}
	})
}
