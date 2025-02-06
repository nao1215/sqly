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

func TestJsonInteractorList(t *testing.T) {
	t.Parallel()

	t.Run("success to get JSON data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		jsonRepo := infrastructure.NewMockJSONRepository(ctrl)
		jsonRepo.EXPECT().List(filepath.Join("testdata", "sample.json")).Return(
			model.NewJSON("sample", []map[string]any{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			}), nil,
		)

		jsonInteractor := NewJSONInteractor(nil, jsonRepo)
		got, err := jsonInteractor.List(filepath.Join("testdata", "sample.json"))
		if err != nil {
			t.Fatal(err)
		}

		want := model.NewTable(
			"sample",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Alice"}, {"2", "Bob"}},
		)
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("mismatch: (-got +want)\n%s", diff)
		}
	})

	t.Run("failed to get JSON data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		jsonRepo := infrastructure.NewMockJSONRepository(ctrl)
		someErr := errors.New("failed to get JSON data")
		jsonRepo.EXPECT().List(filepath.Join("testdata", "not_exist.json")).Return(nil, someErr)

		jsonInteractor := NewJSONInteractor(nil, jsonRepo)
		_, err := jsonInteractor.List(filepath.Join("testdata", "not_exist.json"))
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestJsonInteractorDump(t *testing.T) {
	t.Parallel()

	t.Run("success to dump JSON data", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		jsonRepo := infrastructure.NewMockJSONRepository(ctrl)

		dummyOsFile := &os.File{}
		fileRepo.EXPECT().Create(filepath.Join("testdata", "sample.json")).Return(dummyOsFile, nil)
		jsonTable := model.NewTable(
			"sample",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Alice"}, {"2", "Bob"}},
		)
		jsonRepo.EXPECT().Dump(dummyOsFile, jsonTable).Return(nil)

		jsonInteractor := NewJSONInteractor(fileRepo, jsonRepo)
		err := jsonInteractor.Dump(filepath.Join("testdata", "sample.json"), jsonTable)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("failed to create file", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		fileRepo := infrastructure.NewMockFileRepository(ctrl)
		jsonRepo := infrastructure.NewMockJSONRepository(ctrl)

		someErr := errors.New("failed to create file")
		fileRepo.EXPECT().Create(filepath.Join("testdata", "sample.json")).Return(nil, someErr)

		jsonTable := model.NewTable(
			"sample",
			model.Header{"id", "name"},
			[]model.Record{{"1", "Alice"}, {"2", "Bob"}},
		)

		jsonInteractor := NewJSONInteractor(fileRepo, jsonRepo)
		err := jsonInteractor.Dump(filepath.Join("testdata", "sample.json"), jsonTable)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
