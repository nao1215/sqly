package interactor

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	infrastructure "github.com/nao1215/sqly/infrastructure/mock"
	"go.uber.org/mock/gomock"
)

func TestSQLite3InteractorExecSQL(t *testing.T) {
	t.Parallel()

	// DDL and other no-rowset statements that sqly can run inside its per-statement
	// transaction are routed to the exec path. SQLite remains the authority on
	// validity.
	execRouted := []struct {
		name      string
		statement string
	}{
		{"CREATE routes to exec", "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)"},
		{"DROP routes to exec", "DROP TABLE test"},
		{"ALTER routes to exec", "ALTER TABLE test ADD COLUMN age INTEGER"},
		{"REINDEX routes to exec", "REINDEX test"},
		{"ANALYZE routes to exec", "ANALYZE"},
		{"REPLACE routes to exec", "REPLACE INTO test(id) VALUES (1)"},
	}
	for _, tt := range execRouted {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repo := infrastructure.NewMockSQLite3Repository(ctrl)
			repo.EXPECT().Exec(gomock.Any(), tt.statement).Return(int64(0), nil)

			si := NewSQLite3Interactor(repo, NewSQL(), nil)
			if _, _, err := si.ExecSQL(context.Background(), tt.statement); err != nil {
				t.Errorf("want: nil, got: %v", err)
			}
		})
	}

	// Explicit transaction control, VACUUM, and ATTACH/DETACH cannot run correctly
	// under sqly's per-statement transaction and in-memory session model, so they
	// are rejected up front with a clear error and never reach the repository.
	rejected := []struct {
		name      string
		statement string
	}{
		{"BEGIN is rejected", "BEGIN"},
		{"BEGIN IMMEDIATE is rejected", "BEGIN IMMEDIATE"},
		{"COMMIT is rejected", "COMMIT"},
		{"ROLLBACK is rejected", "ROLLBACK"},
		{"SAVEPOINT is rejected", "SAVEPOINT test"},
		{"RELEASE is rejected", "RELEASE test"},
		{"VACUUM is rejected", "VACUUM"},
		{"VACUUM INTO is rejected", "VACUUM INTO 'dump.db'"},
		{"ATTACH is rejected", "ATTACH DATABASE ':memory:' AS aux"},
		{"DETACH is rejected", "DETACH DATABASE aux"},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			// No Exec/Query is expected: the statement is rejected before routing.
			repo := infrastructure.NewMockSQLite3Repository(ctrl)

			si := NewSQLite3Interactor(repo, NewSQL(), nil)
			if _, _, err := si.ExecSQL(context.Background(), tt.statement); err == nil {
				t.Errorf("ExecSQL(%q) = nil error, want a rejection", tt.statement)
			}
		})
	}

	// PRAGMA, VALUES, and the TABLE shorthand return a result set, so they run on
	// the query path.
	queryRouted := []struct {
		name      string
		statement string
		// queried is the statement the repository receives; it differs from
		// statement when sqly rewrites a shorthand (TABLE name).
		queried string
	}{
		{"PRAGMA routes to query", "PRAGMA table_info(test)", "PRAGMA table_info(test)"},
		{"VALUES routes to query", "VALUES (1), (2)", "VALUES (1), (2)"},
		{"TABLE shorthand is rewritten and routes to query", "TABLE test", "SELECT * FROM test"},
	}
	for _, tt := range queryRouted {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repo := infrastructure.NewMockSQLite3Repository(ctrl)
			repo.EXPECT().Query(gomock.Any(), tt.queried).Return(model.NewTable("test", model.Header{"x"}, []model.Record{{"1"}}), nil)

			si := NewSQLite3Interactor(repo, NewSQL(), nil)
			if _, _, err := si.ExecSQL(context.Background(), tt.statement); err != nil {
				t.Errorf("want: nil, got: %v", err)
			}
		})
	}

	// A no-rowset PRAGMA (a setter like "PRAGMA user_version = 1" or a command like
	// "PRAGMA incremental_vacuum") is routed to the query path by keyword but yields
	// no result columns, so the query path returns ErrNoRows. ExecSQL must re-run it
	// on the exec path and report neutral success instead of a "no records" error.
	pragmaSetters := []string{"PRAGMA user_version = 1", "PRAGMA incremental_vacuum"}
	for _, stmt := range pragmaSetters {
		t.Run("no-rowset PRAGMA falls through to exec: "+stmt, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repo := infrastructure.NewMockSQLite3Repository(ctrl)
			// The query path runs first and reports no records; ExecSQL then re-runs
			// the same statement on the exec path.
			repo.EXPECT().Query(gomock.Any(), stmt).Return(nil, repository.ErrNoRows)
			repo.EXPECT().Exec(gomock.Any(), stmt).Return(int64(0), nil)

			si := NewSQLite3Interactor(repo, NewSQL(), nil)
			table, _, err := si.ExecSQL(context.Background(), stmt)
			if err != nil {
				t.Errorf("ExecSQL(%q) error = %v, want nil", stmt, err)
			}
			if table != nil {
				t.Errorf("ExecSQL(%q) returned a table, want nil for a no-rowset PRAGMA", stmt)
			}
		})
	}

	// A genuine query error (not ErrNoRows) on the query path is surfaced, not
	// retried on the exec path.
	t.Run("query error other than ErrNoRows is surfaced", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		repo.EXPECT().Query(gomock.Any(), "SELECT * FROM missing").Return(nil, errors.New("no such table: missing"))

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), "SELECT * FROM missing"); err == nil {
			t.Error("ExecSQL on a failing query returned nil error, want the query error")
		}
	})

	// A leading comment or UTF-8 BOM is stripped before classification, so the
	// statement runs the same way the batch and --sql-file paths run it.
	t.Run("leading line comment is stripped and SELECT runs on query path", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		repo.EXPECT().Query(gomock.Any(), "SELECT 1 AS x").Return(model.NewTable("", model.Header{"x"}, []model.Record{{"1"}}), nil)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), "-- comment\nSELECT 1 AS x"); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("leading BOM is stripped and SELECT runs on query path", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		repo.EXPECT().Query(gomock.Any(), "SELECT 1 AS x").Return(model.NewTable("", model.Header{"x"}, []model.Record{{"1"}}), nil)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), "\ufeffSELECT 1 AS x"); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	// A leading empty statement (a bare ";") is dropped before classification, so
	// ";SELECT 1" runs as the SELECT on the query path instead of being misread as a
	// no-rowset statement that discards the query.
	t.Run("leading empty statement is dropped and SELECT runs on query path", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		repo.EXPECT().Query(gomock.Any(), "SELECT 1 AS x").Return(model.NewTable("", model.Header{"x"}, []model.Record{{"1"}}), nil)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), ";SELECT 1 AS x"); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("multiple leading empty statements are dropped and SELECT runs on query path", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		repo.EXPECT().Query(gomock.Any(), "SELECT 1 AS x").Return(model.NewTable("", model.Header{"x"}, []model.Record{{"1"}}), nil)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), "; ;SELECT 1 AS x"); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("leading empty statement before UPDATE routes to exec", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		repo.EXPECT().Exec(gomock.Any(), "UPDATE test SET name='z' WHERE id=1").Return(int64(1), nil)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), ";UPDATE test SET name='z' WHERE id=1"); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("leading empty statement before ATTACH is still rejected as unsupported", func(t *testing.T) {
		t.Parallel()
		si := NewSQLite3Interactor(nil, NewSQL(), nil)
		_, _, got := si.ExecSQL(context.Background(), ";ATTACH DATABASE '/tmp/x.db' AS aux")
		if got == nil || !strings.Contains(got.Error(), "ATTACH/DETACH") {
			t.Errorf("want: ATTACH rejection, got: %v", got)
		}
	})

	// A non-returning WITH ... UPDATE/INSERT/DELETE is DML, so it runs on the exec
	// path instead of the query path.
	t.Run("WITH ... UPDATE without RETURNING routes to exec", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		stmt := "WITH src AS (SELECT 1 AS id) UPDATE test SET name='z' WHERE id IN (SELECT id FROM src)"
		repo.EXPECT().Exec(gomock.Any(), stmt).Return(int64(1), nil)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), stmt); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("WITH ... UPDATE RETURNING routes to query", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		stmt := "WITH src AS (SELECT 1 AS id) UPDATE test SET name='z' WHERE id IN (SELECT id FROM src) RETURNING *"
		repo.EXPECT().Query(gomock.Any(), stmt).Return(model.NewTable("test", model.Header{"id", "name"}, []model.Record{{"1", "z"}}), nil)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		if _, _, err := si.ExecSQL(context.Background(), stmt); err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
	})

	t.Run("comment-only input is rejected as no executable SQL", func(t *testing.T) {
		t.Parallel()
		si := NewSQLite3Interactor(nil, NewSQL(), nil)
		_, _, got := si.ExecSQL(context.Background(), "-- just a comment")
		if got == nil || !strings.Contains(got.Error(), "no executable SQL") {
			t.Errorf("want: no executable SQL error, got: %v", got)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
		_, got, err := interactor.ExecSQL(context.Background(), statement)
		if err != nil {
			t.Errorf("want: nil, got: %v", err)
		}
		if got != expectedRows {
			t.Errorf("want: %v, got: %v", expectedRows, got)
		}
	})

	t.Run("DML with RETURNING routes to Query and returns rows", func(t *testing.T) {
		t.Parallel()
		for _, statement := range []string{
			"UPDATE test SET name = 'X' WHERE id = 1 RETURNING id, name",
			"INSERT INTO test (id, name) VALUES (2, 'Y') RETURNING id",
			"DELETE FROM test WHERE id = 1 returning id",
		} {
			ctrl := gomock.NewController(t)
			repo := infrastructure.NewMockSQLite3Repository(ctrl)
			want := model.NewTable("", model.Header{"id"}, []model.Record{{"1"}})
			// A RETURNING statement must go through Query (which captures the
			// rowset), never the affected-count-only Exec path.
			repo.EXPECT().Query(gomock.Any(), statement).Return(want, nil)

			interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
			got, affected, err := interactor.ExecSQL(context.Background(), statement)
			if err != nil {
				t.Fatalf("ExecSQL(%q) error: %v", statement, err)
			}
			if got == nil {
				t.Fatalf("ExecSQL(%q) returned a nil table; RETURNING rows were discarded", statement)
			}
			if affected != 0 {
				t.Errorf("ExecSQL(%q) affected = %d, want 0 for a rowset result", statement, affected)
			}
		}
	})

	t.Run("DML without RETURNING still uses Exec", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		statement := "UPDATE test SET name = 'returning' WHERE id = 1"
		repo.EXPECT().Exec(gomock.Any(), statement).Return(int64(1), nil)

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
		table, affected, err := interactor.ExecSQL(context.Background(), statement)
		if err != nil {
			t.Fatal(err)
		}
		if table != nil {
			t.Error("expected a nil table for a non-RETURNING UPDATE")
		}
		if affected != 1 {
			t.Errorf("affected = %d, want 1", affected)
		}
	})

	t.Run("execute statement failed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)

		statement := "INSERT INTO test (id, name) VALUES (1, 'Gina')"
		someErr := errors.New("failed to execute statement")

		repo.EXPECT().Exec(gomock.Any(), statement).Return(int64(0), someErr)

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
		_, _, err := interactor.ExecSQL(context.Background(), statement)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})

	// An unrecognized leading keyword is no longer rejected by sqly; it is handed
	// to SQLite on the exec path, which surfaces its own syntax error. sqly wraps
	// that error with an "execute statement error" prefix.
	t.Run("unknown statement is passed to SQLite and surfaces its error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		repo := infrastructure.NewMockSQLite3Repository(ctrl)
		statement := "UNDEFINED STATEMENT"
		someErr := errors.New(`near "UNDEFINED": syntax error`)
		repo.EXPECT().Exec(gomock.Any(), statement).Return(int64(0), someErr)

		si := NewSQLite3Interactor(repo, NewSQL(), nil)
		_, _, got := si.ExecSQL(context.Background(), statement)
		if !errors.Is(got, someErr) {
			t.Errorf("want: %v, got: %v", someErr, got)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
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

		interactor := NewSQLite3Interactor(repo, NewSQL(), nil)
		_, err := interactor.Exec(context.Background(), statement)
		if !errors.Is(err, someErr) {
			t.Errorf("want: %v, got: %v", someErr, err)
		}
	})
}
