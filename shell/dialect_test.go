package shell

import (
	"context"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

func TestDialectCommand(t *testing.T) {
	t.Run("no args prints the current dialect and choices", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Dialect().Return(dialect.MySQL)
		s := newBoundaryTestShell(t, Usecases{query: query})

		out := captureStdout(t, func() {
			if err := NewCommands().dialectCommand(context.Background(), s, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "current dialect: mysql") {
			t.Fatalf("output = %q, want it to mention the current dialect", out)
		}
		if !strings.Contains(out, "sqlite, mysql, postgresql, googlesql") {
			t.Fatalf("output = %q, want it to list the available dialects", out)
		}
	})

	t.Run("a valid name (including an alias) sets the dialect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().SetDialect(dialect.PostgreSQL)
		s := newBoundaryTestShell(t, Usecases{query: query})

		out := captureStdout(t, func() {
			if err := NewCommands().dialectCommand(context.Background(), s, []string{"postgres"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "dialect set to postgresql") {
			t.Fatalf("output = %q, want confirmation of the new dialect", out)
		}
	})

	t.Run("an unknown name errors without changing the dialect", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl) // no SetDialect call expected
		s := newBoundaryTestShell(t, Usecases{query: query})

		err := NewCommands().dialectCommand(context.Background(), s, []string{"oracle"})
		if err == nil {
			t.Fatal("expected an error for an unknown dialect")
		}
		if !strings.Contains(err.Error(), "unknown SQL dialect") {
			t.Fatalf("error = %v, want it to mention the unknown dialect", err)
		}
	})

	t.Run("more than one argument errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		s := newBoundaryTestShell(t, Usecases{query: mock.NewMockQueryUsecase(ctrl)})
		if err := NewCommands().dialectCommand(context.Background(), s, []string{"mysql", "extra"}); err == nil {
			t.Fatal("expected an error for too many arguments")
		}
	})
}
