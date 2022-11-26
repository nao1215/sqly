package memory

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

func Test_sqlite3Repository_CreateTable(t *testing.T) {
	t.Run("Create table", func(t *testing.T) {
		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)
		want := model.Table{
			Name:   "sample",
			Header: model.Header{"aaa", "bbb", "ccc"},
			Records: []model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		}

		if err := r.CreateTable(context.Background(), &want); err != nil {
			t.Fatal(err)
		}

		got, err := r.TablesName(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if len(got) == 0 {
			t.Fatal("falied to create table (no table in memory db)")
		}

		if diff := cmp.Diff(got[0].Name, want.Name); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}

		got2, err := r.List(context.Background(), "sample")
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got2.Header, want.Header); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}

		got3, err := r.Header(context.Background(), "sample")
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got3.Header, want.Header); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}
	})
}

func Test_sqlite3Repository_Insert(t *testing.T) {
	t.Run("INSERT data", func(t *testing.T) {
		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)
		table := model.Table{
			Name:   "sample",
			Header: model.Header{"aaa", "bbb", "ccc"},
			Records: []model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		}

		if err := r.CreateTable(context.Background(), &table); err != nil {
			t.Fatal(err)
		}

		input := model.Table{
			Name:   "sample",
			Header: model.Header{"aaa", "bbb", "ccc"},
			Records: []model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		}
		if err := r.Insert(context.Background(), &input); err != nil {
			t.Fatal(err)
		}

		if _, err := r.Exec(context.Background(), "DELETE FROM sample WHERE aaa = '111'"); err != nil {
			t.Fatal(err)
		}

		got, err := r.Query(context.Background(), "SELECT * FROM sample")
		if err != nil {
			t.Fatal(err)
		}

		want := &model.Table{
			Header: model.Header{"aaa", "bbb", "ccc"},
			Records: []model.Record{
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}
	})
}
