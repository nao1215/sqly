package interactor

import (
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestHasReturningClause exercises hasReturningClause across DML statements with
// and without a RETURNING clause, plus cases where the word appears inside string
// literals, quoted identifiers, and comments where it must be ignored.
func TestHasReturningClause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stmt string
		want bool
	}{
		{"insert without returning", "INSERT INTO t(a) VALUES (1)", false},
		{"insert with returning", "INSERT INTO t(a) VALUES (1) RETURNING a", true},
		{"update with returning", "UPDATE t SET a = 1 RETURNING *", true},
		{"delete with returning", "DELETE FROM t WHERE a = 1 RETURNING id", true},
		{"lowercase returning", "insert into t(a) values (1) returning a", true},
		{"mixed case returning", "Insert Into t(a) Values (1) ReTurNiNg a", true},
		{"returning in single quotes", "INSERT INTO t(a) VALUES ('returning')", false},
		{"returning in double-quoted identifier", `INSERT INTO t("returning") VALUES (1)`, false},
		{"returning in backtick identifier", "INSERT INTO t(`returning`) VALUES (1)", false},
		{"returning in bracket identifier", "INSERT INTO t([returning]) VALUES (1)", false},
		{"returning in line comment", "INSERT INTO t(a) VALUES (1) -- returning a\n", false},
		{"returning in block comment", "INSERT INTO t(a) VALUES (1) /* returning a */", false},
		{"word boundary not matched", "INSERT INTO t(returning_at) VALUES (1)", false},
		{"returning after real clause survives comment", "UPDATE t SET a=1 -- note\n RETURNING a", true},
		{"empty statement", "", false},
		{"whitespace only", "   \n\t ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasReturningClause(tt.stmt); got != tt.want {
				t.Errorf("hasReturningClause(%q) = %v, want %v", tt.stmt, got, tt.want)
			}
		})
	}
}

// TestMainStatementVerb exercises mainStatementVerb across plain DML/queries and
// WITH (CTE) prefixed statements, verifying that CTE bodies inside parentheses
// are skipped and the verb of the feeding statement is returned.
func TestMainStatementVerbCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stmt string
		want string
	}{
		{"plain select", "SELECT * FROM t", "SELECT"},
		{"plain insert", "INSERT INTO t VALUES (1)", "INSERT"},
		{"plain update", "UPDATE t SET a = 1", "UPDATE"},
		{"plain delete", "DELETE FROM t", "DELETE"},
		{"plain replace", "REPLACE INTO t VALUES (1)", "REPLACE"},
		{"plain values", "VALUES (1),(2)", "VALUES"},
		{"lowercase select", "select * from t", "SELECT"},
		{"with cte feeding select", "WITH c AS (SELECT 1) SELECT * FROM c", "SELECT"},
		{"with cte feeding update", "WITH c AS (SELECT 1) UPDATE t SET a = (SELECT 1 FROM c)", "UPDATE"},
		{"with cte feeding delete", "WITH c AS (SELECT id FROM t) DELETE FROM t WHERE id IN (SELECT id FROM c)", "DELETE"},
		{"with cte feeding insert", "WITH c AS (SELECT 1 AS x) INSERT INTO t SELECT x FROM c", "INSERT"},
		{"verb inside string ignored", "SELECT 'INSERT' FROM t", "SELECT"},
		{"verb inside double quotes ignored", `SELECT "update" FROM t`, "SELECT"},
		{"verb inside comment ignored", "/* UPDATE */ SELECT 1", "SELECT"},
		{"verb inside line comment ignored", "-- UPDATE\nSELECT 1", "SELECT"},
		{"nested parens skipped", "WITH c AS (SELECT (SELECT 1)) SELECT * FROM c", "SELECT"},
		{"leading whitespace", "   \n  SELECT 1", "SELECT"},
		{"no verb found", "PRAGMA table_info(t)", ""},
		{"empty statement", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mainStatementVerb(tt.stmt); got != tt.want {
				t.Errorf("mainStatementVerb(%q) = %q, want %q", tt.stmt, got, tt.want)
			}
		})
	}
}

// TestSQLite3InteractorMalformedRowPolicy verifies that a policy set with
// SetMalformedRowPolicy is reported back by MalformedRowPolicy through the
// filesql adapter.
func TestSQLite3InteractorMalformedRowPolicy(t *testing.T) {
	t.Parallel()

	si, cleanup := newTestSQLite3InteractorWithAdapter(t)
	defer cleanup()

	policies := []model.MalformedRowPolicy{
		model.MalformedRowStop,
		model.MalformedRowSkip,
		model.MalformedRowFill,
	}
	for _, want := range policies {
		si.SetMalformedRowPolicy(want)
		if got := si.MalformedRowPolicy(); got != want {
			t.Errorf("MalformedRowPolicy() = %v, want %v", got, want)
		}
	}
}

// TestWithCompressedWriterErrors covers the error branches of withCompressedWriter:
// a failing file creation, a rejected compression codec, and a write function that
// returns an error.
func TestWithCompressedWriterErrors(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("t", model.NewHeader([]string{"a"}), []model.Record{
		model.NewRecord([]string{"1"}),
	})

	t.Run("create failure on nonexistent directory", func(t *testing.T) {
		t.Parallel()
		badPath := filepath.Join(t.TempDir(), "no-such-dir", "out.csv")
		if err := exp.DumpTable(badPath, table, model.ExportCSV, model.CompressionNone); err == nil {
			t.Fatal("DumpTable() = nil error, want error when output directory does not exist")
		}
	})

	t.Run("compression init failure for write-only-unsupported codec", func(t *testing.T) {
		t.Parallel()
		out := filepath.Join(t.TempDir(), "out.csv.bz2")
		if err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionBzip2); err == nil {
			t.Fatal("DumpTable() = nil error, want error when compression codec rejects writing")
		}
	})

	t.Run("write failure from duplicate JSON columns", func(t *testing.T) {
		t.Parallel()
		dup := model.NewTable("t", model.NewHeader([]string{"a", "a"}), []model.Record{
			model.NewRecord([]string{"1", "2"}),
		})
		out := filepath.Join(t.TempDir(), "out.json")
		if err := exp.DumpTable(out, dup, model.ExportJSON, model.CompressionNone); err == nil {
			t.Fatal("DumpTable() = nil error, want error when JSON columns are not unique")
		}
	})
}
