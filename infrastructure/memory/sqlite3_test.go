package memory

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

func TestSQLite3RepositoryQueryPreservesSQLTypesForJSON(t *testing.T) {
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	repo := NewSQLite3Repository(memoryDB)
	table, err := repo.Query(context.Background(), `SELECT 42 AS integer_value, 1.5 AS real_value, '123' AS text_number, 'true' AS text_bool, 'false' AS text_false, '00123' AS padded, NULL AS null_value, '' AS empty_value, TRUE AS true_literal, FALSE AS false_literal, 1 = 1 AS true_expression, 2 > 3 AS false_expression`)
	if err != nil {
		t.Fatal(err)
	}

	assertJSONRow := func(t *testing.T, row map[string]any) {
		t.Helper()
		if got, ok := row["integer_value"].(float64); !ok || got != 42 {
			t.Errorf("integer_value = %#v (%T), want JSON number 42", row["integer_value"], row["integer_value"])
		}
		if got, ok := row["real_value"].(float64); !ok || got != 1.5 {
			t.Errorf("real_value = %#v (%T), want JSON number 1.5", row["real_value"], row["real_value"])
		}
		for _, name := range []string{"text_number", "text_bool", "text_false", "padded", "empty_value"} {
			if _, ok := row[name].(string); !ok {
				t.Errorf("%s = %#v (%T), want JSON string", name, row[name], row[name])
			}
		}
		if row["text_number"] != "123" || row["text_bool"] != "true" || row["text_false"] != "false" || row["padded"] != "00123" {
			t.Errorf("text values changed: %#v", row)
		}
		if row["null_value"] != nil {
			t.Errorf("null_value = %#v, want nil", row["null_value"])
		}
		for _, name := range []string{"true_literal", "false_literal", "true_expression", "false_expression"} {
			if _, ok := row[name].(float64); !ok {
				t.Errorf("%s = %#v (%T), want SQLite integer JSON number", name, row[name], row[name])
			}
		}
	}

	t.Run("json", func(t *testing.T) {
		var out bytes.Buffer
		if err := table.Print(&out, model.PrintModeJSON); err != nil {
			t.Fatal(err)
		}
		var rows []map[string]any
		if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("decoded %d rows, want 1", len(rows))
		}
		assertJSONRow(t, rows[0])
	})

	t.Run("ndjson", func(t *testing.T) {
		var out bytes.Buffer
		if err := table.Print(&out, model.PrintModeJSONL); err != nil {
			t.Fatal(err)
		}
		var row map[string]any
		if err := json.Unmarshal(out.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		assertJSONRow(t, row)
	})
}

