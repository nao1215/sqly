package interactor

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/domain/model"
	infrastructure "github.com/nao1215/sqly/infrastructure/mock"
	"go.uber.org/mock/gomock"
)

func TestHistoryInteractorCreateTable(t *testing.T) {
	t.Parallel()

	t.Run("success to create table", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		historyRepo := infrastructure.NewMockHistoryRepository(ctrl)
		historyRepo.EXPECT().CreateTable(context.Background()).Return(nil)

		historyInteractor := NewHistoryInteractor(historyRepo)
		err := historyInteractor.CreateTable(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("failed to create table", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		historyRepo := infrastructure.NewMockHistoryRepository(ctrl)
		someErr := errors.New("failed to create table")
		historyRepo.EXPECT().CreateTable(context.Background()).Return(someErr)

		historyInteractor := NewHistoryInteractor(historyRepo)
		err := historyInteractor.CreateTable(context.Background())
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestHistoryInteractorCreate(t *testing.T) {
	t.Parallel()

	t.Run("success to create history record", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		historyRepo := infrastructure.NewMockHistoryRepository(ctrl)
		history := model.History{
			ID:      1,
			Request: "create table",
		}
		historyRepo.EXPECT().Create(context.Background(), model.Histories{&history}.ToTable()).Return(nil)

		historyInteractor := NewHistoryInteractor(historyRepo)
		err := historyInteractor.Create(context.Background(), history)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("failed to create history record", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		historyRepo := infrastructure.NewMockHistoryRepository(ctrl)
		history := model.History{
			ID:      1,
			Request: "create table",
		}
		someErr := errors.New("failed to create history record")
		historyRepo.EXPECT().Create(context.Background(), model.Histories{&history}.ToTable()).Return(someErr)

		historyInteractor := NewHistoryInteractor(historyRepo)
		err := historyInteractor.Create(context.Background(), history)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestHistoryInteractorList(t *testing.T) {
	t.Parallel()

	t.Run("success to list history records", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		historyRepo := infrastructure.NewMockHistoryRepository(ctrl)
		histories := model.Histories{
			{ID: 1, Request: "create table"},
			{ID: 2, Request: "drop table"},
		}
		historyRepo.EXPECT().List(context.Background()).Return(histories, nil)

		historyInteractor := NewHistoryInteractor(historyRepo)
		got, err := historyInteractor.List(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		want := histories
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch: (-want +got)\n%s", diff)
		}
	})

	t.Run("failed to list history records", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		historyRepo := infrastructure.NewMockHistoryRepository(ctrl)
		someErr := errors.New("failed to list history records")
		historyRepo.EXPECT().List(context.Background()).Return(nil, someErr)

		historyInteractor := NewHistoryInteractor(historyRepo)
		_, err := historyInteractor.List(context.Background())
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
