package model

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTableIsSameHeaderColumnName(t *testing.T) {
	t.Parallel()

	type fields struct {
		Name    string
		Header  Header
		Records []Record
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "table has same header column",
			fields: fields{
				Name:    "table_name",
				Header:  Header{"aaa", "bbb", "ccc", "aa", "aaa"},
				Records: []Record{},
			},
			want: true,
		},
		{
			name: "table does not have same header column",
			fields: fields{
				Name:    "table_name",
				Header:  Header{"aaa", "bbb", "ccc"},
				Records: []Record{},
			},
			want: false,
		},
		{
			name: "table does not have header column",
			fields: fields{
				Name:    "table_name",
				Header:  Header{},
				Records: []Record{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := NewTable(
				tt.fields.Name,
				tt.fields.Header,
				tt.fields.Records,
			)
			if got := tr.IsSameHeaderColumnName(); got != tt.want {
				t.Errorf("Table.IsSameHeaderColumnName() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			name: "ndjson mode",
			p:    PrintModeNDJSON,
			want: "ndjson",
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

func TestTableValid(t *testing.T) {
	type fields struct {
		Name    string
		Header  Header
		Records []Record
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "valid",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			wantErr: false,
		},
		{
			name: "table name is empty",
			fields: fields{
				Name:   "",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			wantErr: true,
		},
		{
			name: "header is empty",
			fields: fields{
				Name:   "invalid_table",
				Header: Header{},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			wantErr: true,
		},
		{
			name: "record is empty",
			fields: fields{
				Name:    "invalid_table",
				Header:  Header{"aaa", "bbb", "ccc"},
				Records: []Record{},
			},
			wantErr: true,
		},
		{
			name: "header has same name colomn",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "aaa"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTable(
				tt.fields.Name,
				tt.fields.Header,
				tt.fields.Records,
			)
			if err := tr.Valid(); (err != nil) != tt.wantErr {
				t.Errorf("Table.Valid() error = %v, wantErr %v", err, tt.wantErr)
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
			args: args{PrintModeNDJSON},
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
			args:    args{PrintModeNDJSON},
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
			args: args{PrintModeNDJSON},
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
	// from an empty string. The null mask marks column 0 (n) as NULL; column 1
	// (e) is a real empty string.
	tbl := NewTable("t", Header{"n", "e", "x"}, []Record{{"", "", "1"}})
	tbl.SetNulls([][]bool{{true, false, false}})

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
		if err := tbl.Print(out, PrintModeNDJSON); err != nil {
			t.Fatal(err)
		}
		want := "{\"n\":null,\"e\":\"\",\"x\":\"1\"}\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestTablePrintJSON_TypedScalars(t *testing.T) {
	t.Parallel()

	// In typed mode a cell that is a canonical JSON number is emitted as a native
	// number (large integers verbatim, so no precision loss or scientific
	// notation), "true"/"false" become native booleans, a SQL NULL becomes null,
	// and everything else stays a JSON string.
	header := Header{"i", "f", "b", "n", "empty", "big", "lead", "text"}
	rec := Record{"42", "-1.5", "true", "", "", "123456789012345678901234567890", "007", "hello"}
	tbl := NewTable("t", header, []Record{rec})
	// Column 3 (n) is a SQL NULL; column 4 (empty) is a real empty string.
	tbl.SetNulls([][]bool{{false, false, false, true, false, false, false, false}})
	tbl.SetJSONTyped(true)

	t.Run("typed json emits native scalars and keeps non-numbers as strings", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		if err := tbl.Print(out, PrintModeJSON); err != nil {
			t.Fatal(err)
		}
		want := "[\n  {\"i\":42,\"f\":-1.5,\"b\":true,\"n\":null,\"empty\":\"\",\"big\":123456789012345678901234567890,\"lead\":\"007\",\"text\":\"hello\"}\n]\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("typed ndjson emits native scalars", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		if err := tbl.Print(out, PrintModeNDJSON); err != nil {
			t.Fatal(err)
		}
		want := "{\"i\":42,\"f\":-1.5,\"b\":true,\"n\":null,\"empty\":\"\",\"big\":123456789012345678901234567890,\"lead\":\"007\",\"text\":\"hello\"}\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})

	t.Run("default mode keeps the legacy string contract", func(t *testing.T) {
		t.Parallel()
		plain := NewTable("t", header, []Record{rec})
		plain.SetNulls([][]bool{{false, false, false, true, false, false, false, false}})
		out := &bytes.Buffer{}
		if err := plain.Print(out, PrintModeJSON); err != nil {
			t.Fatal(err)
		}
		want := "[\n  {\"i\":\"42\",\"f\":\"-1.5\",\"b\":\"true\",\"n\":null,\"empty\":\"\",\"big\":\"123456789012345678901234567890\",\"lead\":\"007\",\"text\":\"hello\"}\n]\n"
		if diff := cmp.Diff(out.String(), want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestIsCanonicalJSONNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want bool
	}{
		{"0", true},
		{"-0", true},
		{"42", true},
		{"-42", true},
		{"3.14", true},
		{"-3.14", true},
		{"1e10", true},
		{"1E10", true},
		{"1.5e-3", true},
		{"-2.0E+8", true},
		{"123456789012345678901234567890", true},
		{"", false},
		{"007", false},
		{"+1", false},
		{"1.", false},
		{".5", false},
		{"1e", false},
		{"1.2.3", false},
		{"0x10", false},
		{" 1", false},
		{"1 ", false},
		{"NaN", false},
		{"Infinity", false},
		{"-", false},
		{"abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := isCanonicalJSONNumber(tt.in); got != tt.want {
				t.Errorf("isCanonicalJSONNumber(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
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

			got := getColumnData(tt.records, tt.columnIndex)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getColumnData() mismatch (-want +got):\n%s", diff)
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
		if err := tbl.Print(&buf, PrintModeNDJSON); err == nil {
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
