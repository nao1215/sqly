package model

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"
)

// TestRowStreamWritesWhatPrintWrites holds a RowStream to Table.Print: for
// result sets of every shape the formats have rules about, streaming the rows
// writes the same bytes Print writes for a Table of the same cells, and where
// Print fails the stream fails with the same message and writes nothing.
func TestRowStreamWritesWhatPrintWrites(t *testing.T) {
	t.Parallel()

	awkward := []any{
		nil, "", "plain", "a,b", `say "hi"`, "line\nbreak", "tab\there", " padded ", "日本語",
		int64(0), int64(-42), int64(math.MaxInt64), 1.5, 0.1, math.Inf(1), math.NaN(),
		[]byte("bytes"), []byte{0xff, 0xfe}, "\xff\xfe", true, time.Date(2026, 9, 28, 1, 2, 3, 0, time.UTC),
	}
	type result struct {
		name   string
		header Header
		rows   [][]Cell
	}
	results := make([]result, 0, 44)
	results = append(results, []result{
		{name: "no rows", header: Header{"a", "b"}},
		{name: "one empty field", header: Header{"v"}, rows: [][]Cell{{NewCell("")}, {NewCell("x")}, {NewCell(nil)}}},
		{name: "duplicate columns", header: Header{"a", "a"}, rows: [][]Cell{{NewCell("1"), NewCell("2")}}},
		{name: "columns one import reads as one", header: Header{"a", "A"}, rows: [][]Cell{{NewCell("1"), NewCell("2")}}},
	}...)
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // deterministic input generation
	for n := range 40 {
		width := 1 + rng.Intn(4)
		header := make(Header, width)
		for i := range header {
			header[i] = fmt.Sprintf("c%d", i)
		}
		rows := make([][]Cell, rng.Intn(6))
		for r := range rows {
			rows[r] = make([]Cell, width)
			for c := range rows[r] {
				rows[r][c] = NewCell(awkward[rng.Intn(len(awkward))])
			}
		}
		results = append(results, result{name: fmt.Sprintf("random %d", n), header: header, rows: rows})
	}

	for _, mode := range []PrintMode{PrintModeCSV, PrintModeTSV, PrintModeJSON, PrintModeJSONL} {
		failed, written := 0, 0
		for _, r := range results {
			table, err := NewTableFromCells("t", r.header, r.rows)
			if err != nil {
				t.Fatal(err)
			}
			var want bytes.Buffer
			wantErr := table.Print(&want, mode)

			var got bytes.Buffer
			gotErr := streamRows(&got, mode, r.header, r.rows)

			if (wantErr == nil) != (gotErr == nil) || (wantErr != nil && wantErr.Error() != gotErr.Error()) {
				t.Errorf("mode %d, %s: Print error %v, stream error %v", mode, r.name, wantErr, gotErr)
				continue
			}
			if gotErr != nil {
				failed++
				if got.Len() != 0 {
					t.Errorf("mode %d, %s: a failed stream wrote %q", mode, r.name, got.String())
				}
				continue
			}
			written++
			if !bytes.Equal(want.Bytes(), got.Bytes()) {
				t.Errorf("mode %d, %s:\nPrint  %q\nstream %q", mode, r.name, want.String(), got.String())
			}
		}
		if failed == 0 || written == 0 {
			t.Errorf("mode %d: %d result sets failed and %d were written; the cases do not reach both outcomes", mode, failed, written)
		}
	}
}

// streamRows writes rows through a RowStream the way the shell does.
func streamRows(out *bytes.Buffer, mode PrintMode, header Header, rows [][]Cell) error {
	s, err := NewRowStream(mode)
	if err != nil {
		return err
	}
	if err := s.Header(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.Row(row); err != nil {
			return err
		}
	}
	_, err = s.WriteTo(out)
	return err
}

func TestRowStreamRefusesWhatItCannotStream(t *testing.T) {
	t.Parallel()

	for _, mode := range []PrintMode{PrintModeTable, PrintModeMarkdownTable, PrintModeLTSV, PrintModeVertical, PrintModeExcel, PrintModeParquet} {
		if Streamable(mode) {
			t.Errorf("mode %d is reported streamable", mode)
		}
		if _, err := NewRowStream(mode); err == nil {
			t.Errorf("mode %d: NewRowStream accepted it", mode)
		}
	}
	s, err := NewRowStream(PrintModeCSV)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Header(Header{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Row([]Cell{NewCell("1")}); !errors.Is(err, ErrCellShapeMismatch) {
		t.Errorf("a short row: err = %v, want ErrCellShapeMismatch", err)
	}
}

// TestRowStreamSpillsLargeOutputAndCleansUp holds the temporary file a large
// result moves to: the output is the same bytes Print writes, and the file is
// gone once the stream has been written out, or closed without being written.
func TestRowStreamSpillsLargeOutputAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)

	header := Header{"id", "text"}
	rows := make([][]Cell, 0, 120000)
	for i := range 120000 {
		rows = append(rows, []Cell{NewCell(int64(i)), NewCell(fmt.Sprintf("a value long enough to add up, number %d", i))})
	}
	table, err := NewTableFromCells("t", header, rows)
	if err != nil {
		t.Fatal(err)
	}

	for _, mode := range []PrintMode{PrintModeCSV, PrintModeJSON} {
		var want bytes.Buffer
		if err := table.Print(&want, mode); err != nil {
			t.Fatal(err)
		}
		if want.Len() <= spoolMemoryLimit {
			t.Fatalf("mode %d: %d bytes does not reach the spill limit", mode, want.Len())
		}

		var got bytes.Buffer
		if err := streamRows(&got, mode, header, rows); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want.Bytes(), got.Bytes()) {
			t.Errorf("mode %d: a spilled stream wrote different bytes from Print", mode)
		}
		assertNoTemporaryFiles(t, dir)

		s, err := NewRowStream(mode)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Header(header); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if err := s.Row(row); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		assertNoTemporaryFiles(t, dir)
	}
}

func assertNoTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("%d temporary files were left behind", len(entries))
	}
}
