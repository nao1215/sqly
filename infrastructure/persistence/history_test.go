// Package persistence handle sqlite3, csv
package persistence

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

func TestHistoryRepositoryCreateTable(t *testing.T) {
	t.Parallel()

	t.Run("create history table and check history", func(t *testing.T) {
		t.Parallel()

		historyDB, cleanup, err := config.NewInMemHistoryDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewHistoryRepository(historyDB)

		if err := r.CreateTable(context.Background()); err != nil {
			t.Fatal(err)
		}

		input := model.Histories{model.NewHistory(1, "test")}.ToTable()
		if err := r.Create(context.Background(), input); err != nil {
			t.Fatal(err)
		}

		want := model.Histories{model.NewHistory(1, "test")}
		got, err := r.List(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}
