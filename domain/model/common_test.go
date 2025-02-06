package model

import (
	"testing"
)

func TestHeaderEqual(t *testing.T) {
	t.Parallel()

	type args struct {
		h2 Header
	}
	tests := []struct {
		name string
		h    Header
		args args
		want bool
	}{
		{
			name: "header1 and header2 are equal",
			h:    Header{"aaa", "bbb", "ccc"},
			args: args{h2: Header{"aaa", "bbb", "ccc"}},
			want: true,
		},
		{
			name: "header1 and header2 are not equal",
			h:    Header{"aaa", "bbb", "ccc"},
			args: args{h2: Header{"aaa", "bbb", "ddd"}},
			want: false,
		},
		{
			name: "header1 is longer than header2",
			h:    Header{"aaa", "bbb", "ccc"},
			args: args{h2: Header{"aaa", "bbb"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.h.Equal(tt.args.h2); got != tt.want {
				t.Errorf("Header.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecordEqual(t *testing.T) {
	t.Parallel()
	type args struct {
		r2 Record
	}
	tests := []struct {
		name string
		r    Record
		args args
		want bool
	}{
		{
			name: "record1 and record2 are equal",
			r:    Record{"aaa", "bbb", "ccc"},
			args: args{r2: Record{"aaa", "bbb", "ccc"}},
			want: true,
		},
		{
			name: "record1 and record2 are not equal",
			r:    Record{"aaa", "bbb", "ccc"},
			args: args{r2: Record{"aaa", "bbb", "ddd"}},
			want: false,
		},
		{
			name: "record1 is longer than record2",
			r:    Record{"aaa", "bbb", "ccc"},
			args: args{r2: Record{"aaa", "bbb"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.r.Equal(tt.args.r2); got != tt.want {
				t.Errorf("Record.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}
