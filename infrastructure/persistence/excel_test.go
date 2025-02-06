package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/domain/model"
)

func TestExcelRepositoryList(t *testing.T) {
	t.Run("list excel data", func(t *testing.T) {
		r := NewExcelRepository()

		excel, err := r.List("testdata/sample.xlsx", "test_sheet")
		if err != nil {
			t.Fatal(err)
		}

		want := model.NewExcel(
			"test_sheet",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
				{"3", "Vika"},
			},
		)
		if diff := cmp.Diff(excel, want); diff != "" {
			t.Fatalf("differs: (-got +want)\n%s", diff)
		}
	})
}

func TestExcelRepositoryDump(t *testing.T) {
	t.Run("dump excel data", func(t *testing.T) {
		r := NewExcelRepository()

		excel := model.NewTable(
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
		if err := r.Dump(tempFilePath, excel); err != nil {
			t.Fatal(err)
		}

		got, err := r.List(tempFilePath, "test_sheet")
		if err != nil {
			t.Fatal(err)
		}
		want := model.NewExcel(
			"test_sheet",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
				{"3", "Vika"},
			},
		)
		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf("differs: (-got +want)\n%s", diff)
		}
	})
}
