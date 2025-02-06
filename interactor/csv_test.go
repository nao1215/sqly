package interactor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/domain/model"
	infrastructure "github.com/nao1215/sqly/infrastructure/mock"
	"go.uber.org/mock/gomock"
)

func TestCsvInteractorList(t *testing.T) {
	t.Parallel()

	t.Run("show list data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		mockCSVRepo := infrastructure.NewMockCSVRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Open(filepath.Join("testdata", "sample.csv")).Return(dummyOsFile, nil)

		mockCSVRepo.EXPECT().List(dummyOsFile).Return(
			model.NewCSV(
				"sample",
				model.Header{"id", "name"},
				[]model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
			), nil,
		)

		csvInteractor := NewCSVInteractor(fileRepo, mockCSVRepo)
		got, err := csvInteractor.List(filepath.Join("testdata", "sample.csv"))
		if err != nil {
			t.Fatal(err)
		}
		want := model.NewTable(
			"sample",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
		)
		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf("differs: (-got +want)\n%s", diff)
		}
	})

	t.Run("failed to open file", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		mockCSVRepo := infrastructure.NewMockCSVRepository(ctrl)

		someErr := errors.New("failed to open file")
		fileRepo.EXPECT().Open(filepath.Join("testdata", "not_exist.csv")).Return(nil, someErr)

		csvInteractor := NewCSVInteractor(fileRepo, mockCSVRepo)
		_, err := csvInteractor.List(filepath.Join("testdata", "not_exist.csv"))
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("failed to list csv", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		mockCSVRepo := infrastructure.NewMockCSVRepository(ctrl)

		dummyOsFile := &os.File{}

		fileRepo.EXPECT().Open(filepath.Join("testdata", "sample.csv")).Return(dummyOsFile, nil)
		someErr := errors.New("failed to list csv")
		mockCSVRepo.EXPECT().List(dummyOsFile).Return(nil, someErr)

		csvInteractor := NewCSVInteractor(fileRepo, mockCSVRepo)
		_, err := csvInteractor.List(filepath.Join("testdata", "sample.csv"))
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestCsvInteractorDump(t *testing.T) {
	t.Parallel()

	t.Run("dump csv data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		mockCSVRepo := infrastructure.NewMockCSVRepository(ctrl)

		osFile := &os.File{}
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dummy.csv")).Return(osFile, nil)

		mockTable := model.NewTable(
			"dummy",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
		)
		mockCSVRepo.EXPECT().Dump(osFile, mockTable).Return(nil)

		csvInteractor := NewCSVInteractor(fileRepo, mockCSVRepo)
		err := csvInteractor.Dump(filepath.Join("testdata", "dummy.csv"), mockTable)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("failed to create file", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		mockCSVRepo := infrastructure.NewMockCSVRepository(ctrl)

		someErr := errors.New("failed to create file")
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dummy.csv")).Return(nil, someErr)

		mockTable := model.NewTable(
			"dummy",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
		)

		csvInteractor := NewCSVInteractor(fileRepo, mockCSVRepo)
		err := csvInteractor.Dump(filepath.Join("testdata", "dummy.csv"), mockTable)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("failed to dump csv", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		mockCSVRepo := infrastructure.NewMockCSVRepository(ctrl)

		osFile := &os.File{}
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dummy.csv")).Return(osFile, nil)

		mockTable := model.NewTable(
			"dummy",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
		)
		someErr := errors.New("failed to dump csv")
		mockCSVRepo.EXPECT().Dump(osFile, mockTable).Return(someErr)

		csvInteractor := NewCSVInteractor(fileRepo, mockCSVRepo)
		err := csvInteractor.Dump(filepath.Join("testdata", "dummy.csv"), mockTable)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
