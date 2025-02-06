package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExcelEqual(t *testing.T) {
	t.Parallel()

	type fields struct {
		name    string
		header  Header
		records []Record
	}
	type args struct {
		e2 *Excel
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "excel1 and excel2 are equal",
			fields: fields{
				name:   "test",
				header: Header{"aaa", "bbb", "ccc"},
				records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
			args: args{
				e2: NewExcel(
					"test",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"ddd", "eee", "fff"},
						{"ggg", "hhh", "iii"},
					},
				),
			},
			want: true,
		},
		{
			name: "excel1 and excel2 are not equal",
			fields: fields{
				name:   "test",
				header: Header{"aaa", "bbb", "ccc"},
				records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
			args: args{
				e2: NewExcel(
					"test",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"ddd", "eee", "fff"},
						{"ggg", "hhh", "jjj"},
					},
				),
			},
			want: false,
		},
		{
			name: "excel1 and excel2 are not equal. name is different",
			fields: fields{
				name:   "test",
				header: Header{"aaa", "bbb", "ccc"},
				records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
			args: args{
				e2: NewExcel(
					"test2",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"ddd", "eee", "fff"},
						{"ggg", "hhh", "iii"},
					},
				),
			},
			want: false,
		},
		{
			name: "excel1 and excel2 are not equal. header is different",
			fields: fields{
				name:   "test",
				header: Header{"aaa", "bbb", "ccc"},
				records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
			args: args{
				e2: NewExcel(
					"test",
					Header{"aaa", "bbb", "ccc", "ddd"},
					[]Record{
						{"ddd", "eee", "fff"},
						{"ggg", "hhh", "iii"},
					},
				),
			},
			want: false,
		},
		{
			name: "excel1 and excel2 are not equal. records is different",
			fields: fields{
				name:   "test",
				header: Header{"aaa", "bbb", "ccc"},
				records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
			args: args{
				e2: NewExcel(
					"test",
					Header{"aaa", "bbb", "ccc"},
					[]Record{
						{"ggg", "hhh", "jjj"},
					},
				),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := NewExcel(
				tt.fields.name,
				tt.fields.header,
				tt.fields.records,
			)
			if got := e.Equal(tt.args.e2); got != tt.want {
				t.Errorf("Excel.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExcelToTable(t *testing.T) {
	t.Parallel()

	type fields struct {
		name    string
		header  Header
		records []Record
	}
	tests := []struct {
		name   string
		fields fields
		want   *Table
	}{
		{
			name: "convert excel to table",
			fields: fields{
				name:   "test",
				header: Header{"aaa", "bbb", "ccc"},
				records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
			want: NewTable(
				"test",
				Header{"aaa", "bbb", "ccc"},
				[]Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := NewExcel(
				tt.fields.name,
				tt.fields.header,
				tt.fields.records,
			)
			if diff := cmp.Diff(e.ToTable(), tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
