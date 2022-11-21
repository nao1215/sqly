// Package model defines Data Transfer Object (Entity, Value Object)
package model

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCSV_IsHeaderEmpty(t *testing.T) {
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
			name: "has header",
			fields: fields{
				Name:    "test.csv",
				Header:  Header{"aaa"},
				Records: nil,
			},
			want: false,
		},
		{
			name: "empty header",
			fields: fields{
				Name:    "test.csv",
				Header:  Header{},
				Records: nil,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			if got := c.IsHeaderEmpty(); got != tt.want {
				t.Errorf("CSV.IsHeaderEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSV_SetHeader(t *testing.T) {
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
				Name:    "test.csv",
				Header:  Header{"aaa", "bbb"},
				Records: nil,
			},
			args: args{
				header: Header{"ccc", "ddd"},
			},
			want: Header{"aaa", "bbb", "ccc", "ddd"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			c.SetHeader(tt.args.header)

			if diff := cmp.Diff(c.Header, tt.want); diff != "" {
				t.Errorf("mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestCSV_SetRecord(t *testing.T) {
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
			name: "set record",
			fields: fields{
				Name:    "test.csv",
				Header:  Header{"aaa", "bbb"},
				Records: []Record{{"aaa", "bbb"}},
			},
			args: args{
				record: Record{"ccc", "ddd"},
			},
			want: []Record{{"aaa", "bbb"}, {"ccc", "ddd"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			c.SetRecord(tt.args.record)

			if diff := cmp.Diff(c.Records, tt.want); diff != "" {
				t.Errorf("mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestCSV_ToTable(t *testing.T) {
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
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
			want: &Table{
				Name:   "test",
				Header: Header{"aaa", "bbb", "ccc"},
				Records: []Record{
					{"ddd", "eee", "fff"},
					{"ggg", "hhh", "iii"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CSV{
				Name:    tt.fields.Name,
				Header:  tt.fields.Header,
				Records: tt.fields.Records,
			}
			if got := c.ToTable(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CSV.ToTable() = %v, want %v", got, tt.want)
			}
		})
	}
}
