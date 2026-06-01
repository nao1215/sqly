package memory

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

func TestSqlite3RepositoryCreateTable(t *testing.T) {
	t.Run("Create table", func(t *testing.T) {
		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)
		want := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)

		if err := r.CreateTable(context.Background(), want); err != nil {
			t.Fatal(err)
		}

		got, err := r.TablesName(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if len(got) == 0 {
			t.Fatal("falied to create table (no table in memory db)")
		}

		if diff := cmp.Diff(got[0].Name(), want.Name()); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}

		got2, err := r.List(context.Background(), "sample")
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got2.Header(), want.Header()); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}

		got3, err := r.Header(context.Background(), "sample")
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(got3.Header(), want.Header()); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestSqlite3RepositoryInsert(t *testing.T) {
	t.Run("INSERT data", func(t *testing.T) {
		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)
		table := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)
		if err := r.CreateTable(context.Background(), table); err != nil {
			t.Fatal(err)
		}

		input := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"111", "222", "333"},
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)
		if err := r.Insert(context.Background(), input); err != nil {
			t.Fatal(err)
		}

		if _, err := r.Exec(context.Background(), "DELETE FROM sample WHERE aaa = '111'"); err != nil {
			t.Fatal(err)
		}

		got, err := r.Query(context.Background(), "SELECT * FROM sample ORDER BY aaa")
		if err != nil {
			t.Fatal(err)
		}

		want := model.NewTable(
			"sample",
			model.Header{"aaa", "bbb", "ccc"},
			[]model.Record{
				{"444", "555", "666"},
				{"777", "888", "999"},
			},
		)
		if diff := cmp.Diff(got, want); diff != "" {
			t.Fatalf("mismatch (-got +want):\n%s", diff)
		}
	})
}

