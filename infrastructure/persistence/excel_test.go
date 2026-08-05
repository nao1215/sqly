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

	// XLSX is XML, and XML 1.0 has no way to write most control characters.
	// excelize substitutes U+FFFD for them, which turns an export into silent
	// data loss: the file is written, the process exits 0, and the value is gone.
	// LTSV already refuses the values it cannot represent, and so must this.
	t.Run("a value XLSX cannot represent is refused, not mangled", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			value string
		}{
			{name: "NUL", value: "A\x00B"},
			{name: "start of heading", value: "A\x01B"},
			{name: "vertical tab", value: "A\x0bB"},
			{name: "unit separator", value: "A\x1fB"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := NewExcelRepository()
				table := model.NewTable("t", model.Header{"v"}, []model.Record{{tt.value}})
				out := filepath.Join(t.TempDir(), "control.xlsx")

				err := r.Dump(out, table)
				if err == nil {
					t.Fatalf("Dump(%q) = nil error, want a refusal", tt.value)
				}
				if !strings.Contains(err.Error(), "excel") || !strings.Contains(err.Error(), "control character") {
					t.Errorf("error = %q, want it to name excel and the control character", err.Error())
				}
				if _, statErr := os.Stat(out); statErr == nil {
					t.Errorf("a refused export left %s behind", out)
				}
			})
		}
	})

	// Tab, newline, and carriage return are the three control characters XML
	// keeps, so a value holding them is written rather than refused.
	t.Run("the control characters XLSX can represent are written", func(t *testing.T) {
		t.Parallel()

		r := NewExcelRepository()
		const value = "a\tb\nc\rd"
		table := model.NewTable("t", model.Header{"v"}, []model.Record{{value}})
		out := filepath.Join(t.TempDir(), "keepable.xlsx")

		if err := r.Dump(out, table); err != nil {
			t.Fatalf("Dump: %v", err)
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
			"long":              "this_is_a_very_long_table_name_exceeding_31_chars",
			"forbidden":         "sales/2023:q1[west]",
			"default_lowercase": "sheet1",
			"default_uppercase": "SHEET1",
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

// TestExcelDumpKeepsCharactersXLSXCanCarry pins the other side of the refusal:
// DEL and the replacement character are above U+0020, so XML carries them and
// the export must not reject a value for holding one.
func TestExcelDumpKeepsCharactersXLSXCanCarry(t *testing.T) {
	t.Parallel()

	const value = "A\x7fB�C"
	r := NewExcelRepository()
	table := model.NewTable("t", model.Header{"v"}, []model.Record{{value}})
	out := filepath.Join(t.TempDir(), "high.xlsx")

	if err := r.Dump(out, table); err != nil {
		t.Fatalf("Dump: %v", err)
	}

	f, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := f.GetRows("t")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != value {
		t.Errorf("round-tripped value = %q, want %q", rows, value)
	}
}