func TestSQLite3RepositoryListPreservesQueryMetadata(t *testing.T) {
	t.Parallel()
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	db := (*sql.DB)(memoryDB)
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE typed_values (integer_value INTEGER, real_value REAL, text_value TEXT, null_value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `INSERT INTO typed_values VALUES (42, 1.5, '123', NULL)`); err != nil {
		t.Fatal(err)
	}

	table, err := NewSQLite3Repository(memoryDB).List(context.Background(), "typed_values")
	if err != nil {
		t.Fatal(err)
	}
	if table.Name() != "typed_values" {
		t.Fatalf("table name = %q, want typed_values", table.Name())
	}
	var rows []map[string]any
	var out bytes.Buffer
	if err := table.Print(&out, model.PrintModeJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("decoded %d rows, want 1", len(rows))
	}
	if _, ok := rows[0]["integer_value"].(float64); !ok {
		t.Errorf("integer_value = %#v (%T), want JSON number", rows[0]["integer_value"], rows[0]["integer_value"])
	}
	if _, ok := rows[0]["real_value"].(float64); !ok {
		t.Errorf("real_value = %#v (%T), want JSON number", rows[0]["real_value"], rows[0]["real_value"])
	}
	if rows[0]["text_value"] != "123" {
		t.Errorf("text_value = %#v, want string 123", rows[0]["text_value"])
	}
	if rows[0]["null_value"] != nil {
		t.Errorf("null_value = %#v, want JSON null", rows[0]["null_value"])
	}
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
		{
			name: "a from in a trailing line comment names nothing",
			args: args{query: "SELECT 1 -- from"},
			want: "",
		},
		{
			name: "a from in a block comment names nothing",
			args: args{query: "SELECT 1 /* from */"},
			want: "",
		},
		{
			name: "a column aliased from names nothing",
			args: args{query: "SELECT 1 AS `from`"},
			want: "",
		},
		{
			name: "a from in a string literal names nothing",
			args: args{query: "SELECT 'from t' AS s"},
			want: "",
		},
		{
			name: "a subquery after from names nothing",
			args: args{query: "SELECT * FROM (SELECT 1 AS a)"},
			want: "",
		},
		{
			name: "a terminator is not part of the name",
			args: args{query: "SELECT * FROM people;"},
			want: "people",
		},
		{
			name: "a schema-qualified name is kept whole",
			args: args{query: "SELECT * FROM main.people"},
			want: "main.people",
		},
		{
			name: "a double-quoted name is unquoted",
			args: args{query: `SELECT * FROM "my table"`},
			want: "my table",
		},
		{
			name: "a bracket-quoted name is unquoted",
			args: args{query: "SELECT * FROM [my table]"},
			want: "my table",
		},
		{
			name: "the first from wins over a later one",
			args: args{query: "SELECT * FROM a JOIN b ON a.id = b.id WHERE a.x IN (SELECT x FROM c)"},
			want: "a",
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

	// A table named query_result_* is the user's, not sqly's. sqly once
	// materialized results into tables of that name and filtered them out of
	// every listing; it no longer creates them, so the filter could only ever
	// reach a table the user imported or created.
	t.Run("lists a user table whose name begins with query_result_", func(t *testing.T) {
		t.Parallel()

		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)

		for _, stmt := range []string{
			"CREATE TABLE users (id TEXT, name TEXT)",
			"CREATE TABLE query_result_report (col1 TEXT, col2 TEXT)",
			"CREATE TABLE products (id TEXT, price TEXT)",
		} {
			if _, err := r.Exec(context.Background(), stmt); err != nil {
				t.Fatal(err)
			}
		}

		tables, err := r.TablesName(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		tableNames := make([]string, len(tables))
		for i, table := range tables {
			tableNames[i] = table.Name()
		}

		want := []string{"users", "query_result_report", "products"}
		if diff := cmp.Diff(want, tableNames); diff != "" {
			t.Errorf("TablesName() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("reports a query_result_ table in SchemaObjects", func(t *testing.T) {
		t.Parallel()

		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)
		if _, err := r.Exec(context.Background(), "CREATE TABLE query_result_report (a TEXT)"); err != nil {
			t.Fatal(err)
		}

		tables, err := r.SchemaObjects(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		found := false
		for _, table := range tables {
			if table.Name() == "query_result_report" {
				found = true
			}
		}
		if !found {
			t.Errorf("SchemaObjects() omitted query_result_report; got %v", tables)
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

		if _, err := r.Exec(context.Background(), "CREATE TABLE data (id TEXT, value TEXT)"); err != nil {
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

	t.Run("returns tables in creation order, not alphabetical", func(t *testing.T) {
		t.Parallel()

		memoryDB, cleanup, err := config.NewInMemDB()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		r := NewSQLite3Repository(memoryDB)

		// Create "zebra" before "ant" so creation order and alphabetical order
		// disagree; TablesName preserves creation (import) order.
		for _, name := range []string{"zebra", "ant"} {
			if _, err := r.Exec(context.Background(), "CREATE TABLE "+name+" (id TEXT)"); err != nil {
				t.Fatal(err)
			}
		}

		tables, err := r.TablesName(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(tables))
		for i, table := range tables {
			got[i] = table.Name()
		}
		if len(got) != 2 || got[0] != "zebra" || got[1] != "ant" {
			t.Errorf("TablesName order = %v, want [zebra ant] (creation order)", got)
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
	if _, err := r.Exec(context.Background(), "CREATE TABLE sample (id TEXT, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Exec(context.Background(), "INSERT INTO sample VALUES ('1', 'alice'), ('2', 'bob')"); err != nil {
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

// TestSqlite3Repository_Exec_StatementWithNothingToRun covers a statement the
// driver accepts without running anything: it answers with no result at all, and
// reading a row count off that answer dereferenced nothing. A dialect that
// translates its own comment syntax away is how such a statement reaches here.
func TestSqlite3Repository_Exec_StatementWithNothingToRun(t *testing.T) {
	r := newSampleRepo(t)
	for _, statement := range []string{"", "   ", "-- only a comment", "/* only a comment */", ";"} {
		affected, err := r.Exec(context.Background(), statement)
		if err != nil {
			t.Errorf("Exec(%q) error = %v, want nil", statement, err)
		}
		if affected != 0 {
			t.Errorf("Exec(%q) affected = %d, want 0", statement, affected)
		}
	}
}

// TestSqlite3RepositorySchemaObjects verifies that SchemaObjects lists base
// tables, views, and TEMP tables (so .tables can enumerate everything queryable),
// while TablesName stays limited to base tables used by write-back.
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
// schema-qualified table name, so .dump and .describe can target main.<table>.
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
