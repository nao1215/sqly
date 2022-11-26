// Package persistence handle sqlite3, csv
package persistence

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

func Test_historyRepository_CreateTable(t *testing.T) {
	t.Run("create history table and check history", func(t *testing.T) {
		c, err := config.NewConfig()
		if err != nil {
			t.Fatal(err)
		}

		c.HistoryDBPath = filepath.Join(t.TempDir(), "history.db")
		historyDB, cleanup, err := config.NewHistoryDB(c)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewHistoryRepository(historyDB)

		if err := r.CreateTable(context.Background()); err != nil {
			t.Fatal(err)
		}

		history := model.History{
			ID:      1,
			Request: "test",
		}
		input := model.Histories{&history}.ToTable()

		if err := r.Create(context.Background(), input); err != nil {
			t.Fatal(err)
		}

		want := model.Histories{&history}
		got, err := r.List(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}
