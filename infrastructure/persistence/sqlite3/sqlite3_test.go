// Package sqlite3 handle sqlite3 database.
package sqlite3

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
			name: "success to generate create table statement",
			args: args{
				t: &model.Table{
					Name:   "test",
					Header: model.Header{"abc", "def", "ghj"},
				},
			},
			want: "CREATE TABLE `test`(`abc`, `def`, `ghj`);",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateCreateTableStatement(tt.args.t); got != tt.want {
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
					Header: model.Header{"abc", "def", "ghj"},
				},
			},
			want: "INSERT INTO `test`VALUES(?, ?, ?);",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateInsertStatement(tt.args.t.Name, tt.args.t.Records[0]); got != tt.want {
				t.Errorf("generateInsertStatement() = %v, want %v", got, tt.want)
			}
		})
	}
}
