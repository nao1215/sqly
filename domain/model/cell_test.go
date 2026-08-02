package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestCellStringUsesDriverType pins the single display contract every text
// format shares. Before Cell existed, the two query repositories each formatted
// a driver value with their own switch statement, so the same INTEGER could
// print differently depending on which repository produced the table.
func TestCellStringUsesDriverType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cell Cell
		want string
	}{
		{name: "null renders as empty", cell: NullCell(), want: ""},
		{name: "empty text renders as empty", cell: NewTextCell(""), want: ""},
		{name: "text is verbatim", cell: NewTextCell("hello"), want: "hello"},
		{name: "leading zeros survive", cell: NewTextCell("00123"), want: "00123"},
		{name: "integer is decimal", cell: NewCell(int64(42)), want: "42"},
		{name: "negative integer", cell: NewCell(int64(-7)), want: "-7"},
		{name: "real is shortest round-trip", cell: NewCell(1.5), want: "1.5"},
		{name: "real without fraction", cell: NewCell(2.0), want: "2"},
		{name: "bool", cell: NewCell(true), want: "true"},
		{name: "bytes are text", cell: NewCell([]byte("blob")), want: "blob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.cell.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCellNullIsNotEmptyString is the distinction the whole design exists for:
// a NULL and an empty string are indistinguishable in every text format, so if
// the model conflated them, JSON output could not tell them apart either.
func TestCellNullIsNotEmptyString(t *testing.T) {
	t.Parallel()

	if !NullCell().IsNull() {
		t.Error("NullCell().IsNull() = false")
	}
	if NewTextCell("").IsNull() {
		t.Error("NewTextCell(\"\").IsNull() = true, want false")
	}
	if NullCell().String() != NewTextCell("").String() {
		t.Error("NULL and empty string should render identically in text output")
	}
	if NullCell().Value() != nil {
		t.Errorf("NullCell().Value() = %#v, want nil", NullCell().Value())
	}
}

// TestCellCopiesByteSlices locks the database/sql rule that a scanned []byte is
// only valid until the next Rows.Next. Aliasing the driver's buffer let a later
// row silently rewrite an earlier cell; both the store and the read are copied,
// so neither side can reach the other's bytes.
func TestCellCopiesByteSlices(t *testing.T) {
	t.Parallel()

	src := []byte("original")
	cell := NewCell(src)
	src[0] = 'X' // the driver reuses its buffer for the next row
	if got := cell.String(); got != "original" {
		t.Errorf("String() = %q after the source buffer was reused, want %q", got, "original")
	}

	out, ok := cell.Value().([]byte)
	if !ok {
		t.Fatalf("Value() = %#v, want []byte", cell.Value())
	}
	out[0] = 'Y'
	if got := cell.String(); got != "original" {
		t.Errorf("String() = %q after mutating Value()'s result, want %q", got, "original")
	}
}

// TestNewTableFromCellsRejectsShapeMismatch checks that a row whose width
// disagrees with the header fails when the table is built. Previously the two
// parallel slices (records and jsonValues) could disagree, and the mismatch
// surfaced only as a silently skipped value part-way through a JSON stream that
// had already been written to stdout.
func TestNewTableFromCellsRejectsShapeMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rows [][]Cell
	}{
		{name: "row too short", rows: [][]Cell{{NewTextCell("a")}}},
		{name: "row too long", rows: [][]Cell{{NewTextCell("a"), NewTextCell("b"), NewTextCell("c")}}},
		{
			name: "second row diverges",
			rows: [][]Cell{
				{NewTextCell("a"), NewTextCell("b")},
				{NewTextCell("a")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewTableFromCells("t", Header{"x", "y"}, tt.rows)
			if err == nil {
				t.Fatalf("NewTableFromCells returned a table for a mismatched row: %#v", got)
			}
			if !errors.Is(err, ErrCellShapeMismatch) {
				t.Errorf("error = %v, want it to wrap ErrCellShapeMismatch", err)
			}
			if got != nil {
				t.Errorf("table = %#v, want nil on error", got)
			}
		})
	}
}

// TestNewTableFromCellsCopiesInput verifies the table owns its cells: mutating
// the caller's rows afterwards must not change what the table prints.
func TestNewTableFromCellsCopiesInput(t *testing.T) {
	t.Parallel()

	rows := [][]Cell{{NewCell(int64(1)), NewTextCell("keep")}}
	tbl, err := NewTableFromCells("t", Header{"id", "name"}, rows)
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	rows[0][0] = NewTextCell("tampered")
	rows[0][1] = NullCell()

	var out bytes.Buffer
	if err := tbl.Print(&out, PrintModeJSON); err != nil {
		t.Fatalf("Print: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if n, ok := got[0]["id"].(float64); !ok || n != 1 {
		t.Errorf("id = %#v (%T), want JSON number 1 — caller mutation leaked in", got[0]["id"], got[0]["id"])
	}
	if got[0]["name"] != "keep" {
		t.Errorf("name = %#v, want \"keep\" — caller mutation leaked in", got[0]["name"])
	}
}

// TestTableRecordsMatchCells is the invariant that replaced hand-synchronized
// slices: the string a text format prints is derived from the same cell the JSON
// format encodes, so the two representations of a row cannot disagree.
func TestTableRecordsMatchCells(t *testing.T) {
	t.Parallel()

	tbl, err := NewTableFromCells("t", Header{"i", "r", "s", "n"}, [][]Cell{
		{NewCell(int64(42)), NewCell(1.5), NewTextCell("00123"), NullCell()},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	var csv bytes.Buffer
	if err := tbl.Print(&csv, PrintModeCSV); err != nil {
		t.Fatalf("Print(csv): %v", err)
	}
	wantCSV := "i,r,s,n\n42,1.5,00123,\n"
	if csv.String() != wantCSV {
		t.Errorf("csv = %q, want %q", csv.String(), wantCSV)
	}

	var nd bytes.Buffer
	if err := tbl.Print(&nd, PrintModeNDJSON); err != nil {
		t.Fatalf("Print(ndjson): %v", err)
	}
	wantNDJSON := "{\"i\":42,\"r\":1.5,\"s\":\"00123\",\"n\":null}\n"
	if nd.String() != wantNDJSON {
		t.Errorf("ndjson = %q, want %q", nd.String(), wantNDJSON)
	}

	// The CSV field and the JSON scalar must be spellings of the same value.
	fields := strings.Split(strings.Split(csv.String(), "\n")[1], ",")
	for i, record := range tbl.Records() {
		for j := range tbl.Header() {
			if record[j] != fields[j] {
				t.Errorf("row %d col %d: record %q != csv field %q", i, j, record[j], fields[j])
			}
		}
	}
}

// TestWithNameDoesNotShareRowStorage locks the copy rule for the rename helper.
// The previous struct-value copy gave both tables the same slice headers, so an
// append to one wrote into the other's backing array past its length — a silent
// cross-table corruption. The rows are cloned into separate arrays now.
func TestWithNameDoesNotShareRowStorage(t *testing.T) {
	t.Parallel()

	tbl, err := NewTableFromCells("original", Header{"id"}, [][]Cell{
		{NewCell(int64(1))},
		{NewCell(int64(2))},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}
	renamed := tbl.WithName("renamed")

	// Appending to the copy must not be observable through the original, and
	// vice versa: the two must not share one backing array.
	renamed.records = append(renamed.records, Record{"appended"})
	renamed.cells = append(renamed.cells, NewTextCell("appended"))
	if got := len(tbl.Records()); got != 2 {
		t.Errorf("original rows = %d after appending to the renamed copy, want 2", got)
	}
	if tbl.Records()[1][0] != "2" {
		t.Errorf("original record 1 = %q, want %q", tbl.Records()[1][0], "2")
	}
	if renamed.Name() != "renamed" || tbl.Name() != "original" {
		t.Errorf("names = %q / %q, want renamed / original", renamed.Name(), tbl.Name())
	}
	// Native values survive the rename: this is why WithName exists at all.
	if renamed.IsNull(0, 0) {
		t.Error("IsNull = true after rename for a non-NULL INTEGER cell")
	}
	var out bytes.Buffer
	if err := renamed.Print(&out, PrintModeNDJSON); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if !strings.Contains(out.String(), `{"id":1}`) {
		t.Errorf("ndjson = %q, want the INTEGER to stay a JSON number", out.String())
	}
}

// TestTableWithoutCellsTreatsEveryValueAsText covers the string-built tables
// (imported files, synthesized reports): they carry no type information, so a
// numeric-looking value must never be promoted to a JSON number.
func TestTableWithoutCellsTreatsEveryValueAsText(t *testing.T) {
	t.Parallel()

	tbl := NewTable("t", Header{"a", "b", "c"}, []Record{{"123", "true", "00123"}})
	if tbl.IsNull(0, 0) {
		t.Error("IsNull = true for a table with no NULL information")
	}
	var out bytes.Buffer
	if err := tbl.Print(&out, PrintModeNDJSON); err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "{\"a\":\"123\",\"b\":\"true\",\"c\":\"00123\"}\n"
	if out.String() != want {
		t.Errorf("ndjson = %q, want %q", out.String(), want)
	}
}
