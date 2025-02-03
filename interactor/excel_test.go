package interactor

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/domain/model"
	infrastructure "github.com/nao1215/sqly/infrastructure/mock"
	"go.uber.org/mock/gomock"
)

func TestExcelInteractorList(t *testing.T) {
	t.Parallel()

	t.Run("show list data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		excelRepo := infrastructure.NewMockExcelRepository(ctrl)
		excelRepo.EXPECT().List(filepath.Join("testdata", "sample.xlsx"), "Sheet1").Return(
			&model.Excel{
				Header:  model.Header{"id", "name"},
				Records: []model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
			}, nil,
		)

		excelInteractor := NewExcelInteractor(excelRepo)
		got, err := excelInteractor.List(filepath.Join("testdata", "sample.xlsx"), "Sheet1")
		if err != nil {
			t.Fatal(err)
		}

		want := &model.Excel{
			Header:  model.Header{"id", "name"},
			Records: []model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf("differs: (-got +want)\n%s", diff)
		}
	})

	t.Run("failed to open excel file", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		excelRepo := infrastructure.NewMockExcelRepository(ctrl)
		someErr := errors.New("failed to open excel file")
		excelRepo.EXPECT().List(filepath.Join("testdata", "not_exist.xlsx"), "Sheet1").Return(nil, someErr)

		excelInteractor := NewExcelInteractor(excelRepo)
		_, err := excelInteractor.List(filepath.Join("testdata", "not_exist.xlsx"), "Sheet1")
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestExcelInteractorDump(t *testing.T) {
	t.Parallel()

	t.Run("success to dump", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		excelRepo := infrastructure.NewMockExcelRepository(ctrl)
		table := &model.Table{
			Header:  model.Header{"id", "name"},
			Records: []model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
		}
		excelRepo.EXPECT().Dump(filepath.Join("testdata", "dump.xlsx"), table).Return(nil)

		excelInteractor := NewExcelInteractor(excelRepo)
		err := excelInteractor.Dump(filepath.Join("testdata", "dump.xlsx"), table)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("failed to dump", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		excelRepo := infrastructure.NewMockExcelRepository(ctrl)
		table := &model.Table{
			Header:  model.Header{"id", "name"},
			Records: []model.Record{{"1", "Gina"}, {"2", "Yulia"}, {"3", "Vika"}},
		}
		someErr := errors.New("failed to dump")
		excelRepo.EXPECT().Dump(filepath.Join("testdata", "dump.xlsx"), table).Return(someErr)

		excelInteractor := NewExcelInteractor(excelRepo)
		err := excelInteractor.Dump(filepath.Join("testdata", "dump.xlsx"), table)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
