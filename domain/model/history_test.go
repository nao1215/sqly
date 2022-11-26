package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHistories_ToTable(t *testing.T) {
	tests := []struct {
		name string
		h    Histories
		want *Table
	}{
		{
			name: "history to table",
			h: Histories{
				&History{
					ID:      1,
					Request: "request1",
				},
				&History{
					ID:      2,
					Request: "request2",
				},

				&History{
					ID:      3,
					Request: "request3",
				},
			},
			want: &Table{
				Name: "history",
				Header: Header{
					"id", "request",
				},
				Records: []Record{
					{"1", "request1"},
					{"2", "request2"},
					{"3", "request3"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.h.ToTable()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestHistories_ToStringList(t *testing.T) {
	tests := []struct {
		name string
		h    Histories
		want []string
	}{
		{
			name: "history to table",
			h: Histories{
				&History{
					ID:      1,
					Request: "request1",
				},
				&History{
					ID:      2,
					Request: "request2",
				},

				&History{
					ID:      3,
					Request: "request3",
				},
			},
			want: []string{"request1", "request2", "request3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.h.ToStringList()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
