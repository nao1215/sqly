package model

import "testing"

func TestTable_IsSameHeaderColumnName(t *testing.T) {
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
			tr := &Table{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			if got := tr.IsSameHeaderColumnName(); got != tt.want {
				t.Errorf("Table.IsSameHeaderColumnName() = %v, want %v", got, tt.want)
			}
		})
	}
}
