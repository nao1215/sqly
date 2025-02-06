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

func TestLTSVInteractorList(t *testing.T) {
	t.Parallel()

	t.Run("success to get LTSV data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		ltsvRepo := infrastructure.NewMockLTSVRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Open(filepath.Join("testdata", "sample.ltsv")).Return(dummyOsFile, nil)

		ltsvRepo.EXPECT().List(dummyOsFile).Return(
			model.NewLTSV(
				"sample",
				model.Label{"id", "name"},
				[]model.Record{
					{"1", "Gina"},
					{"2", "Yulia"},
				},
			), nil)

		ltsvInteractor := NewLTSVInteractor(fileRepo, ltsvRepo)
		got, err := ltsvInteractor.List(filepath.Join("testdata", "sample.ltsv"))
		if err != nil {
			t.Fatal(err)
		}
		want := model.NewTable(
			"sample",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}, {"2", "Yulia"}},
		)
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("mismatch: (-got +want)\n%s", diff)
		}
	})

	t.Run("failed to open file", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		ltsvRepo := infrastructure.NewMockLTSVRepository(ctrl)

		someErr := errors.New("failed to open file")
		fileRepo.EXPECT().Open(filepath.Join("testdata", "not_exist.ltsv")).Return(nil, someErr)

		ltsvInteractor := NewLTSVInteractor(fileRepo, ltsvRepo)
		_, err := ltsvInteractor.List(filepath.Join("testdata", "not_exist.ltsv"))
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestLTSVInteractorDump(t *testing.T) {
	t.Parallel()

	t.Run("success to dump", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		ltsvRepo := infrastructure.NewMockLTSVRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dump.ltsv")).Return(dummyOsFile, nil)
		table := model.NewTable(
			"dump",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)
		ltsvRepo.EXPECT().Dump(dummyOsFile, table).Return(nil)

		ltsvInteractor := NewLTSVInteractor(fileRepo, ltsvRepo)
		err := ltsvInteractor.Dump(filepath.Join("testdata", "dump.ltsv"), table)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("failed to create file", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		ltsvRepo := infrastructure.NewMockLTSVRepository(ctrl)

		someErr := errors.New("failed to create file")
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dump.ltsv")).Return(nil, someErr)

		ltsvInteractor := NewLTSVInteractor(fileRepo, ltsvRepo)
		err := ltsvInteractor.Dump(filepath.Join("testdata", "dump.ltsv"), &model.Table{})
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("failed to dump", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		ltsvRepo := infrastructure.NewMockLTSVRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dump.ltsv")).Return(dummyOsFile, nil)
		someErr := errors.New("failed to dump")
		table := model.NewTable(
			"dump",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)
		ltsvRepo.EXPECT().Dump(dummyOsFile, table).Return(someErr)

		ltsvInteractor := NewLTSVInteractor(fileRepo, ltsvRepo)
		err := ltsvInteractor.Dump(filepath.Join("testdata", "dump.ltsv"), table)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
