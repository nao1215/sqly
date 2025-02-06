package model

import (
	"bytes"
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
			name: "json mode",
			p:    PrintModeJSON,
			want: "json",
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
			name: "print json",
			fields: fields{
				Name:   "valid_table",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"111", "222", "333"},
					{"444", "555", "666"},
					{"777", "888", "999"},
				},
			},
			args: args{PrintModeJSON},
			wantOut: `[
   {
      "aaa": "111",
      "bbb": "222",
      "ccc": "333"
   },
   {
      "aaa": "444",
      "bbb": "555",
      "ccc": "666"
   },
   {
      "aaa": "777",
      "bbb": "888",
      "ccc": "999"
   }
]
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
			tr.Print(out, tt.args.mode)
			gotOut := out.String()
			if diff := cmp.Diff(gotOut, tt.wantOut); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
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
