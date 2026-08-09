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
		if err := DumpExcel(tempFilePath, table); err != nil {
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
			// XML 1.0 stops the BMP at U+FFFD, so the two noncharacters above it
			// are as unwritable as a control character and are substituted the
			// same way.
			{name: "noncharacter U+FFFE", value: "A\ufffeB"},
			{name: "noncharacter U+FFFF", value: "A\uffffB"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				table := model.NewTable("t", model.Header{"v"}, []model.Record{{tt.value}})
				out := filepath.Join(t.TempDir(), "control.xlsx")

				err := DumpExcel(out, table)
				if err == nil {
					t.Fatalf("Dump(%q) = nil error, want a refusal", tt.value)
				}
				if !strings.Contains(err.Error(), "excel") || !strings.Contains(err.Error(), "U+") {
					t.Errorf("error = %q, want it to name excel and the character", err.Error())
				}
				if _, statErr := os.Stat(out); statErr == nil {
					t.Errorf("a refused export left %s behind", out)
				}
			})
		}
	})

	// A column name is written into the same XML as a value, so it is checked the
	// same way rather than reaching the writer to be substituted.
	t.Run("a column name XLSX cannot represent is refused", func(t *testing.T) {
		t.Parallel()

		for _, label := range []string{"a\x01b", "a\ufffeb", "a\uffffb"} {
			table := model.NewTable("t", model.Header{label}, []model.Record{{"v"}})
			out := filepath.Join(t.TempDir(), "header.xlsx")

			if err := DumpExcel(out, table); err == nil {
				t.Errorf("Dump with header %q = nil error, want a refusal", label)
			}
		}
	})

	// Tab, newline, and carriage return are the three control characters XML
	// keeps, so a value holding them is written rather than refused.
	t.Run("the control characters XLSX can represent are written", func(t *testing.T) {
		t.Parallel()

		const value = "a\tb\nc\rd"
		table := model.NewTable("t", model.Header{"v"}, []model.Record{{value}})
		out := filepath.Join(t.TempDir(), "keepable.xlsx")

		if err := DumpExcel(out, table); err != nil {
			t.Fatalf("Dump: %v", err)
		}
	})

	t.Run("dumped excel file is not executable", func(t *testing.T) {
		t.Parallel()

		table := model.NewTable(
			"test_sheet",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}},
		)
		tempFilePath := filepath.Join(t.TempDir(), "perms.xlsx")
		if err := DumpExcel(tempFilePath, table); err != nil {
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

				table := model.NewTable(
					name,
					model.Header{"id", "name"},
					[]model.Record{{"1", "Gina"}},
				)
				tempFilePath := filepath.Join(t.TempDir(), "out.xlsx")
				if err := DumpExcel(tempFilePath, table); err != nil {
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
	table := model.NewTable("t", model.Header{"v"}, []model.Record{{value}})
	out := filepath.Join(t.TempDir(), "high.xlsx")

	if err := DumpExcel(out, table); err != nil {
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

// TestExcelDumpWritesNullAsAnEmptyCell covers the difference a workbook can
// carry and the export was throwing away. A NULL and an empty string are two
// values, and writing the empty string for both told every reader of the file
// that the column had a value in every row. A workbook has no null, but it does
// have "no cell", which is what a reader shows as blank.
func TestExcelDumpWritesNullAsAnEmptyCell(t *testing.T) {
	t.Parallel()

	table, err := model.NewTableFromCells("t", model.Header{"id", "note", "tail"}, [][]model.Cell{
		{model.NewCell(int64(1)), model.NewCell(nil), model.NewCell("x")},
		{model.NewCell(int64(2)), model.NewCell(""), model.NewCell("y")},
	})
	if err != nil {
		t.Fatalf("build the table: %v", err)
	}

	out := filepath.Join(t.TempDir(), "nulls.xlsx")
	if err := DumpExcel(out, table); err != nil {
		t.Fatalf("Dump: %v", err)
	}

	f, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	// A NULL leaves no cell behind; an empty string leaves a cell holding it.
	// excelize reports the unset cell as type 0 and the written one as a string.
	nullType, err := f.GetCellType("t", "B2")
	if err != nil {
		t.Fatal(err)
	}
	if nullType != excelize.CellTypeUnset {
		t.Errorf("the NULL cell has type %v, want unset: a workbook says null by holding no cell", nullType)
	}

	emptyType, err := f.GetCellType("t", "B3")
	if err != nil {
		t.Fatal(err)
	}
	if emptyType == excelize.CellTypeUnset {
		t.Error("the empty string was written as no cell, which is what NULL means here")
	}

	// The rows are otherwise intact, so the blank cell cost nothing around it.
	rows, err := f.GetRows("t")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %q, want a header and two data rows", rows)
	}
	if rows[1][0] != "1" || rows[1][2] != "x" {
		t.Errorf("row 1 = %q, want the id and tail unchanged around the NULL", rows[1])
	}
}

// TestExcelDumpRefusesInvalidUTF8 pins the hole in the character guard. XLSX is
// XML and cannot hold a byte that is not valid UTF-8, and the writer replaced
// each one with U+FFFD: the export succeeded, the file appeared, and the byte
// was gone. The guard could not see it, because ranging over a Go string decodes
// an invalid byte as U+FFFD before the check ever runs.
func TestExcelDumpRefusesInvalidUTF8(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		table *model.Table
	}{
		{
			name:  "in a value",
			table: model.NewTable("t", model.Header{"v"}, []model.Record{{"bad\xffbyte"}}),
		},
		{
			name:  "in a column name",
			table: model.NewTable("t", model.Header{"bad\xffname"}, []model.Record{{"ok"}}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(t.TempDir(), "out.xlsx")
			err := DumpExcel(out, tt.table)
			if err == nil {
				t.Fatal("Dump succeeded, want a refusal: the invalid byte would be written as U+FFFD")
			}
			if !strings.Contains(err.Error(), "not valid UTF-8") {
				t.Errorf("error = %v, want it to name the invalid UTF-8", err)
			}
			if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
				t.Errorf("Dump left a file behind at %s; a refused export writes nothing", out)
			}
		})
	}
}

