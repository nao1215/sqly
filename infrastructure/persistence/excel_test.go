package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/domain/model"
)

func Test_excelRepository_List(t *testing.T) {
	t.Run("list excel data", func(t *testing.T) {
		r := NewExcelRepository()

		excel, err := r.List("testdata/sample.xlsx", "test_sheet")
		if err != nil {
			t.Fatal(err)
		}

		want := &model.Excel{
			Name:   "test_sheet",
			Header: model.Header{"id", "name"},
			Records: []model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
				{"3", "Vika"},
			},
		}

		if diff := cmp.Diff(excel, want); diff != "" {
			t.Fatalf("differs: (-got +want)\n%s", diff)
		}
	})
}

func Test_excelRepository_Dump(t *testing.T) {
	t.Run("dump excel data", func(t *testing.T) {
		r := NewExcelRepository()

		excel := &model.Table{
			Name:   "test_sheet",
			Header: model.Header{"id", "name"},
			Records: []model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
				{"3", "Vika"},
				{"4", "Hartlova"},
			},
		}

		tempFilePath := filepath.Join(os.TempDir(), "dump.xlsx")
		defer os.Remove(tempFilePath) //nolint
		if err := r.Dump(tempFilePath, excel); err != nil {
			t.Fatal(err)
		}

		got, err := r.List(tempFilePath, "test_sheet")
		if err != nil {
			t.Fatal(err)
		}
		want := &model.Excel{
			Name:   "test_sheet",
			Header: model.Header{"id", "name"},
			Records: []model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
				{"3", "Vika"},
				{"4", "Hartlova"},
			},
		}

		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf("differs: (-got +want)\n%s", diff)
		}
	})
}
