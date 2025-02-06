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

func TestTsvInteractorList(t *testing.T) {
	t.Parallel()

	t.Run("success to get tsv data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		tsvRepo := infrastructure.NewMockTSVRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Open(filepath.Join("testdata", "sample.tsv")).Return(dummyOsFile, nil)

		tsvRepo.EXPECT().List(dummyOsFile).Return(model.NewTSV(
			"sample",
			[]string{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		), nil)

		tsvInteractor := NewTSVInteractor(fileRepo, tsvRepo)
		got, err := tsvInteractor.List(filepath.Join("testdata", "sample.tsv"))
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
		tsvRepo := infrastructure.NewMockTSVRepository(ctrl)

		someErr := errors.New("failed to open file")
		fileRepo.EXPECT().Open(filepath.Join("testdata", "not_exist.tsv")).Return(nil, someErr)

		tsvInteractor := NewTSVInteractor(fileRepo, tsvRepo)
		_, err := tsvInteractor.List(filepath.Join("testdata", "not_exist.tsv"))
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestTsvInteractorDump(t *testing.T) {
	t.Parallel()

	t.Run("success to dump", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		tsvRepo := infrastructure.NewMockTSVRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Create(filepath.Join("testdata", "sample.tsv")).Return(dummyOsFile, nil)

		table := model.NewTable(
			"sample",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Gina"}, {"2", "Yulia"}},
		)
		tsvRepo.EXPECT().Dump(dummyOsFile, table).Return(nil)

		tsvInteractor := NewTSVInteractor(fileRepo, tsvRepo)
		err := tsvInteractor.Dump(filepath.Join("testdata", "sample.tsv"), table)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("failed to create file", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		tsvRepo := infrastructure.NewMockTSVRepository(ctrl)

		someErr := errors.New("failed to create file")
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dump.tsv")).Return(nil, someErr)

		tsvInteractor := NewTSVInteractor(fileRepo, tsvRepo)
		err := tsvInteractor.Dump(filepath.Join("testdata", "dump.tsv"), &model.Table{})
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("failed to dump", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		tsvRepo := infrastructure.NewMockTSVRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Create(filepath.Join("testdata", "dump.tsv")).Return(dummyOsFile, nil)

		someErr := errors.New("failed to dump")
		tsvRepo.EXPECT().Dump(dummyOsFile, &model.Table{}).Return(someErr)

		tsvInteractor := NewTSVInteractor(fileRepo, tsvRepo)
		err := tsvInteractor.Dump(filepath.Join("testdata", "dump.tsv"), &model.Table{})
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
