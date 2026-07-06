package persistence

import (
	"os"
	"path/filepath"
	"strings"
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
		defer func() { _ = os.Remove(tempFilePath) }()
		if err := r.Dump(tempFilePath, table); err != nil {
			t.Fatal(err)
		}

		// Verify by reading the dumped file directly with excelize
		f, err := excelize.OpenFile(tempFilePath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()

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

	t.Run("dumped excel file is not executable", func(t *testing.T) {
		t.Parallel()

		r := NewExcelRepository()
		table := model.NewTable(
			"test_sheet",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}},
		)
		tempFilePath := filepath.Join(t.TempDir(), "perms.xlsx")
		if err := r.Dump(tempFilePath, table); err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(tempFilePath)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode&0o111 != 0 {
			t.Errorf("excel export mode = %o, want no executable bits", mode)
		}
	})

	// Excel limits a worksheet name to 31 characters and forbids : \ / ? * [ ].
	// A table name from a long or punctuated filename must still export instead
	// of failing on excelize's NewSheet call.
	t.Run("dumps a table whose name violates Excel sheet-name rules", func(t *testing.T) {
		t.Parallel()

		names := map[string]string{
			"long":      "this_is_a_very_long_table_name_exceeding_31_chars",
			"forbidden": "sales/2023:q1[west]",
		}
		for label, name := range names {
			t.Run(label, func(t *testing.T) {
				t.Parallel()

				r := NewExcelRepository()
				table := model.NewTable(
					name,
					model.Header{"id", "name"},
					[]model.Record{{"1", "Gina"}},
				)
				tempFilePath := filepath.Join(t.TempDir(), "out.xlsx")
				if err := r.Dump(tempFilePath, table); err != nil {
					t.Fatalf("Dump failed for %q: %v", name, err)
				}

				f, err := excelize.OpenFile(tempFilePath)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = f.Close() }()

				sheets := f.GetSheetList()
				if len(sheets) != 1 {
					t.Fatalf("sheet count = %d, want 1", len(sheets))
				}
				sheet := sheets[0]
				if len([]rune(sheet)) > 31 {
					t.Errorf("sheet name %q exceeds 31 characters", sheet)
				}
				if strings.ContainsAny(sheet, `:\/?*[]`) {
					t.Errorf("sheet name %q contains a forbidden character", sheet)
				}
				rows, err := f.GetRows(sheet)
				if err != nil {
					t.Fatal(err)
				}
				wantRows := [][]string{{"id", "name"}, {"1", "Gina"}}
				if diff := cmp.Diff(rows, wantRows); diff != "" {
					t.Fatalf("rows differ: (-got +want)\n%s", diff)
				}
			})
		}
	})
}
