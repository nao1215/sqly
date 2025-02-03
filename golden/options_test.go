package golden

import (
	"io/fs"
	"testing"
)

func TestGolden_WithFilePerms(t *testing.T) {
	t.Run("change file permission to 0666", func(t *testing.T) {
		want := fs.FileMode(0666)
		g := New(t,
			WithFilePerms(want),
		)

		if g.filePerms != want {
			t.Errorf("mismatch got=%v, want=%v", g.filePerms, want)
		}
	})
}

func TestGolden_WithDirPerms(t *testing.T) {
	t.Run("change directory permission to 0666", func(t *testing.T) {
		want := fs.FileMode(0666)
		g := New(t,
			WithDirPerms(want),
		)

		if g.dirPerms != want {
			t.Errorf("mismatch got=%v, want=%v", g.dirPerms, want)
		}
	})
}

func TestGolden_WithDiffEngine(t *testing.T) {
	t.Run("change diff engine to simple one", func(t *testing.T) {
		want := Simple
		g := New(t,
			WithDiffEngine(want),
		)
		if g.diffEngine != want {
			t.Errorf("mismatch got=%v, want=%v", g.diffEngine, want)
		}
	})
}

func TestGolden_WithDiffFn(t *testing.T) {
	t.Run("change diff function", func(t *testing.T) {
		want := "change-diff-func"
		f := func(_, _ string) string {
			return want
		}
		g := New(t,
			WithDiffFn(f),
		)

		got := g.diffFn("actual", "expected")
		if got != want {
			t.Errorf("mismatch got=%s, want=%s", got, want)
		}
	})
}

func TestGolden_WithIgnoreTemplateErrors(t *testing.T) {
	t.Run("change diff function", func(t *testing.T) {
		want := true
		g := New(t,
			WithIgnoreTemplateErrors(want),
		)

		got := g.ignoreTemplateErrors
		if got != want {
			t.Errorf("mismatch got=%v, want=%v", g.ignoreTemplateErrors, want)
		}
	})
}
