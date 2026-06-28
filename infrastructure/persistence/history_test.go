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

	t.Run("auto-assigns sequential ids when the caller supplies none", func(t *testing.T) {
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

		// recordUserRequest now passes id 0 and relies on AUTOINCREMENT.
		for _, req := range []string{"first", "second", "third"} {
			in := model.Histories{model.NewHistory(0, req)}.ToTable()
			if err := r.Create(context.Background(), in); err != nil {
				t.Fatal(err)
			}
		}

		want := model.Histories{
			model.NewHistory(1, "first"),
			model.NewHistory(2, "second"),
			model.NewHistory(3, "third"),
		}
		got, err := r.List(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("value is mismatch (-got +want):\n%s", diff)
		}
	})
}

func BenchmarkHistoryRepositoryCreate(b *testing.B) {
	historyDB, cleanup, err := config.NewInMemHistoryDB()
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup()

	r := NewHistoryRepository(historyDB)
	if err := r.CreateTable(context.Background()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		in := model.Histories{model.NewHistory(0, "SELECT 1")}.ToTable()
		if err := r.Create(context.Background(), in); err != nil {
			b.Fatal(err)
		}
	}
}
