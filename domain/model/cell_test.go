package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
		{name: "null renders as empty", cell: NewCell(nil), want: ""},
		{name: "empty text renders as empty", cell: NewCell(""), want: ""},
		{name: "text is verbatim", cell: NewCell("hello"), want: "hello"},
		{name: "leading zeros survive", cell: NewCell("00123"), want: "00123"},
		{name: "integer is decimal", cell: NewCell(int64(42)), want: "42"},
		{name: "negative integer", cell: NewCell(int64(-7)), want: "-7"},
		{name: "real is shortest round-trip", cell: NewCell(1.5), want: "1.5"},
		{name: "real without fraction", cell: NewCell(2.0), want: "2"},
		{name: "bool", cell: NewCell(true), want: "true"},
		{name: "bytes are text", cell: NewCell([]byte("blob")), want: "blob"},

		// A float32 keeps its own precision: read as 64 bits it prints the error
		// its conversion introduced, 1.100000023841858.
		{name: "float32 keeps its width", cell: NewCell(float32(1.1)), want: "1.1"},
		{name: "positive infinity", cell: NewCell(math.Inf(1)), want: "Infinity"},
		{name: "negative infinity", cell: NewCell(math.Inf(-1)), want: "-Infinity"},
		{name: "not a number", cell: NewCell(math.NaN()), want: "NaN"},
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

	if !NewCell(nil).IsNull() {
		t.Error("NewCell(nil).IsNull() = false")
	}
	if NewCell("").IsNull() {
		t.Error("NewCell(\"\").IsNull() = true, want false")
	}
	if NewCell(nil).String() != NewCell("").String() {
		t.Error("NULL and empty string should render identically in text output")
	}
	if NewCell(nil).Value() != nil {
		t.Errorf("NewCell(nil).Value() = %#v, want nil", NewCell(nil).Value())
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
		{name: "row too short", rows: [][]Cell{{NewCell("a")}}},
		{name: "row too long", rows: [][]Cell{{NewCell("a"), NewCell("b"), NewCell("c")}}},
		{
			name: "second row diverges",
			rows: [][]Cell{
				{NewCell("a"), NewCell("b")},
				{NewCell("a")},
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

	rows := [][]Cell{{NewCell(int64(1)), NewCell("keep")}}
	tbl, err := NewTableFromCells("t", Header{"id", "name"}, rows)
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	rows[0][0] = NewCell("tampered")
	rows[0][1] = NewCell(nil)

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
		{NewCell(int64(42)), NewCell(1.5), NewCell("00123"), NewCell(nil)},
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
	if err := tbl.Print(&nd, PrintModeJSONL); err != nil {
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
	renamed.cells = append(renamed.cells, NewCell("appended"))
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
	if err := renamed.Print(&out, PrintModeJSONL); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if !strings.Contains(out.String(), `{"id":1}`) {
		t.Errorf("ndjson = %q, want the INTEGER to stay a JSON number", out.String())
	}
}

// TestRecordsCannotCorruptTheTable is the ownership rule for the public
// accessor. Records() used to hand out the Table's own storage, so one
// assignment could make the same table print one value as CSV and another as
// JSON — the two representations would disagree and nothing could say which was
// right. The copy is deep, so a write reaches only the caller's slice.
func TestRecordsCannotCorruptTheTable(t *testing.T) {
	t.Parallel()

	table, err := NewTableFromCells("t", Header{"n", "s"}, [][]Cell{
		{NewCell(int64(42)), NewCell("original")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	records := table.Records()
	records[0][0] = "corrupted"
	records[0][1] = "corrupted"

	again := table.Records()
	if again[0][0] != "42" || again[0][1] != "original" {
		t.Errorf("Records() = %v after a caller wrote to an earlier result, want [42 original]", again[0])
	}

	var csv bytes.Buffer
	if err := table.Print(&csv, PrintModeCSV); err != nil {
		t.Fatalf("Print(csv): %v", err)
	}
	if strings.Contains(csv.String(), "corrupted") {
		t.Errorf("csv output was corrupted through Records(): %q", csv.String())
	}

	var nd bytes.Buffer
	if err := table.Print(&nd, PrintModeJSONL); err != nil {
		t.Fatalf("Print(ndjson): %v", err)
	}
	if strings.Contains(nd.String(), "corrupted") {
		t.Errorf("ndjson output was corrupted through Records(): %q", nd.String())
	}
	// Text and JSON still agree with each other, which is the property the
	// corruption would have broken.
	if !strings.Contains(csv.String(), "42") || !strings.Contains(nd.String(), `"n":42`) {
		t.Errorf("csv %q and ndjson %q disagree", csv.String(), nd.String())
	}
}

// TestRowsAndRowShareStorageForHotPaths documents the other half of the
// contract: the iteration API used inside sqly does not copy, so a caller that
// walks a million rows pays nothing, and in exchange must not write to what it
// is given.
func TestRowsAndRowShareStorageForHotPaths(t *testing.T) {
	t.Parallel()

	table := NewTable("t", Header{"a"}, []Record{{"x"}, {"y"}, {"z"}})

	if got := table.RowCount(); got != 3 {
		t.Errorf("RowCount() = %d, want 3", got)
	}

	var seen []string
	for i, record := range table.Rows {
		if i == 2 {
			break // the iterator must honor an early exit
		}
		seen = append(seen, record.At(0))
	}
	if strings.Join(seen, ",") != "x,y" {
		t.Errorf("Rows yielded %v, want [x y] before the break", seen)
	}

	row, ok := table.Row(1)
	if !ok || row.At(0) != "y" {
		t.Errorf("Row(1) = (%v, %v), want (y, true)", row.Record(), ok)
	}
	if _, ok := table.Row(3); ok {
		t.Error("Row(3) reported ok for an out-of-range index")
	}
	if _, ok := table.Row(-1); ok {
		t.Error("Row(-1) reported ok")
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
	if err := tbl.Print(&out, PrintModeJSONL); err != nil {
		t.Fatalf("Print: %v", err)
	}
	want := "{\"a\":\"123\",\"b\":\"true\",\"c\":\"00123\"}\n"
	if out.String() != want {
		t.Errorf("ndjson = %q, want %q", out.String(), want)
	}
}

// TestRowViewCannotWriteThroughToTheTable is the ownership rule for the
// zero-copy read path. Row() and Rows() used to hand out the Table's own
// Record, so `row[0] = "corrupted"` rewrote the table — and because a query
// result's strings are derived from its cells while JSON reads the cells, the
// same table would then print one value as CSV and another as JSON.
//
// A RecordView has no method that writes, so those two lines no longer compile.
// What is left to check is that reading through it is correct and that nothing
// it does return aliases the table.
func TestRowViewCannotWriteThroughToTheTable(t *testing.T) {
	t.Parallel()

	tbl, err := NewTableFromCells("t", Header{"n", "s"}, [][]Cell{
		{NewCell(int64(42)), NewCell("original")},
		{NewCell(int64(7)), NewCell("second")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	row, ok := tbl.Row(0)
	if !ok {
		t.Fatal("Row(0) not found")
	}
	if row.Len() != 2 || row.At(0) != "42" || row.At(1) != "original" {
		t.Errorf("row = %v, want [42 original]", row.Record())
	}
	// Out-of-range reads are blank rather than a panic, because a string-built
	// table may hold rows shorter than its header.
	if got := row.At(2); got != "" {
		t.Errorf("At(2) = %q, want the empty string", got)
	}
	if got := row.At(-1); got != "" {
		t.Errorf("At(-1) = %q, want the empty string", got)
	}

	// Record() is a copy: writing to it must not reach the table.
	copied := row.Record()
	copied[0] = "corrupted"
	if tbl.ValueAt(0, 0) != "42" {
		t.Errorf("ValueAt(0,0) = %q after writing to Record(), want 42", tbl.ValueAt(0, 0))
	}

	// AppendTo copies into the caller's buffer; writing to it must not reach in.
	buf := row.AppendTo(nil)
	buf[1] = "corrupted"
	if tbl.ValueAt(0, 1) != "original" {
		t.Errorf("ValueAt(0,1) = %q after writing to AppendTo's result, want original", tbl.ValueAt(0, 1))
	}

	// Out-of-range rows report absence rather than returning a usable view.
	if _, ok := tbl.Row(2); ok {
		t.Error("Row(2) reported ok for an out-of-range index")
	}
	if _, ok := tbl.Row(-1); ok {
		t.Error("Row(-1) reported ok")
	}
	if got := tbl.ValueAt(5, 0); got != "" {
		t.Errorf("ValueAt(5,0) = %q, want the empty string", got)
	}
}

// TestRowsIteration pins order, early exit, and the empty and single-row cases,
// and checks that nothing the callback receives can write back into the table.
func TestRowsIteration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rows  [][]Cell
		want  []string
		stop  int // stop after this many rows; -1 to iterate all
		after []string
	}{
		{name: "no rows", rows: nil, want: nil, stop: -1},
		{name: "one row", rows: [][]Cell{{NewCell("a")}}, want: []string{"a"}, stop: -1},
		{
			name: "several rows in order",
			rows: [][]Cell{{NewCell("a")}, {NewCell("b")}, {NewCell("c")}},
			want: []string{"a", "b", "c"},
			stop: -1,
		},
		{
			name: "early exit stops the iteration",
			rows: [][]Cell{{NewCell("a")}, {NewCell("b")}, {NewCell("c")}},
			want: []string{"a", "b"},
			stop: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tbl, err := NewTableFromCells("t", Header{"v"}, tt.rows)
			if err != nil {
				t.Fatalf("NewTableFromCells: %v", err)
			}
			if got := tbl.RowCount(); got != len(tt.rows) {
				t.Errorf("RowCount() = %d, want %d", got, len(tt.rows))
			}

			var seen []string
			var indexes []int
			for i, row := range tbl.Rows {
				indexes = append(indexes, i)
				seen = append(seen, row.At(0))
				// Whatever the callback does with what it is handed, the table
				// must be unchanged afterwards.
				r := row.Record()
				r[0] = "corrupted"
				if tt.stop >= 0 && len(seen) == tt.stop {
					break
				}
			}
			if strings.Join(seen, ",") != strings.Join(tt.want, ",") {
				t.Errorf("Rows yielded %v, want %v", seen, tt.want)
			}
			for i, idx := range indexes {
				if idx != i {
					t.Errorf("index %d yielded out of order: got %d", i, idx)
				}
			}
			for i, want := range tt.want {
				if got := tbl.ValueAt(i, 0); got != want {
					t.Errorf("ValueAt(%d,0) = %q after iterating, want %q", i, got, want)
				}
			}
		})
	}
}

// TestHeaderCannotBeMutatedThroughTheAccessor checks the header copy, and that
// the zero-copy reads agree with it.
func TestHeaderCannotBeMutatedThroughTheAccessor(t *testing.T) {
	t.Parallel()

	tbl, err := NewTableFromCells("t", Header{"first", "second"}, [][]Cell{
		{NewCell("a"), NewCell("b")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	header := tbl.Header()
	header[0] = "corrupted"
	if got := tbl.Header(); got[0] != "first" {
		t.Errorf("Header()[0] = %q after writing to an earlier result, want first", got[0])
	}
	if got := tbl.ColumnName(0); got != "first" {
		t.Errorf("ColumnName(0) = %q, want first", got)
	}
	if got := tbl.ColumnCount(); got != 2 {
		t.Errorf("ColumnCount() = %d, want 2", got)
	}
	if got := tbl.ColumnName(9); got != "" {
		t.Errorf("ColumnName(9) = %q, want the empty string", got)
	}

	var names []string
	for _, name := range tbl.Columns {
		names = append(names, name)
	}
	if strings.Join(names, ",") != "first,second" {
		t.Errorf("Columns yielded %v, want [first second]", names)
	}

	// The corrupted name must not have reached the output either.
	var out bytes.Buffer
	if err := tbl.Print(&out, PrintModeCSV); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if !strings.HasPrefix(out.String(), "first,second\n") {
		t.Errorf("csv header = %q, want first,second", out.String())
	}
}

// TestWithNameHeaderIsIndependent locks the rename helper's ownership: renaming
// a column on one table must not rename it on the other, and everything except
// the table name must stay identical.
func TestWithNameHeaderIsIndependent(t *testing.T) {
	t.Parallel()

	tbl, err := NewTableFromCells("original", Header{"id", "name"}, [][]Cell{
		{NewCell(int64(1)), NewCell("alice")},
		{NewCell(int64(2)), NewCell("bob")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}
	renamed := tbl.WithName("renamed")

	// Mutating either table's header copy must not reach the other.
	renamed.Header()[0] = "corrupted"
	tbl.Header()[1] = "corrupted"
	if got := tbl.ColumnName(0); got != "id" {
		t.Errorf("original ColumnName(0) = %q, want id", got)
	}
	if got := renamed.ColumnName(1); got != "name" {
		t.Errorf("renamed ColumnName(1) = %q, want name", got)
	}

	if renamed.Name() != "renamed" || tbl.Name() != "original" {
		t.Errorf("names = %q / %q, want renamed / original", renamed.Name(), tbl.Name())
	}

	// Everything except the name renders identically, including the native types.
	for _, mode := range []PrintMode{PrintModeCSV, PrintModeJSONL, PrintModeTable} {
		var a, b bytes.Buffer
		if err := tbl.Print(&a, mode); err != nil {
			t.Fatalf("Print(original, %s): %v", mode, err)
		}
		if err := renamed.Print(&b, mode); err != nil {
			t.Fatalf("Print(renamed, %s): %v", mode, err)
		}
		if a.String() != b.String() {
			t.Errorf("%s output differs after WithName:\noriginal:\n%s\nrenamed:\n%s", mode, a.String(), b.String())
		}
	}
}

// TestOutputFormatsAgreeAfterPublicAPIUse is the end-to-end statement of why the
// ownership rules exist: after a caller has used every public accessor and
// written to everything it was given, the record-based formats and the
// cell-based formats must still describe the same values.
func TestOutputFormatsAgreeAfterPublicAPIUse(t *testing.T) {
	t.Parallel()

	tbl, err := NewTableFromCells("t", Header{"i", "r", "s", "n"}, [][]Cell{
		{NewCell(int64(42)), NewCell(1.5), NewCell("00123"), NewCell(nil)},
		{NewCell(int64(-7)), NewCell(2.0), NewCell("true"), NewCell("")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	// Use every accessor a caller has, and write to everything it hands back.
	tbl.Header()[0] = "corrupted"
	for _, record := range tbl.Records() {
		for i := range record {
			record[i] = "corrupted"
		}
	}
	if row, ok := tbl.Row(0); ok {
		row.Record()[0] = "corrupted"
		row.AppendTo(nil)
	}
	for _, row := range tbl.Rows {
		row.Record()[0] = "corrupted"
	}

	var csvOut, tsvOut, jsonOut, tableOut bytes.Buffer
	for _, tc := range []struct {
		mode PrintMode
		buf  *bytes.Buffer
	}{
		{PrintModeCSV, &csvOut},
		{PrintModeTSV, &tsvOut},
		{PrintModeJSON, &jsonOut},
		{PrintModeTable, &tableOut},
	} {
		if err := tbl.Print(tc.buf, tc.mode); err != nil {
			t.Fatalf("Print(%s): %v", tc.mode, err)
		}
		if strings.Contains(tc.buf.String(), "corrupted") {
			t.Fatalf("%s output was corrupted through a public accessor:\n%s", tc.mode, tc.buf.String())
		}
	}

	// The record-based and cell-based formats must agree value by value.
	var decoded []map[string]any
	if err := json.Unmarshal(jsonOut.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json: %v (%q)", err, jsonOut.String())
	}
	csvLines := strings.Split(strings.TrimRight(csvOut.String(), "\n"), "\n")
	tsvLines := strings.Split(strings.TrimRight(tsvOut.String(), "\n"), "\n")
	if len(csvLines) != tbl.RowCount()+1 || len(tsvLines) != tbl.RowCount()+1 {
		t.Fatalf("csv/tsv line counts = %d/%d, want %d", len(csvLines), len(tsvLines), tbl.RowCount()+1)
	}
	for row := range tbl.RowCount() {
		csvFields := strings.Split(csvLines[row+1], ",")
		tsvFields := strings.Split(tsvLines[row+1], "\t")
		for col := range tbl.ColumnCount() {
			want := tbl.ValueAt(row, col)
			if csvFields[col] != want {
				t.Errorf("row %d col %d: csv %q != %q", row, col, csvFields[col], want)
			}
			if tsvFields[col] != want {
				t.Errorf("row %d col %d: tsv %q != %q", row, col, tsvFields[col], want)
			}
			name := tbl.ColumnName(col)
			// A JSON null is the one value with no string spelling of its own;
			// it must correspond to a blank display cell, not to "null".
			if decoded[row][name] == nil {
				if want != "" {
					t.Errorf("row %d %q: json null but display %q", row, name, want)
				}
				continue
			}
			if got := fmt.Sprintf("%v", decoded[row][name]); got != want {
				t.Errorf("row %d %q: json %q != display %q", row, name, got, want)
			}
		}
	}
}
