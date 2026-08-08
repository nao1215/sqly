package model

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestPrintModeString(t *testing.T) {
	tests := []struct {
		name string
		p    PrintMode
		want string
	}{
		{
			name: "table mode",
			p:    PrintModeTable,
			want: "table",
		},
		{
			name: "markdown mode",
			p:    PrintModeMarkdownTable,
			want: "markdown",
		},
		{
			name: "csv mode",
			p:    PrintModeCSV,
			want: "csv",
		},
		{
			name: "tsv mode",
			p:    PrintModeTSV,
			want: "tsv",
		},
		{
			name: "ltsv mode",
			p:    PrintModeLTSV,
			want: "ltsv",
		},
		{
			name: "excel mode",
			p:    PrintModeExcel,
			want: "excel",
		},
		{
			name: "json mode",
			p:    PrintModeJSON,
			want: "json",
		},
		{
			name: "jsonl mode",
			p:    PrintModeJSONL,
			want: "jsonl",
		},
		{
			name: "parquet mode",
			p:    PrintModeParquet,
			want: "parquet",
		},
		{
			name: "unknown mode",
			p:    100, // not defined
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("PrintMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTablePrint(t *testing.T) {
	type fields struct {
		Name    string
		Header  Header
		Records []Record
	}
	type args struct {
		mode PrintMode
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantOut string
	}{
		{
			name: "print table",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{PrintModeTable},
			wantOut: `+-----+-----+-----+
| aaa | bbb | ccc |
+-----+-----+-----+
| 111 | 222 | 333 |
| 444 | 555 | 666 |
| 777 | 888 | 999 |
+-----+-----+-----+
`,
		},
		{
			name: "print markdown table",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{PrintModeMarkdownTable},
			wantOut: `| aaa | bbb | ccc |
|-----|-----|-----|
| 111 | 222 | 333 |
| 444 | 555 | 666 |
| 777 | 888 | 999 |
`,
		},
		{
			name: "print csv",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{PrintModeCSV},
			wantOut: `aaa,bbb,ccc
111,222,333
444,555,666
777,888,999
`,
		},
		{
			name: "print tsv",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{PrintModeTSV},
			wantOut: `aaa	bbb	ccc
111	222	333
444	555	666
777	888	999
`,
		},
		{
			name: "print ltsv",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{PrintModeLTSV},
			wantOut: `aaa:111	bbb:222	ccc:333
aaa:444	bbb:555	ccc:666
aaa:777	bbb:888	ccc:999
`,
		},
		{
			name: "print excel (same as csv)",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{PrintModeExcel},
			wantOut: `aaa,bbb,ccc
111,222,333
444,555,666
777,888,999
`,
		},
		{
			name: "print json",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
				},
			},
			args: args{PrintModeJSON},
			wantOut: `[
  {"aaa":"111","bbb":"222","ccc":"333"},
  {"aaa":"444","bbb":"555","ccc":"666"}
]
`,
		},
		{
			name: "print json with no records",
			fields: fields{
				Name:    "empty_table",
				Header:  Header{"aaa", "bbb"},
				Records: []Record{},
			},
			args:    args{PrintModeJSON},
			wantOut: "[]\n",
		},
		{
			name: "print ndjson",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
				},
			},
			args: args{PrintModeJSONL},
			wantOut: `{"aaa":"111","bbb":"222","ccc":"333"}
{"aaa":"444","bbb":"555","ccc":"666"}
`,
		},
		{
			name: "print ndjson with no records",
			fields: fields{
				Name:    "empty_table",
				Header:  Header{"aaa", "bbb"},
				Records: []Record{},
			},
			args:    args{PrintModeJSONL},
			wantOut: "",
		},
		{
			name: "print ndjson escapes special characters",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"name", "note"},
				Records: []Record{
					{`a"b`, "c\td"},
				},
			},
			args: args{PrintModeJSONL},
			wantOut: `{"name":"a\"b","note":"c\td"}
`,
		},
		{
			name: "print table (default mode)",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{100}, // not defined
			wantOut: `+-----+-----+-----+
| aaa | bbb | ccc |
+-----+-----+-----+
| 111 | 222 | 333 |
| 444 | 555 | 666 |
| 777 | 888 | 999 |
+-----+-----+-----+
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTable(
				tt.fields.Name,
				tt.fields.Header,
				tt.fields.Records,
			)
			out := &bytes.Buffer{}
			if err := tr.Print(out, tt.args.mode); err != nil {
				t.Errorf("Print() error = %v", err)
				return
			}
			gotOut := out.String()
			if diff := cmp.Diff(gotOut, tt.wantOut); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestTablePrintJSON_NullDistinctFromEmpty(t *testing.T) {
	t.Parallel()
	// Regression test: a SQL NULL must render as JSON null, distinct
	// from an empty string. Column 0 (n) is a NULL cell; column 1 (e) is a real
	// empty string. Both render as "" in text output, so JSON is the only place
	// the difference is observable.
	tbl, err := NewTableFromCells("t", Header{"n", "e", "x"}, [][]Cell{
		{NewCell(nil), NewCell(""), NewCell("1")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	t.Run("json emits null for a NULL cell", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		if err := tbl.Print(out, PrintModeJSON); err != nil {
			t.Fatal(err)
		}
		want := "[\n  {\"n\":null,\"e\":\"\",\"x\":\"1\"}\n]\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("ndjson emits null for a NULL cell", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		if err := tbl.Print(out, PrintModeJSONL); err != nil {
			t.Fatal(err)
		}
		want := "{\"n\":null,\"e\":\"\",\"x\":\"1\"}\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestTablePrintJSONScalars(t *testing.T) {
	t.Parallel()

	// Every non-NULL cell here is TEXT, so string-looking numbers and booleans
	// must stay JSON strings. Only a cell the database reported as a native
	// number becomes a JSON number, which the repository-level tests cover.
	header := Header{"i", "f", "b", "n", "empty", "big", "lead", "text"}
	// Column 3 (n) is a SQL NULL; column 4 (empty) is a real empty string.
	tbl, err := NewTableFromCells("t", header, [][]Cell{{
		NewCell("42"), NewCell("-1.5"), NewCell("true"), NewCell(nil),
		NewCell(""), NewCell("123456789012345678901234567890"),
		NewCell("007"), NewCell("hello"),
	}})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	t.Run("json keeps string records as strings", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		if err := tbl.Print(out, PrintModeJSON); err != nil {
			t.Fatal(err)
		}
		want := "[\n  {\"i\":\"42\",\"f\":\"-1.5\",\"b\":\"true\",\"n\":null,\"empty\":\"\",\"big\":\"123456789012345678901234567890\",\"lead\":\"007\",\"text\":\"hello\"}\n]\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("ndjson keeps string records as strings", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		if err := tbl.Print(out, PrintModeJSONL); err != nil {
			t.Fatal(err)
		}
		want := "{\"i\":\"42\",\"f\":\"-1.5\",\"b\":\"true\",\"n\":null,\"empty\":\"\",\"big\":\"123456789012345678901234567890\",\"lead\":\"007\",\"text\":\"hello\"}\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestTablePrintJSONPreservesDatabaseTypes(t *testing.T) {
	t.Parallel()

	tbl, err := NewTableFromCells("query",
		Header{"integer", "real", "text_number", "text_bool", "padded", "null", "empty"},
		[][]Cell{{
			NewCell(int64(42)), NewCell(float64(1.5)), NewCell("123"),
			NewCell("true"), NewCell("00123"), NewCell(nil), NewCell(""),
		}})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	assertRow := func(t *testing.T, row map[string]any) {
		t.Helper()
		if got, ok := row["integer"].(float64); !ok || got != 42 {
			t.Errorf("integer = %#v (%T), want JSON number 42", row["integer"], row["integer"])
		}
		if got, ok := row["real"].(float64); !ok || got != 1.5 {
			t.Errorf("real = %#v (%T), want JSON number 1.5", row["real"], row["real"])
		}
		for _, name := range []string{"text_number", "text_bool", "padded", "empty"} {
			if _, ok := row[name].(string); !ok {
				t.Errorf("%s = %#v (%T), want JSON string", name, row[name], row[name])
			}
		}
		if row["text_number"] != "123" || row["text_bool"] != "true" || row["padded"] != "00123" {
			t.Errorf("string values changed: %#v", row)
		}
		if row["null"] != nil {
			t.Errorf("null = %#v, want nil", row["null"])
		}
	}

	t.Run("json", func(t *testing.T) {
		var rows []map[string]any
		var out bytes.Buffer
		if err := tbl.Print(&out, PrintModeJSON); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("decoded %d rows, want 1", len(rows))
		}
		assertRow(t, rows[0])
	})

	t.Run("ndjson", func(t *testing.T) {
		var row map[string]any
		var out bytes.Buffer
		if err := tbl.Print(&out, PrintModeJSONL); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		assertRow(t, row)
	})
}

func TestTableWithNamePreservesJSONMetadata(t *testing.T) {
	t.Parallel()
	table, err := NewTableFromCells("query_result", Header{"n", "text", "null"}, [][]Cell{
		{NewCell(int64(42)), NewCell("123"), NewCell(nil)},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}

	renamed := table.WithName("typed_values")
	if renamed.Name() != "typed_values" {
		t.Fatalf("name = %q, want typed_values", renamed.Name())
	}
	if !renamed.IsNull(0, 2) {
		t.Fatal("WithName lost SQL NULL metadata")
	}
	var rows []map[string]any
	var out bytes.Buffer
	if err := renamed.Print(&out, PrintModeJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if _, ok := rows[0]["n"].(float64); !ok {
		t.Errorf("n = %#v (%T), want JSON number", rows[0]["n"], rows[0]["n"])
	}
	if rows[0]["text"] != "123" || rows[0]["null"] != nil {
		t.Errorf("metadata after WithName = %#v, want text string and null", rows[0])
	}
}

func TestJSONScalarTokenUsesOriginalType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"integer", int64(42), "42"},
		{"real", float64(1.5), "1.5"},
		{"string number", "123", `"123"`},
		{"string boolean", "true", `"true"`},
		{"bytes", []byte("00123"), `"00123"`},
		{"nil", nil, "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := jsonScalarToken(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("jsonScalarToken(%#v) = %s, want %s", tc.value, got, tc.want)
			}
		})
	}

	if _, err := jsonScalarToken(make(chan int)); err == nil {
		t.Fatal("jsonScalarToken(chan int) returned nil error")
	}
}

func TestTableEqual(t *testing.T) {
	t.Parallel()

	type fields struct {
		name    string
		Header  Header
		Records []Record
	}
	type args struct {
		t2 *Table
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "table is equal",
			fields: fields{
				name:   "table_name",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{
				t2: NewTable(
					"table_name",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"111", "222", "333"},
						{"444", "555", "666"},
						{"777", "888", "999"},
					},
				),
			},
			want: true,
		},
		{
			name: "table is not equal (name)",
			fields: fields{
				name:   "table_name",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{
				t2: NewTable(
					"table_name2",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"111", "222", "333"},
						{"444", "555", "666"},
						{"777", "888", "999"},
					},
				),
			},
			want: false,
		},
		{
			name: "table is not equal (header)",
			fields: fields{
				name:   "table_name",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{
				t2: NewTable(
					"table_name",
					Header{"aaa", "bbb", "ccc", "ddd"},
					[]Record{
						{"111", "222", "333"},
						{"444", "555", "666"},
						{"777", "888", "999"},
					},
				),
			},
			want: false,
		},
		{
			name: "table is not equal (record)",
			fields: fields{
				name:   "table_name",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{
				t2: NewTable(
					"table_name",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"111", "222", "333"},
						{"444", "555", "666"},
						{"777", "888", "999"},
						{"aaa", "bbb", "ccc"},
					},
				),
			},
			want: false,
		},
		{
			name: "table is not equal (record value)",
			fields: fields{
				name:   "table_name",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{
				t2: NewTable(
					"table_name",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"111", "222", "333"},
						{"444", "555", "666"},
						{"777", "888", "99"},
					},
				),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := NewTable(
				tt.fields.name,
				tt.fields.Header,
				tt.fields.Records,
			)
			if got := tr.Equal(tt.args.t2); got != tt.want {
				t.Errorf("Table.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetColumnData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		records     []Record
		columnIndex int
		want        []string
	}{
		{
			name: "extract first column data",
			records: []Record{
				{"a", "b", "c"},
				{"d", "e", "f"},
				{"g", "h", "i"},
			},
			columnIndex: 0,
			want:        []string{"a", "d", "g"},
		},
		{
			name: "extract second column data",
			records: []Record{
				{"a", "b", "c"},
				{"d", "e", "f"},
				{"g", "h", "i"},
			},
			columnIndex: 1,
			want:        []string{"b", "e", "h"},
		},
		{
			name: "column index out of bounds",
			records: []Record{
				{"a", "b"},
				{"d", "e", "f"},
			},
			columnIndex: 2,
			want:        []string{"f"},
		},
		{
			name:        "empty records",
			records:     []Record{},
			columnIndex: 0,
			want:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tbl := NewTable("t", Header{"a", "b"}, tt.records)
			got := tbl.columnData(tt.columnIndex)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("columnData() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsAllNumeric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []string
		want   bool
	}{
		{
			name:   "all integers",
			values: []string{"1", "2", "3", "100"},
			want:   true,
		},
		{
			name:   "all floats",
			values: []string{"1.5", "2.0", "3.14", "100.99"},
			want:   true,
		},
		{
			name:   "numbers with commas",
			values: []string{"1,000", "2,500.50", "3,000"},
			want:   true,
		},
		{
			name:   "negative numbers",
			values: []string{"-1", "-2.5", "-100"},
			want:   true,
		},
		{
			name:   "mixed numeric and text",
			values: []string{"1", "abc", "3"},
			want:   false,
		},
		{
			name:   "all text",
			values: []string{"abc", "def", "ghi"},
			want:   false,
		},
		{
			name:   "empty values only",
			values: []string{"", "  ", ""},
			want:   true,
		},
		{
			name:   "empty slice",
			values: []string{},
			want:   false,
		},
		{
			name:   "numbers with spaces",
			values: []string{" 123 ", "  456.78  ", " -789 "},
			want:   true,
		},
		{
			name:   "invalid number format",
			values: []string{"1.2.3", "abc123", "12abc"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isAllNumeric(tt.values)
			if got != tt.want {
				t.Errorf("isAllNumeric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNumericValue(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"1": true, "-2.5": true, "1e3": true, "0": true,
		// Comma thousands separators and surrounding whitespace are stripped, so
		// the contract matches table-mode alignment.
		"1,000": true, "2,500.50": true, " 123 ": true,
		"abc": false, "": false, "NaN": false, "Inf": false, "1e400": false,
		// Go-specific float spellings are not treated as data numbers.
		"0x1p4": false, "1_000": false,
	}
	for in, want := range cases {
		if got := IsNumericValue(in); got != want {
			t.Errorf("IsNumericValue(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestTablePrintAlignsOnTheValuesNotTheColumnName pins that table mode decides
// alignment from what a column holds, not from what its name contains.
func TestTablePrintAlignsOnTheValuesNotTheColumnName(t *testing.T) {
	t.Parallel()

	// The values are the same in every column, so any difference in alignment
	// came from the name.
	tbl := NewTable("t", Header{"message", "package", "total_label", "id", "name"}, []Record{
		{"hi", "hi", "hi", "hi", "hi"},
		{"longer text", "longer text", "longer text", "longer text", "longer text"},
	})
	var buf bytes.Buffer
	if err := tbl.Print(&buf, PrintModeTable); err != nil {
		t.Fatalf("Print table: %v", err)
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if !strings.Contains(line, "hi") {
			continue
		}
		// Left-aligned "hi" is followed by padding before the next separator;
		// right-aligned "hi" is preceded by it.
		if strings.Contains(line, "|          hi ") {
			t.Errorf("a text column was right aligned because of its name:\n%s", buf.String())
		}
	}

	t.Run("a column of numbers is still right aligned", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"n"}, []Record{{"1"}, {"1000"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeTable); err != nil {
			t.Fatalf("Print table: %v", err)
		}
		if !strings.Contains(buf.String(), "|    1 |") {
			t.Errorf("a numeric column lost its right alignment:\n%s", buf.String())
		}
	})
}

// TestTablePrintEscaping covers the output-format bugs: CSV/TSV stdout must
// stay valid when values contain the delimiter, quotes, or newlines; LTSV must
// reject values it cannot represent losslessly; JSON/NDJSON must reject
// duplicate column names; and Markdown must keep a row on one physical line
// when a value contains a newline.
func TestTablePrintEscaping(t *testing.T) {
	t.Parallel()

	t.Run("CSV quotes a value containing a comma, quote, and newline", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"c"}, []Record{{"a,\"b\"\nc"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeCSV); err != nil {
			t.Fatalf("Print CSV: %v", err)
		}
		// Re-parse to confirm the round trip yields the original single field.
		r := csv.NewReader(bytes.NewReader(buf.Bytes()))
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatalf("output is not valid CSV: %v", err)
		}
		if len(rows) != 2 || len(rows[1]) != 1 || rows[1][0] != "a,\"b\"\nc" {
			t.Errorf("CSV did not round-trip, got rows=%v", rows)
		}
	})

	t.Run("TSV quotes a value containing a tab and newline", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"c"}, []Record{{"a\tb\nc"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeTSV); err != nil {
			t.Fatalf("Print TSV: %v", err)
		}
		r := csv.NewReader(bytes.NewReader(buf.Bytes()))
		r.Comma = '\t'
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatalf("output is not valid TSV: %v", err)
		}
		if len(rows) != 2 || rows[1][0] != "a\tb\nc" {
			t.Errorf("TSV did not round-trip, got rows=%v", rows)
		}
	})

	t.Run("a one-column row whose only value is empty survives a re-read", func(t *testing.T) {
		t.Parallel()

		// A blank line is not a record, so three printed rows would read back as two.
		for _, tt := range []struct {
			name  string
			mode  PrintMode
			comma rune
		}{
			{name: "CSV", mode: PrintModeCSV, comma: ','},
			{name: "TSV", mode: PrintModeTSV, comma: '\t'},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				tbl := NewTable("t", Header{"v"}, []Record{{"alice"}, {""}, {"bob"}})
				var buf bytes.Buffer
				if err := tbl.Print(&buf, tt.mode); err != nil {
					t.Fatalf("Print: %v", err)
				}
				r := csv.NewReader(bytes.NewReader(buf.Bytes()))
				r.Comma = tt.comma
				rows, err := r.ReadAll()
				if err != nil {
					t.Fatalf("output is not valid: %v", err)
				}
				want := [][]string{{"v"}, {"alice"}, {""}, {"bob"}}
				if diff := cmp.Diff(want, rows); diff != "" {
					t.Errorf("re-read mismatch (-want +got):\n%s\nprinted %q", diff, buf.String())
				}
			})
		}
	})

	t.Run("a lone empty column name survives a re-read", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{""}, []Record{{"x"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeCSV); err != nil {
			t.Fatalf("Print CSV: %v", err)
		}
		rows, err := csv.NewReader(bytes.NewReader(buf.Bytes())).ReadAll()
		if err != nil {
			t.Fatalf("output is not valid CSV: %v", err)
		}
		want := [][]string{{""}, {"x"}}
		if diff := cmp.Diff(want, rows); diff != "" {
			t.Errorf("re-read mismatch (-want +got):\n%s\nprinted %q", diff, buf.String())
		}
	})

	t.Run("a multi-column row of empty values keeps its delimiters", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"a", "b"}, []Record{{"", ""}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeCSV); err != nil {
			t.Fatalf("Print CSV: %v", err)
		}
		if got, want := buf.String(), "a,b\n,\n"; got != want {
			t.Errorf("Print CSV = %q, want %q", got, want)
		}
	})

	t.Run("LTSV rejects a value containing a tab", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"c"}, []Record{{"a\tb"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeLTSV); err == nil {
			t.Errorf("want error for tab in LTSV value, got output %q", buf.String())
		}
	})

	t.Run("LTSV rejects a value containing a newline", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"c"}, []Record{{"a\nb"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeLTSV); err == nil {
			t.Errorf("want error for newline in LTSV value, got output %q", buf.String())
		}
	})

	t.Run("CSV rejects duplicate column names", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"x", "x"}, []Record{{"1", "2"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeCSV); err == nil {
			t.Errorf("want error for duplicate CSV header, got output %q", buf.String())
		}
	})

	t.Run("TSV rejects duplicate column names", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"x", "x"}, []Record{{"1", "2"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeTSV); err == nil {
			t.Errorf("want error for duplicate TSV header, got output %q", buf.String())
		}
	})

	t.Run("CSV rejects column names the importer reads as one name", func(t *testing.T) {
		t.Parallel()
		// The importer compares names with surrounding whitespace removed and
		// ASCII case folded, so a header unique byte-for-byte can still be one
		// no sqly run could load back.
		for _, tt := range []struct {
			name   string
			header Header
		}{
			{name: "differ only by case", header: Header{"a", "A"}},
			{name: "differ only by surrounding whitespace", header: Header{"x", " x"}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				tbl := NewTable("t", tt.header, []Record{{"1", "2"}})
				var buf bytes.Buffer
				if err := tbl.Print(&buf, PrintModeCSV); err == nil {
					t.Errorf("want error for header %v, got output %q", tt.header, buf.String())
				}
			})
		}
	})

	t.Run("CSV keeps non-ASCII case pairs, which SQLite tells apart", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"ä", "Ä"}, []Record{{"1", "2"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeCSV); err != nil {
			t.Errorf("Print CSV: %v, want success for names SQLite tells apart", err)
		}
	})

	t.Run("LTSV rejects labels that differ only by case", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"a", "A"}, []Record{{"1", "2"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeLTSV); err == nil {
			t.Errorf("want error for case-folded duplicate LTSV labels, got output %q", buf.String())
		}
	})

	t.Run("JSON rejects duplicate column names", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"x", "x"}, []Record{{"1", "2"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeJSON); err == nil {
			t.Errorf("want error for duplicate JSON keys, got output %q", buf.String())
		}
	})

	t.Run("NDJSON rejects duplicate column names", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"x", "x"}, []Record{{"1", "2"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeJSONL); err == nil {
			t.Errorf("want error for duplicate NDJSON keys, got output %q", buf.String())
		}
	})

	t.Run("Markdown keeps a newline value on one physical row", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"x", "y"}, []Record{{"a\nb", "c|d"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeMarkdownTable); err != nil {
			t.Fatalf("Print markdown: %v", err)
		}
		lines := bytes.Split(bytes.TrimRight(buf.Bytes(), "\n"), []byte("\n"))
		// header, separator, and exactly one data row.
		if len(lines) != 3 {
			t.Fatalf("want 3 markdown lines, got %d: %q", len(lines), buf.String())
		}
		dataRow := string(lines[2])
		if !bytes.Contains(lines[2], []byte("a<br>b")) {
			t.Errorf("newline not rendered as <br>: %q", dataRow)
		}
		if !bytes.Contains(lines[2], []byte("c\\|d")) {
			t.Errorf("pipe not escaped: %q", dataRow)
		}
	})
}

// TestEnsureLTSVHeaderWritable verifies that LTSV output rejects column names that
// are not valid LTSV labels and rejects duplicate labels, so LTSV output stays
// valid and round-trippable.
func TestEnsureLTSVHeaderWritable(t *testing.T) {
	t.Parallel()

	valid := []Header{
		{"x"},
		{"user_name", "identifier"},
		{"a.b", "c-d", "e_f"},
		{"Col1", "Col2"},
	}
	for _, h := range valid {
		if err := EnsureLTSVHeaderWritable(h); err != nil {
			t.Errorf("EnsureLTSVHeaderWritable(%v) = %v, want nil", h, err)
		}
	}

	invalid := []struct {
		name   string
		header Header
	}{
		{"colon in label", Header{"foo:bar"}},
		{"space in label", Header{"foo bar"}},
		{"tab in label", Header{"foo\tbar"}},
		{"newline in label", Header{"foo\nbar"}},
		{"empty label", Header{""}},
		{"duplicate labels", Header{"x", "x"}},
		{"duplicate among valid", Header{"a", "b", "a"}},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := EnsureLTSVHeaderWritable(tt.header); err == nil {
				t.Errorf("EnsureLTSVHeaderWritable(%v) = nil, want an error", tt.header)
			}
		})
	}
}

// TestTablePrintLTSV_RejectsInvalidLabels verifies that printing a table as LTSV
// fails for an invalid or duplicate label rather than emitting ambiguous output.
func TestTablePrintLTSV_RejectsInvalidLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header Header
	}{
		{"label with colon", Header{"foo:bar"}},
		{"duplicate labels", Header{"x", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tbl := NewTable("t", tt.header, []Record{make(Record, len(tt.header))})
			var buf bytes.Buffer
			if err := tbl.Print(&buf, PrintModeLTSV); err == nil {
				t.Errorf("Print(LTSV) for header %v = nil error, want rejection; output=%q", tt.header, buf.String())
			}
		})
	}

	t.Run("valid labels still print", func(t *testing.T) {
		t.Parallel()
		tbl := NewTable("t", Header{"a", "b"}, []Record{{"1", "2"}})
		var buf bytes.Buffer
		if err := tbl.Print(&buf, PrintModeLTSV); err != nil {
			t.Fatalf("Print(LTSV) for a valid header returned error: %v", err)
		}
		if got := buf.String(); got != "a:1\tb:2\n" {
			t.Errorf("Print(LTSV) = %q, want %q", got, "a:1\tb:2\n")
		}
	})
}

var errForcedWrite = errors.New("forced write error")

type errorWriter struct{}

func (e errorWriter) Write(_ []byte) (n int, err error) {
	return 0, errForcedWrite
}

// TestTablePrint_WriteError verifies that Print propagates errors when writing fails
func TestTablePrint_WriteError(t *testing.T) {
	t.Parallel()

	tbl := NewTable("test", Header{"col1", "col2"}, []Record{{"val1", "val2"}})

	modes := []PrintMode{
		PrintModeMarkdownTable,
		PrintModeCSV,
		PrintModeTSV,
		PrintModeLTSV,
		PrintModeJSON,
		PrintModeJSONL,
	}

	for _, mode := range modes {
		t.Run(mode.String()+" propagates write error", func(t *testing.T) {
			t.Parallel()
			err := tbl.Print(errorWriter{}, mode)
			if !errors.Is(err, errForcedWrite) {
				t.Errorf("expected error %v, got %v", errForcedWrite, err)
			}
		})
	}
}

// TestTablePrintRefusesBeforeWritingAnything pins the guarantee a failing export
// owes a pipeline: nothing on the writer.
//
// LTSV checked each value as it wrote it, so a value it could not represent in
// row 2 left row 1 on stdout and then failed. JSON opened its array before
// encoding the first row, so an unencodable value left a bare "[" behind. Either
// way the reader on the other end had already taken a truncated document for a
// complete one, and only the exit code said otherwise.
func TestTablePrintRefusesBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		table  *Table
		mode   PrintMode
		reason string
	}{
		{
			name: "ltsv with a tab in the second row",
			table: NewTable("t", Header{"a"}, []Record{
				{"clean"},
				{"has\ttab"},
			}),
			mode:   PrintModeLTSV,
			reason: "a tab or newline",
		},
		{
			name: "ltsv with a newline in the second row",
			table: NewTable("t", Header{"a"}, []Record{
				{"clean"},
				{"has\nnewline"},
			}),
			mode:   PrintModeLTSV,
			reason: "a tab or newline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			err := tt.table.Print(&out, tt.mode)
			if err == nil {
				t.Fatal("Print succeeded, want a refusal")
			}
			if !strings.Contains(err.Error(), tt.reason) {
				t.Errorf("error = %v, want it to mention %q", err, tt.reason)
			}
			if out.Len() != 0 {
				t.Errorf("Print wrote %q before failing; a refused export writes nothing", out.String())
			}
		})
	}
}

// TestEnsureJSONWritableAgreesWithTheWriter keeps the pre-write check and the
// writer from drifting: a value one accepts and the other rejects would either
// pass validation and fail mid-document, or refuse a value that writes fine.
func TestEnsureJSONWritableAgreesWithTheWriter(t *testing.T) {
	t.Parallel()

	values := []any{
		nil, "text", []byte("bytes"), []byte{0xff}, true,
		int64(1), int(2), int32(3), uint64(4), float64(1.5), float32(2.5),
		math.Inf(1), math.NaN(), time.Now(),
		make(chan int), func() {}, struct{ A int }{1},
	}

	for _, v := range values {
		table, err := NewTableFromCells("t", Header{"v"}, [][]Cell{{NewCell(v)}})
		if err != nil {
			t.Fatalf("NewTableFromCells(%T): %v", v, err)
		}
		checkErr := table.EnsureJSONWritable()
		var buf bytes.Buffer
		writeErr := table.Print(&buf, PrintModeJSONL)
		if (checkErr == nil) != (writeErr == nil) {
			t.Errorf("%T: EnsureJSONWritable err = %v, but writing gave %v", v, checkErr, writeErr)
		}
		if checkErr != nil && buf.Len() != 0 {
			t.Errorf("%T: refused but wrote %q", v, buf.String())
		}
	}
}
