package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHistoriesToTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		h    Histories
		want *Table
	}{
		{
			name: "history to table",
			h: Histories{
				NewHistory(1, "request1"),
				NewHistory(2, "request2"),
				NewHistory(3, "request3"),
			},
			want: NewTable(
				"history",
				Header{"id", "request"},
				[]Record{
					{"1", "request1"},
					{"2", "request2"},
					{"3", "request3"},
				},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.h.ToTable()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestHistoriesToStringList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		h    Histories
		want []string
	}{
		{
			name: "history to table",
			h: Histories{
				NewHistory(1, "request1"),
				NewHistory(2, "request2"),
				NewHistory(3, "request3"),
			},
			want: []string{"request1", "request2", "request3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.h.ToStringList()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
