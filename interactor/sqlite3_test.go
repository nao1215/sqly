package interactor

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	infrastructure "github.com/nao1215/sqly/infrastructure/mock"
	"go.uber.org/mock/gomock"
)

func TestSQLite3InteractorExecSQL(t *testing.T) {
	t.Parallel()

	t.Run("execute CREATE error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")

		want := "not support data definition language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute DROP error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "DROP TABLE test")

		want := "not support data definition language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute ALTER error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "ALTER TABLE test ADD COLUMN age INTEGER")

		want := "not support data definition language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute REINDEX error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "REINDEX test")

		want := "not support data definition language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute BEGIN error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "BEGIN")

		want := "not support transaction control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute COMMIT error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "COMMIT")

		want := "not support transaction control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute ROLLBACK error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "ROLLBACK")

		want := "not support transaction control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute SAVEPOINT error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "SAVEPOINT test")

		want := "not support transaction control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute RELEASE error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "RELEASE test")

		want := "not support transaction control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute GRANT error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "GRANT SELECT ON test TO user")

		want := "not support data control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute REVOKE error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "REVOKE SELECT ON test FROM user")

		want := "not support data control language"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})

	t.Run("execute SELECT statement succeeded", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "SELECT * FROM test ORDER BY id"
		expectedTable := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)

		repo.EXPECT().Query(gomock.Any(), query).Return(expectedTable, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, _, err := interactor.ExecSQL(context.Background(), query)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if !reflect.DeepEqual(got, expectedTable) {
			t.Errorf("want: %v, got: %v", expectedTable, got)
		}
	})

	t.Run("execute SELECT statement failed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "SELECT * FROM test ORDER BY id"
		someErr := errors.New("failed to execute query")

		repo.EXPECT().Query(gomock.Any(), query).Return(nil, someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, _, err := interactor.ExecSQL(context.Background(), query)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("execute EXPLAIN statement succeeded", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "EXPLAIN SELECT * FROM test"
		expectedTable := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)
		repo.EXPECT().Query(gomock.Any(), query).Return(expectedTable, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, _, err := interactor.ExecSQL(context.Background(), query)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if !reflect.DeepEqual(got, expectedTable) {
			t.Errorf("want: %v, got: %v", expectedTable, got)
		}
	})

	t.Run("execute EXPLAIN statement failed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "EXPLAIN SELECT * FROM test"
		someErr := errors.New("failed to execute query")

		repo.EXPECT().Query(gomock.Any(), query).Return(nil, someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, _, err := interactor.ExecSQL(context.Background(), query)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("execute WITH (CTE) statement succeeded", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "WITH test_cte AS (SELECT * FROM test WHERE id > 1) SELECT * FROM test_cte"
		expectedTable := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"2", "Yulia"},
				{"3", "Bob"},
			},
		)

		repo.EXPECT().Query(gomock.Any(), query).Return(expectedTable, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, _, err := interactor.ExecSQL(context.Background(), query)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if !reflect.DeepEqual(got, expectedTable) {
			t.Errorf("want: %v, got: %v", expectedTable, got)
		}
	})

	t.Run("execute WITH (CTE) statement failed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "WITH test_cte AS (SELECT * FROM test WHERE id > 1) SELECT * FROM test_cte"
		someErr := errors.New("failed to execute CTE query")

		repo.EXPECT().Query(gomock.Any(), query).Return(nil, someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, _, err := interactor.ExecSQL(context.Background(), query)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("execute INSERT statement succeeded", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "INSERT INTO test (id, name) VALUES (1, 'Gina')"
		expectedRows := int64(1)

		repo.EXPECT().Exec(gomock.Any(), statement).Return(expectedRows, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, got, err := interactor.ExecSQL(context.Background(), statement)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if got != expectedRows {
			t.Errorf("want: %v, got: %v", expectedRows, got)
		}
	})

	t.Run("execute UPDATE statement succeeded", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "UPDATE test SET name = 'Yulia' WHERE id = 1"
		expectedRows := int64(1)

		repo.EXPECT().Exec(gomock.Any(), statement).Return(expectedRows, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, got, err := interactor.ExecSQL(context.Background(), statement)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if got != expectedRows {
			t.Errorf("want: %v, got: %v", expectedRows, got)
		}
	})

	t.Run("execute DELETE statement succeeded", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "DELETE FROM test WHERE id = 1"
		expectedRows := int64(1)

		repo.EXPECT().Exec(gomock.Any(), statement).Return(expectedRows, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, got, err := interactor.ExecSQL(context.Background(), statement)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if got != expectedRows {
			t.Errorf("want: %v, got: %v", expectedRows, got)
		}
	})

	t.Run("execute statement failed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "INSERT INTO test (id, name) VALUES (1, 'Gina')"
		someErr := errors.New("failed to execute statement")

		repo.EXPECT().Exec(gomock.Any(), statement).Return(int64(0), someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, _, err := interactor.ExecSQL(context.Background(), statement)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	t.Run("execute undifined statement error", func(t *testing.T) {
		t.Parallel()
		interactor := NewSQLite3Interactor(nil, nil)

		si := NewSQLite3Interactor(interactor, NewSQL())
		_, _, got := si.ExecSQL(context.Background(), "UNDEFINED STATEMENT")

		want := "this input is not sql query or sqly helper command:"
		if !strings.Contains(got.Error(), want) {
			t.Errorf("want: %v, got: %v", want, got)
		}
	})
}

func TestSqlite3InteractorCreateTable(t *testing.T) {
	t.Parallel()

	t.Run("create table succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		table := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)
		repo.EXPECT().CreateTable(gomock.Any(), table).Return(nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		if err := interactor.CreateTable(context.Background(), table); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("create table failed", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		table := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)

		someErr := errors.New("failed to create table")
		repo.EXPECT().CreateTable(gomock.Any(), table).Return(someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		if err := interactor.CreateTable(context.Background(), table); !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestSqlite3InteractorTablesName(t *testing.T) {
	t.Parallel()

	t.Run("get tables name succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		tables := []*model.Table{
			model.NewTable("table1", model.Header{}, nil),
			model.NewTable("table2", model.Header{}, nil),
		}

		repo.EXPECT().TablesName(gomock.Any()).Return(tables, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, err := interactor.TablesName(context.Background())
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if len(got) != len(tables) {
			t.Errorf("want: %v, got: %v", tables, got)
		}
	})

	t.Run("get tables name failed", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		someErr := errors.New("failed to get tables name")
		repo.EXPECT().TablesName(gomock.Any()).Return(nil, someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, err := interactor.TablesName(context.Background())
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestSqlite3InteractorInsert(t *testing.T) {
	t.Parallel()

	t.Run("insert records succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		table := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)

		repo.EXPECT().Insert(gomock.Any(), table).Return(nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		if err := interactor.Insert(context.Background(), table); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("insert records failed", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		table := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)

		someErr := errors.New("failed to insert records")
		repo.EXPECT().Insert(gomock.Any(), table).Return(someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		if err := interactor.Insert(context.Background(), table); !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestSqlite3InteractorList(t *testing.T) {
	t.Parallel()

	t.Run("list records succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		tableName := "test"
		expectedTable := model.NewTable(
			tableName,
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)

		repo.EXPECT().List(gomock.Any(), tableName).Return(expectedTable, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, err := interactor.List(context.Background(), tableName)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if !reflect.DeepEqual(got, expectedTable) {
			t.Errorf("want: %v, got: %v", expectedTable, got)
		}
	})

	t.Run("list records failed", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		tableName := "test"
		someErr := errors.New("failed to list records")

		repo.EXPECT().List(gomock.Any(), tableName).Return(nil, someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, err := interactor.List(context.Background(), tableName)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestSqlite3InteractorHeader(t *testing.T) {
	t.Parallel()

	t.Run("get header succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		tableName := "test"
		expectedHeader := model.NewTable(
			tableName,
			model.Header{"id", "name"},
			nil,
		)
		repo.EXPECT().Header(gomock.Any(), tableName).Return(expectedHeader, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, err := interactor.Header(context.Background(), tableName)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if !reflect.DeepEqual(got, expectedHeader) {
			t.Errorf("want: %v, got: %v", expectedHeader, got)
		}
	})

	t.Run("get header failed", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		tableName := "test"
		someErr := errors.New("failed to get header")

		repo.EXPECT().Header(gomock.Any(), tableName).Return(nil, someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, err := interactor.Header(context.Background(), tableName)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestSqlite3InteractorQuery(t *testing.T) {
	t.Parallel()

	t.Run("query succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "SELECT * FROM test ORDER BY id"
		expectedTable := model.NewTable(
			"test",
			model.Header{"id", "name"},
			[]model.Record{
				{"1", "Gina"},
				{"2", "Yulia"},
			},
		)
		repo.EXPECT().Query(gomock.Any(), query).Return(expectedTable, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, err := interactor.Query(context.Background(), query)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if !reflect.DeepEqual(got, expectedTable) {
			t.Errorf("want: %v, got: %v", expectedTable, got)
		}
	})

	t.Run("query failed", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		query := "SELECT * FROM test ORDER BY id"
		someErr := errors.New("failed to execute query")

		repo.EXPECT().Query(gomock.Any(), query).Return(nil, someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, err := interactor.Query(context.Background(), query)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}

func TestSqlite3InteractorExec(t *testing.T) {
	t.Parallel()

	t.Run("exec insert statement succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "INSERT INTO test (id, name) VALUES (1, 'Gina')"
		expectedRows := int64(1)

		repo.EXPECT().Exec(gomock.Any(), statement).Return(expectedRows, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, err := interactor.Exec(context.Background(), statement)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if got != expectedRows {
			t.Errorf("want: %v, got: %v", expectedRows, got)
		}
	})

	t.Run("exec update statement succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "UPDATE test SET name = 'Yulia' WHERE id = 1"
		expectedRows := int64(1)

		repo.EXPECT().Exec(gomock.Any(), statement).Return(expectedRows, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, err := interactor.Exec(context.Background(), statement)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if got != expectedRows {
			t.Errorf("want: %v, got: %v", expectedRows, got)
		}
	})

	t.Run("exec delete statement succeeded", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "DELETE FROM test WHERE id = 1"
		expectedRows := int64(1)

		repo.EXPECT().Exec(gomock.Any(), statement).Return(expectedRows, nil)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		got, err := interactor.Exec(context.Background(), statement)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if got != expectedRows {
			t.Errorf("want: %v, got: %v", expectedRows, got)
		}
	})

	t.Run("exec statement failed", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "INSERT INTO test (id, name) VALUES (1, 'Gina')"
		someErr := errors.New("failed to execute statement")

		repo.EXPECT().Exec(gomock.Any(), statement).Return(int64(0), someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL())
		_, err := interactor.Exec(context.Background(), statement)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