// TestExcelDumpRefusesAValuePastTheCellLimit pins the length half of what a cell
// can hold. Excel stops at 32,767 characters and excelize cuts a longer value to
// fit, so a 40,000-character value was written as its first 32,767 with the
// export reporting success — the same silent truncation the character check
// already refuses for a byte XLSX cannot carry.
func TestExcelDumpRefusesAValuePastTheCellLimit(t *testing.T) {
	t.Parallel()

	const limit = 32767

	t.Run("a value of exactly the limit is written whole", func(t *testing.T) {
		t.Parallel()

		out := filepath.Join(t.TempDir(), "out.xlsx")
		table := model.NewTable("t", model.Header{"v"}, []model.Record{{strings.Repeat("x", limit)}})
		if err := DumpExcel(out, table); err != nil {
			t.Fatalf("Dump refused a value the cell holds: %v", err)
		}
	})

	// The limit is in characters, and this is the case that says so: counted in
	// bytes the value is three times over it. Refusing at 32,768 multi-byte runes
	// proves nothing on its own, because a byte count refuses that too.
	t.Run("a multi-byte value of exactly the limit is written whole", func(t *testing.T) {
		t.Parallel()

		out := filepath.Join(t.TempDir(), "out.xlsx")
		table := model.NewTable("t", model.Header{"v"}, []model.Record{{strings.Repeat("あ", limit)}})
		if err := DumpExcel(out, table); err != nil {
			t.Fatalf("Dump refused a value the cell holds: %v", err)
		}
	})

	tests := []struct {
		name  string
		table *model.Table
	}{
		{
			name:  "in a value",
			table: model.NewTable("t", model.Header{"v"}, []model.Record{{strings.Repeat("x", limit+1)}}),
		},
		{
			name:  "in a column name",
			table: model.NewTable("t", model.Header{strings.Repeat("x", limit+1)}, []model.Record{{"ok"}}),
		},
		{
			name:  "a multi-byte value past the limit",
			table: model.NewTable("t", model.Header{"v"}, []model.Record{{strings.Repeat("あ", limit+1)}}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := filepath.Join(t.TempDir(), "out.xlsx")
			err := DumpExcel(out, tt.table)
			if err == nil {
				t.Fatal("Dump succeeded, want a refusal: the value would be cut to the cell limit")
			}
			if !strings.Contains(err.Error(), "32767") {
				t.Errorf("error = %v, want it to name the limit", err)
			}
			if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
				t.Errorf("Dump left a file behind at %s; a refused export writes nothing", out)
			}
		})
	}
}

// TestExcelDumpRefusesATrailingEmptyRow pins the refusal ensureLastRowSurvives
// exists for: such a file reads back one row short.
func TestExcelDumpRefusesATrailingEmptyRow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		table *model.Table
		want  bool
	}{
		{
			name:  "a lone empty row",
			table: model.NewTable("t", model.Header{"a"}, []model.Record{{""}}),
			want:  true,
		},
		{
			name:  "an empty row at the end",
			table: model.NewTable("t", model.Header{"a", "b"}, []model.Record{{"x", "1"}, {"", ""}}),
			want:  true,
		},
		{
			name:  "an empty row in the middle",
			table: model.NewTable("t", model.Header{"a", "b"}, []model.Record{{"x", "1"}, {"", ""}, {"y", "2"}}),
			want:  false,
		},
		{
			name:  "a last row that holds something",
			table: model.NewTable("t", model.Header{"a", "b"}, []model.Record{{"", ""}, {"", "2"}}),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(t.TempDir(), "out.xlsx")
			err := DumpExcel(out, tt.table)
			if tt.want {
				if err == nil {
					t.Fatal("Dump succeeded, want a refusal: the row would be lost on re-import")
				}
				if !strings.Contains(err.Error(), "last row") {
					t.Errorf("error = %v, want it to name the last row", err)
				}
				if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
					t.Errorf("Dump left a file behind at %s; a refused export writes nothing", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("Dump refused a table it can carry: %v", err)
			}
		})
	}
}

// TestExcelDumpKeepsARealReplacementCharacter checks the refusal does not catch
// a U+FFFD the data genuinely contains. The byte sequence is valid UTF-8, and
// XML can carry the character, so it is written unchanged.
func TestExcelDumpKeepsARealReplacementCharacter(t *testing.T) {
	t.Parallel()

	const value = "a�b"
	out := filepath.Join(t.TempDir(), "out.xlsx")
	table := model.NewTable("t", model.Header{"v"}, []model.Record{{value}})
	if err := DumpExcel(out, table); err != nil {
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
