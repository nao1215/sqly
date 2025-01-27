package infrastructure

import (
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

func Test_generateCreateTableStatement(t *testing.T) {
	type args struct {
		t *model.Table
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "success to generate create table statement with no records",
			args: args{
				t: &model.Table{
					Name:   "test",
					Header: model.Header{"abc", "def", "ghj"},
				},
			},
			want: "CREATE TABLE `test`(`abc` TEXT, `def` TEXT, `ghj` TEXT);",
		},
		{
			name: "success to generate create table statement with records",
			args: args{
				t: &model.Table{
					Name:   "test",
					Header: model.Header{"id", "name", "number_and_string"},
					Records: []model.Record{
						{"1", "name1", "1"},
						{"2", "name2", "a"},
						{"3", "name3", "3"},
					},
				},
			},
			want: "CREATE TABLE `test`(`id` INTEGER, `name` TEXT, `number_and_string` TEXT);",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateCreateTableStatement(tt.args.t); got != tt.want {
				t.Errorf("createTableQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateInsertStatement(t *testing.T) {
	type args struct {
		t *model.Table
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "success to generate insert recored statement",
			args: args{
				t: &model.Table{
					Name:   "test",
					Header: model.Header{"a_header", "b_header", "c_header"},
					Records: []model.Record{
						{"a", "b", "c"},
					},
				},
			},
			want: "INSERT INTO `test` VALUES ('a', 'b', 'c');",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateInsertStatement(tt.args.t.Name, tt.args.t.Records[0]); got != tt.want {
				t.Errorf("generateInsertStatement() = %v, want %v", got, tt.want)
			}
		})
	}
}
