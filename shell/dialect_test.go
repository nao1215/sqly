package shell

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

func TestDialectCommand(t *testing.T) {
	t.Run("no args prints the current dialect and choices", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Dialect().Return(dialect.MySQL)
		s := newBoundaryTestShell(t, Usecases{query: query})

		var out string
		stdout := captureStdout(t, func() {
			out = captureStderr(t, func() {
				if err := NewCommands().dialectCommand(context.Background(), s, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})
		// stdout carries the rows a program parses, and a setting is not one.
		if stdout != "" {
			t.Fatalf("stdout = %q, want nothing: .dialect writes no data", stdout)
		}
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

		var out string
		stdout := captureStdout(t, func() {
			out = captureStderr(t, func() {
				if err := NewCommands().dialectCommand(context.Background(), s, []string{"postgres"}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})
		if stdout != "" {
			t.Fatalf("stdout = %q, want nothing: .dialect writes no data", stdout)
		}
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

// TestEveryDialectReachesEveryList is the drift guard the three hand-written
// lists needed: a dialect added to filesql must appear in completion, in
// --dialect's error, and in .dialect's error at once. Before this they were
// three copies, and a new dialect reached whichever the author remembered.
func TestEveryDialectReachesEveryList(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	s := newBoundaryTestShell(t, Usecases{query: mock.NewMockQueryUsecase(ctrl)})
	completions := completionTexts(dialectSuggestions())

	dotCommandErr := NewCommands().dialectCommand(context.Background(), s, []string{"oracle"})
	if dotCommandErr == nil {
		t.Fatal(".dialect oracle was accepted")
	}
	_, flagErr := config.NewArg([]string{"sqly", "--dialect", "oracle", "x.csv"})
	if flagErr == nil {
		t.Fatal("--dialect oracle was accepted")
	}

	for _, d := range dialect.Dialects() {
		name := string(d)
		if !slices.Contains(completions, name) {
			t.Errorf("completion does not offer dialect %q: %v", name, completions)
		}
		if !strings.Contains(dotCommandErr.Error(), name) {
			t.Errorf(".dialect error does not name dialect %q: %v", name, dotCommandErr)
		}
		if !strings.Contains(flagErr.Error(), name) {
			t.Errorf("--dialect error does not name dialect %q: %v", name, flagErr)
		}
	}
}
