package interactor

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/config"
	ifilesql "github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/memory"
)

func TestSQLite3Interactor_Dialect(t *testing.T) {
	t.Parallel()
	si := &SQLite3Interactor{}
	// The zero value reports SQLite, matching the no-translation default.
	if got := si.Dialect(); got != dialect.SQLite {
		t.Fatalf("zero-value Dialect() = %q, want sqlite", got)
	}
	si.SetDialect(dialect.MySQL)
	if got := si.Dialect(); got != dialect.MySQL {
		t.Fatalf("Dialect() = %q, want mysql", got)
	}
	// An empty dialect resets to SQLite rather than becoming an unknown dialect.
	si.SetDialect("")
	if got := si.Dialect(); got != dialect.SQLite {
		t.Fatalf("SetDialect(\"\") then Dialect() = %q, want sqlite", got)
	}
}

// TestSQLite3Interactor_ExecSQLTranslates verifies ExecSQL translates a user
// query from the configured dialect before running it, and that the dialect
// helper functions are available.
func TestSQLite3Interactor_ExecSQLTranslates(t *testing.T) {
	config.InitSQLite3()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER, name TEXT)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO t VALUES (1, 'alice'), (2, 'bob')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	si := NewSQLite3Interactor(
		memory.NewSQLite3Repository(db),
		NewSQL(),
		ifilesql.NewFileSQLAdapter(db),
	)

	// MySQL: backtick identifiers and the IF helper function.
	si.SetDialect(dialect.MySQL)
	table, _, err := si.ExecSQL(ctx, "SELECT `name`, IF(`id` = 1, 'first', 'other') AS tag FROM `t` ORDER BY `id`")
	if err != nil {
		t.Fatalf("ExecSQL(mysql): %v", err)
	}
	if len(table.Records()) != 2 {
		t.Fatalf("got %d rows, want 2", len(table.Records()))
	}
	if got := table.Records()[0][0]; got != "alice" {
		t.Fatalf("row0 name = %q, want alice", got)
	}
	if got := table.Records()[0][1]; got != "first" {
		t.Fatalf("row0 tag = %q, want first", got)
	}

	// An unsupported construct surfaces as an error rather than a wrong result.
	si.SetDialect(dialect.PostgreSQL)
	if _, _, err := si.ExecSQL(ctx, "SELECT DISTINCT ON (name) name FROM t"); err == nil {
		t.Fatal("ExecSQL with DISTINCT ON should error")
	}

	// Back on SQLite, the same query runs untranslated.
	si.SetDialect(dialect.SQLite)
	if _, _, err := si.ExecSQL(ctx, "SELECT name FROM t"); err != nil {
		t.Fatalf("ExecSQL(sqlite): %v", err)
	}
}

// TestSQLite3Interactor_ExecSQLRejectsStatementTranslatedToNothing covers a
// statement that is only a comment in its own dialect. MySQL and GoogleSQL write
// a line comment with "#", which SQLite does not, so the check for an empty
// statement made before the translation still sees something to run. What the
// translation leaves is empty, and running that reached the driver, which
// answered with no result and crashed the row count. The dialect that named the
// comment is the one that has to notice it.
func TestSQLite3Interactor_ExecSQLRejectsStatementTranslatedToNothing(t *testing.T) {
	config.InitSQLite3()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	si := NewSQLite3Interactor(
		memory.NewSQLite3Repository(db),
		NewSQL(),
		ifilesql.NewFileSQLAdapter(db),
	)

	tests := []struct {
		name      string
		dialect   dialect.Dialect
		statement string
	}{
		{name: "mysql hash comment", dialect: dialect.MySQL, statement: "# hello"},
		{name: "mysql bare hash", dialect: dialect.MySQL, statement: "#"},
		{name: "mysql block comment then hash comment", dialect: dialect.MySQL, statement: "/* c */ # d"},
		{name: "googlesql hash comment", dialect: dialect.GoogleSQL, statement: "# hello"},
		{name: "googlesql bare hash", dialect: dialect.GoogleSQL, statement: "#"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			si.SetDialect(tt.dialect)
			table, affected, err := si.ExecSQL(context.Background(), tt.statement)
			if err == nil {
				t.Fatalf("ExecSQL(%q) = (%v, %d, nil), want an error", tt.statement, table, affected)
			}
			if !strings.Contains(err.Error(), "no executable SQL statement") {
				t.Errorf("ExecSQL(%q) error = %v, want it to say there is no executable statement", tt.statement, err)
			}
		})
	}
}
