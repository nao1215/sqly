// Package model defines Data Transfer Object (Entity, Value Object)
package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTSVToTable(t *testing.T) {
	t.Parallel()
	type fields struct {
		Name    string
		Header  Header
		Records []Record
	}
	tests := []struct {
		name   string
		fields fields
		want   *Table
	}{
		{
			name: "convert tsv to table",
			fields: fields{
				Name:   "test.tsv",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
					{"jjj", "kkk", "lll"},
				},
			},
			want: NewTable(
				"test",
				Header{"aaa", "bbb", "ccc"},
				[]Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
					{"jjj", "kkk", "lll"},
				},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := NewTSV(
				tt.fields.Name,
				tt.fields.Header,
				tt.fields.Records,
			)
			got := tr.ToTable()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
