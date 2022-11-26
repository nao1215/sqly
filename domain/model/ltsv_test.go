// Package model defines Data Transfer Object (Entity, Value Object)
package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLTSV_IsLabelEmpty(t *testing.T) {
	type fields struct {
		Name    string
		Label   Label
		Records []Record
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "label is empty",
			fields: fields{
				Name:  "ltsv",
				Label: Label{},
			},
			want: true,
		},
		{
			name: "label is not empty",
			fields: fields{
				Name:  "ltsv",
				Label: Label{"aaa", "bbb", "ccc"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LTSV{
				Name:    tt.fields.Name,
				Label:   tt.fields.Label,
				Records: tt.fields.Records,
			}
			if got := l.IsLabelEmpty(); got != tt.want {
				t.Errorf("LTSV.IsLabelEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLTSV_SetLabel(t *testing.T) {
	type fields struct {
		Name    string
		Label   Label
		Records []Record
	}
	type args struct {
		label Label
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Label
	}{
		{
			name: "set label",
			fields: fields{
				Name:  "ltsv",
				Label: Label{"aaa"},
			},
			args: args{
				label: Label{"bbb", "ccc"},
			},
			want: Label{"aaa", "bbb", "ccc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LTSV{
				Name:    tt.fields.Name,
				Label:   tt.fields.Label,
				Records: tt.fields.Records,
			}
			l.SetLabel(tt.args.label)
			if diff := cmp.Diff(l.Label, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestLTSV_SetRecord(t *testing.T) {
	type fields struct {
		Name    string
		Label   Label
		Records []Record
	}
	type args struct {
		record Record
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []Record
	}{
		{
			name: "set record",
			fields: fields{
				Name:    "ltsv",
				Records: []Record{{"aaa", "bbb"}},
			},
			args: args{
				record: Record{"ccc", "ddd"},
			},
			want: []Record{
				{"aaa", "bbb"},
				{"ccc", "ddd"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LTSV{
				Name:    tt.fields.Name,
				Label:   tt.fields.Label,
				Records: tt.fields.Records,
			}
			l.SetRecord(tt.args.record)
			if diff := cmp.Diff(l.Records, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestLTSV_ToTable(t *testing.T) {
	type fields struct {
		Name    string
		Label   Label
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
				Name:  "test.ltsv",
				Label: Label{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
					{"jjj", "kkk", "lll"},
				},
			},
			want: &Table{
				Name:   "test",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
					{"jjj", "kkk", "lll"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LTSV{
				Name:    tt.fields.Name,
				Label:   tt.fields.Label,
				Records: tt.fields.Records,
			}
			got := l.ToTable()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
