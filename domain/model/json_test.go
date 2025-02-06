package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestJSONToTable(t *testing.T) {
	t.Parallel()

	type fields struct {
		Name string
		JSON []map[string]interface{}
	}
	tests := []struct {
		name   string
		fields fields
		want   *Table
	}{
		{
			name: "convert json to table",
			fields: fields{
				Name: "test.json",
				JSON: []map[string]interface{}{
					{
						"id":   "1",
						"data": "test-data1",
						"date": "2022-11-23",
					},
					{
						"id":   "2",
						"data": "test-data2",
						"date": "2022-11-24",
					},
					{
						"id":   "3",
						"data": "test-data3",
						"date": "2022-11-25",
					},
				},
			},
			want: NewTable(
				"test",
				Header{"data", "date", "id"},
				[]Record{
					{"test-data1", "2022-11-23", "1"},
					{"test-data2", "2022-11-24", "2"},
					{"test-data3", "2022-11-25", "3"},
				},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			j := NewJSON(tt.fields.Name, tt.fields.JSON)
			got := j.ToTable()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
