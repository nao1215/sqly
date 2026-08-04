package model

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestTablePrintVertical pins the vertical layout.
//
// Every other mode lays a record out across the line, which stops working at the
// width sqly exists for: a 300-column row is one 2700-character line in table,
// csv, tsv, and ltsv alike. Vertical turns the row on its side, so the cost is
// vertical space, which a terminal scrolls.
func TestTablePrintVertical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		header  Header
		records []Record
		want    string
	}{
		{
			name:    "one record, names padded to the longest",
			header:  Header{"id", "user_name"},
			records: []Record{{"1", "alice"}},
			want: "-[ RECORD 1 ]-----------------------------------------------\n" +
				"id        | 1\n" +
				"user_name | alice\n",
		},
		{
			name:    "each record gets its own numbered rule",
			header:  Header{"id"},
			records: []Record{{"1"}, {"2"}, {"3"}},
			want: "-[ RECORD 1 ]-----------------------------------------------\n" +
				"id | 1\n" +
				"-[ RECORD 2 ]-----------------------------------------------\n" +
				"id | 2\n" +
				"-[ RECORD 3 ]-----------------------------------------------\n" +
				"id | 3\n",
		},
		{
			name:    "an empty value keeps its line",
			header:  Header{"a", "b"},
			records: []Record{{"", "x"}},
			want: "-[ RECORD 1 ]-----------------------------------------------\n" +
				"a | \n" +
				"b | x\n",
		},
		{
			name:    "the gutter is measured in runes, not bytes",
			header:  Header{"名前", "id"},
			records: []Record{{"アリス", "1"}},
			want: "-[ RECORD 1 ]-----------------------------------------------\n" +
				"名前 | アリス\n" +
				"id   | 1\n",
		},
		{
			name:    "a value carrying a newline is written as it is",
			header:  Header{"note"},
			records: []Record{{"first\nsecond"}},
			want: "-[ RECORD 1 ]-----------------------------------------------\n" +
				"note | first\nsecond\n",
		},
		{
			name:    "a value carrying the separator needs no escaping",
			header:  Header{"expr"},
			records: []Record{{"a | b"}},
			want: "-[ RECORD 1 ]-----------------------------------------------\n" +
				"expr | a | b\n",
		},
		{
			name:    "no records prints nothing",
			header:  Header{"id", "name"},
			records: []Record{},
			want:    "",
		},
		{
			name:    "a record shorter than the header still lists every column",
			header:  Header{"a", "b", "c"},
			records: []Record{{"1"}},
			want: "-[ RECORD 1 ]-----------------------------------------------\n" +
				"a | 1\n" +
				"b | \n" +
				"c | \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			tbl := NewTable("t", tt.header, tt.records)
			if err := tbl.Print(&buf, PrintModeVertical); err != nil {
				t.Fatalf("Print() error = %v, want nil", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("Print(vertical) =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// TestTablePrintVerticalWideRow is the case the mode exists for: a row too wide
// to read across becomes one line per column, each short enough to read and to
// grep. The horizontal modes are measured alongside it so the comparison is not
// an assertion about a number nobody checked.
func TestTablePrintVerticalWideRow(t *testing.T) {
	t.Parallel()

	const columns = 300
	header := make(Header, columns)
	record := make(Record, columns)
	for i := range columns {
		header[i] = fmt.Sprintf("col_%03d", i+1)
		record[i] = fmt.Sprintf("v%d", i+1)
	}
	record[157] = "BAD"
	tbl := NewTable("wide", header, []Record{record})

	var flat bytes.Buffer
	if err := tbl.Print(&flat, PrintModeCSV); err != nil {
		t.Fatal(err)
	}
	csvDataLine := strings.Split(strings.TrimRight(flat.String(), "\n"), "\n")[1]
	if len(csvDataLine) < 1000 {
		t.Fatalf("the csv data line is %d characters; this test assumes a row too wide to read across", len(csvDataLine))
	}

	var vertical bytes.Buffer
	if err := tbl.Print(&vertical, PrintModeVertical); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(vertical.String(), "\n"), "\n")
	if len(lines) != columns+1 {
		t.Errorf("vertical produced %d lines, want %d (one rule + one per column)", len(lines), columns+1)
	}
	for _, line := range lines {
		if len(line) > 80 {
			t.Errorf("a vertical line is %d characters, which defeats the point: %q", len(line), line)
		}
	}
	// The column holding the odd value is findable by name, which is the whole
	// point: the flat line gives no name to grep for.
	if !strings.Contains(vertical.String(), "col_158 | BAD") {
		t.Errorf("vertical output should name the column holding BAD:\n%s", vertical.String())
	}
}

// TestPrintModeIsDisplayOnly pins which modes name no export format. The export
// path asks this to decide whether the mode chose a destination's format or the
// destination's extension did, so a mode landing on the wrong side turns `.dump
// out.tsv` into either a conflict error or a file in the wrong format.
func TestPrintModeIsDisplayOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode PrintMode
		want bool
	}{
		{mode: PrintModeTable, want: true},
		{mode: PrintModeVertical, want: true},
		{mode: PrintModeCSV, want: false},
		{mode: PrintModeTSV, want: false},
		{mode: PrintModeLTSV, want: false},
		{mode: PrintModeMarkdownTable, want: false},
		{mode: PrintModeJSON, want: false},
		{mode: PrintModeJSONL, want: false},
		{mode: PrintModeExcel, want: false},
		{mode: PrintModeParquet, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			t.Parallel()

			if got := tt.mode.IsDisplayOnly(); got != tt.want {
				t.Errorf("%s.IsDisplayOnly() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

// TestExportFormatFromVertical pins that vertical falls back to CSV like table
// does, so a `.dump` with no extension in vertical mode writes something a reader
// can load rather than the on-screen layout.
func TestExportFormatFromVertical(t *testing.T) {
	t.Parallel()

	if got, want := ExportFormatFromPrintMode(PrintModeVertical), ExportCSV; got != want {
		t.Errorf("ExportFormatFromPrintMode(vertical) = %v, want %v", got, want)
	}
}
