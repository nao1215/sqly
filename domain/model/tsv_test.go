// Package model defines Data Transfer Object (Entity, Value Object)
package model

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTSV_IsHeaderEmpty(t *testing.T) {
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
			name: "empty header",
			fields: fields{
				Header: Header{},
			},
			want: true,
		},
		{
			name: "not empty header",
			fields: fields{
				Header: Header{"aaa", "bbb", "ccc"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			if got := tr.IsHeaderEmpty(); got != tt.want {
				t.Errorf("TSV.IsHeaderEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTSV_SetHeader(t *testing.T) {
	type fields struct {
		Name    string
		Header  Header
		Records []Record
	}
	type args struct {
		header Header
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Header
	}{
		{
			name: "set header",
			fields: fields{
				Header: Header{"aaa"},
			},
			args: args{Header{"bbb", "ccc", "ddd"}},
			want: Header{"aaa", "bbb", "ccc", "ddd"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			tr.SetHeader(tt.args.header)
			if diff := cmp.Diff(tr.Header, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestTSV_SetRecord(t *testing.T) {
	type fields struct {
		Name    string
		Header  Header
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
			name: "set header",
			fields: fields{
				Records: []Record{{"aaa", "bbb", "ccc"}},
			},
			args: args{
				Record{"ddd", "eee", "fff"},
			},
			want: []Record{
				{"aaa", "bbb", "ccc"},
				{"ddd", "eee", "fff"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			tr.SetRecord(tt.args.record)
			if diff := cmp.Diff(tr.Records, tt.want); diff != "" {
				t.Errorf("value is mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestTSV_ToTable(t *testing.T) {
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
			tr := &TSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			if got := tr.ToTable(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TSV.ToTable() = %v, want %v", got, tt.want)
			}
		})
	}
}
