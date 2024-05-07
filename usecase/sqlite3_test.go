package usecase

import (
	"context"
	"strings"
	"testing"
)

func TestSQLite3Interactor_ExecSQL(t *testing.T) {
	t.Run("execute CREATE error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")

		want := "not support data definition language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute DROP error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "DROP TABLE test")

		want := "not support data definition language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute ALTER error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "ALTER TABLE test ADD COLUMN age INTEGER")

		want := "not support data definition language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute REINDEX error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "REINDEX test")

		want := "not support data definition language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute BEGIN error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "BEGIN")

		want := "not support transaction control language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute COMMIT error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "COMMIT")

		want := "not support transaction control language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute ROLLBACK error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "ROLLBACK")

		want := "not support transaction control language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute SAVEPOINT error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "SAVEPOINT test")

		want := "not support transaction control language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute RELEASE error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "RELEASE test")

		want := "not support transaction control language" //nolint
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute GRANT error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "GRANT SELECT ON test TO user")

		want := "not support data control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute REVOKE error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "REVOKE SELECT ON test FROM user")

		want := "not support data control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute undifined statement error", func(t *testing.T) {
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "UNDEFINED STATEMENT")

		want := "this input is not sql query or sqly helper command:"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})
}
