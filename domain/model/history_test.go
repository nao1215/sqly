package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHistoriesToStringList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		h    Histories
		want []string
	}{
		{
			name: "each entry contributes its request, in order",
			h: Histories{
				NewHistory("request1"),
				NewHistory("request2"),
				NewHistory("request3"),
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
