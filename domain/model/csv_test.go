// Package model defines Data Transfer Object (Entity, Value Object)
package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCSVToTable(t *testing.T) {
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
			name: "convert csv to table",
			fields: fields{
				Name:   "test.csv",
				Header: NewHeader([]string{"aaa", "bbb", "ccc"}),
				Records: []Record{
					NewRecord([]string{"ddd", "eee", "fff"}),
					NewRecord([]string{"ggg", "hhh", "iii"}),
				},
			},
			want: NewTable(
				"test",
				Header{"aaa", "bbb", "ccc"},
				[]Record{
					NewRecord([]string{"ddd", "eee", "fff"}),
					NewRecord([]string{"ggg", "hhh", "iii"}),
				},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := NewCSV(tt.fields.Name, tt.fields.Header, tt.fields.Records)
			got := c.ToTable()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