func TestExtractTableName(t *testing.T) {
	t.Parallel()

	type args struct {
		query string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "extract table name",
			args: args{
				query: "SELECT * FROM `sample_table`",
			},
			want: "sample_table",
		},
		{
			name: "lower-case from keyword",
			args: args{query: "select id from people"},
			want: "people",
		},
		{
			name: "explain select",
			args: args{query: "EXPLAIN SELECT * FROM logs"},
			want: "logs",
		},
		{
			name: "no from clause returns empty",
			args: args{query: "SELECT 1"},
			want: "",
		},
		{
			name: "empty query returns empty",
			args: args{query: ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := extractTableName(tt.args.query); got != tt.want {
				t.Errorf("extractTableName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSqlite3RepositoryTablesNameExcludesInternalTables(t *testing.T) {
	t.Parallel()

	t.Run("excludes query_result_ tables", func(t *testing.T) {
		t.Parallel()

		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)

		// Create a regular table (must have at least one record for Valid() check)
		regularTable := model.NewTable(
			"users",
			model.Header{"id", "name"},
			[]model.Record{{"1", "test_user"}},
		)
		if err := r.CreateTable(context.Background(), regularTable); err != nil {
			t.Fatal(err)
		}

		// Create a query_result_ table (simulating internal table)
		db := (*sql.DB)(memoryDB)
		_, err = db.ExecContext(context.Background(),
			"CREATE TABLE query_result_abc123 (col1 TEXT, col2 TEXT)")
		if err != nil {
			t.Fatal(err)
		}

		// Create another regular table (must have at least one record for Valid() check)
		anotherTable := model.NewTable(
			"products",
			model.Header{"id", "price"},
			[]model.Record{{"1", "100"}},
		)
		if err := r.CreateTable(context.Background(), anotherTable); err != nil {
			t.Fatal(err)
		}

		// Get table names
		tables, err := r.TablesName(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		// Verify query_result_ table is excluded
		tableNames := make([]string, len(tables))
		for i, table := range tables {
			tableNames[i] = table.Name()
		}

		// Should have exactly 2 tables (users and products)
		if len(tables) != 2 {
			t.Errorf("Expected 2 tables, got %d: %v", len(tables), tableNames)
		}

		// Verify query_result_ table is not in the list
		for _, name := range tableNames {
			if name == "query_result_abc123" {
				t.Error("query_result_ table should be excluded from TablesName result")
			}
		}

		// Verify regular tables are included
		hasUsers := false
		hasProducts := false
		for _, name := range tableNames {
			if name == "users" {
				hasUsers = true
			}
			if name == "products" {
				hasProducts = true
			}
		}
		if !hasUsers {
			t.Error("Expected 'users' table to be in the list")
		}
		if !hasProducts {
			t.Error("Expected 'products' table to be in the list")
		}
	})

	t.Run("excludes sqlite_ tables", func(t *testing.T) {
		t.Parallel()

		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)

		// Create a regular table (must have at least one record for Valid() check)
		regularTable := model.NewTable(
			"data",
			model.Header{"id", "value"},
			[]model.Record{{"1", "test_value"}},
		)
		if err := r.CreateTable(context.Background(), regularTable); err != nil {
			t.Fatal(err)
		}

		// Get table names
		tables, err := r.TablesName(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		// Verify sqlite_ tables are excluded
		for _, table := range tables {
			if len(table.Name()) >= 7 && table.Name()[:7] == "sqlite_" {
				t.Errorf("sqlite_ table should be excluded: %s", table.Name())
			}
		}

		// Should have exactly 1 table (data)
		if len(tables) != 1 {
			t.Errorf("Expected 1 table, got %d", len(tables))
		}
		if tables[0].Name() != "data" {
			t.Errorf("Expected 'data' table, got %s", tables[0].Name())
		}
	})
}

func newSampleRepo(t *testing.T) repository.SQLite3Repository {
	t.Helper()
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	r := NewSQLite3Repository(memoryDB)
	table := model.NewTable("sample", model.Header{"id", "name"}, []model.Record{
		{"1", "alice"},
		{"2", "bob"},
	})
	if err := r.CreateTable(context.Background(), table); err != nil {
		t.Fatal(err)
	}
	if err := r.Insert(context.Background(), table); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSqlite3Repository_Query_InvalidSQLReturnsError(t *testing.T) {
	r := newSampleRepo(t)
	if _, err := r.Query(context.Background(), "SELECT * FROM no_such_table"); err == nil {
		t.Error("expected error for query against missing table, got nil")
	}
}

func TestSqlite3Repository_Exec_UpdateReturnsAffectedRows(t *testing.T) {
	r := newSampleRepo(t)
	affected, err := r.Exec(context.Background(), "UPDATE sample SET name = 'carol' WHERE id = '1'")
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Errorf("affected rows = %d, want 1", affected)
	}
}

func TestSqlite3Repository_Exec_InvalidStatementReturnsError(t *testing.T) {
	r := newSampleRepo(t)
	if _, err := r.Exec(context.Background(), "UPDATE no_such_table SET x = 1"); err == nil {
		t.Error("expected error for exec against missing table, got nil")
	}
}

func TestSqlite3Repository_CreateTable_InvalidTableReturnsError(t *testing.T) {
	r := newSampleRepo(t)
	// An empty table (no name/header/records) fails Valid() before any SQL runs.
	if err := r.CreateTable(context.Background(), model.NewTable("", model.Header{}, []model.Record{})); err == nil {
		t.Error("expected error for invalid table, got nil")
	}
}

// TestSqlite3RepositorySchemaObjects verifies that SchemaObjects lists base
// tables, views, and TEMP tables (so .tables can enumerate everything queryable),
// while TablesName stays limited to base tables used by write-back.,
func TestSqlite3RepositorySchemaObjects(t *testing.T) {
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	r := NewSQLite3Repository(memoryDB)
	ctx := context.Background()
	if _, err := r.Exec(ctx, "CREATE TABLE base (id INTEGER)"); err != nil {
		t.Fatalf("create base table: %v", err)
	}
	if _, err := r.Exec(ctx, "CREATE VIEW v AS SELECT id FROM base"); err != nil {
		t.Fatalf("create view: %v", err)
	}
	if _, err := r.Exec(ctx, "CREATE TEMP TABLE temp_t (id INTEGER)"); err != nil {
		t.Fatalf("create temp table: %v", err)
	}

	objects, err := r.SchemaObjects(ctx)
	if err != nil {
		t.Fatalf("SchemaObjects error: %v", err)
	}
	got := map[string]bool{}
	for _, o := range objects {
		got[o.Name()] = true
	}
	for _, want := range []string{"base", "v", "temp_t"} {
		if !got[want] {
			t.Errorf("SchemaObjects missing %q; got %v", want, got)
		}
	}

	// TablesName remains scoped to base tables for write-back, excluding the view
	// and the temp table.
	tables, err := r.TablesName(ctx)
	if err != nil {
		t.Fatalf("TablesName error: %v", err)
	}
	for _, tbl := range tables {
		if tbl.Name() == "v" {
			t.Error("TablesName unexpectedly included the view 'v'")
		}
	}
}

// TestSqlite3RepositoryListSchemaQualified verifies that List accepts a
// schema-qualified table name, so .dump and .header can target main.<table>.
func TestSqlite3RepositoryListSchemaQualified(t *testing.T) {
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	r := NewSQLite3Repository(memoryDB)
	ctx := context.Background()
	if _, err := r.Exec(ctx, "CREATE TABLE person (id INTEGER, name TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := r.Exec(ctx, "INSERT INTO person VALUES (1, 'Ann')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	table, err := r.List(ctx, "main.person")
	if err != nil {
		t.Fatalf("List(main.person) error = %v, want nil", err)
	}
	if len(table.Records()) != 1 || table.Records()[0][1] != "Ann" {
		t.Errorf("List(main.person) records = %v, want one row with Ann", table.Records())
	}
}
