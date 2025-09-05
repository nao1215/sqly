package memory

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

func TestSqlite3RepositoryCreateTable(t *testing.T) {
	t.Run("Create table", func(t *testing.T) {
		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)
		want := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)

		if err := r.CreateTable(context.Background(), want); err != nil {
			t.Fatal(err)
		}

		got, err := r.TablesName(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if len(got) == 0 {
			t.Fatal("falied to create table (no table in memory db)")
		}

		if diff := cmp.Diff(got[0].Name(), want.Name()); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}

		got2, err := r.List(context.Background(), "sample")
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got2.Header(), want.Header()); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}

		got3, err := r.Header(context.Background(), "sample")
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got3.Header(), want.Header()); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestSqlite3RepositoryInsert(t *testing.T) {
	t.Run("INSERT data", func(t *testing.T) {
		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)
		table := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)
		if err := r.CreateTable(context.Background(), table); err != nil {
			t.Fatal(err)
		}

		input := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)
		if err := r.Insert(context.Background(), input); err != nil {
			t.Fatal(err)
		}

		if _, err := r.Exec(context.Background(), "DELETE FROM sample WHERE aaa = '111'"); err != nil {
			t.Fatal(err)
		}

		got, err := r.Query(context.Background(), "SELECT * FROM sample")
		if err != nil {
			t.Fatal(err)
		}

		want := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)
		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestExtractTableName(t *testing.T) {
	t.Parallel()

	type args struct {
		query string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "extract table name",
			args: args{
				query: "SELECT * FROM `sample_table`",
			},
			want: "sample_table",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := extractTableName(tt.args.query); got != tt.want {
				t.Errorf("extractTableName() = %v, want %v", got, tt.want)
			}
		})
	}
}
