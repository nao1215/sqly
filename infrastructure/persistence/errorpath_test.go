package persistence

import (
	"context"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// covPerClosedHistoryRepo returns a history repository whose backing in-memory
// database has already been closed, so each method call hits its BeginTx error
// branch and must report the failure rather than panic. Each caller gets its own
// closed database so parallel subtests never share state.
func covPerClosedHistoryRepo(t *testing.T) *historyRepository {
	t.Helper()
	historyDB, cleanup, err := config.NewInMemHistoryDB()
	if err != nil {
		t.Fatal(err)
	}
	// cleanup closes the database; call it now so subsequent method calls fail.
	cleanup()
	repo, ok := NewHistoryRepository(historyDB).(*historyRepository)
	if !ok {
		t.Fatal("NewHistoryRepository did not return *historyRepository")
	}
	return repo
}

func TestHistoryRepository_ClosedDB_ReturnsErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("CreateTable returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covPerClosedHistoryRepo(t)
		if err := r.CreateTable(ctx); err == nil {
			t.Error("expected error from CreateTable on a closed DB, got nil")
		}
	})

	t.Run("Create returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covPerClosedHistoryRepo(t)
		// A valid single-row history table passes Valid() and reaches the DB call.
		input := model.Histories{model.NewHistory(0, "SELECT 1")}.ToTable()
		if err := r.Create(ctx, input); err == nil {
			t.Error("expected error from Create on a closed DB, got nil")
		}
	})

	t.Run("List returns error when DB is closed", func(t *testing.T) {
		t.Parallel()
		r := covPerClosedHistoryRepo(t)
		if _, err := r.List(ctx); err == nil {
			t.Error("expected error from List on a closed DB, got nil")
		}
	})
}
