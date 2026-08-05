package interactor

import (
	"testing"
)

func TestSQLProducesRowset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stmt string
		want bool
	}{
		// Rowset-producing statements run on the query path.
		{"SELECT produces rows", "SELECT 1 AS x", true},
		{"VALUES produces rows", "VALUES (1), (2)", true},
		{"TABLE shorthand produces rows", "TABLE user", true},
		{"EXPLAIN produces rows", "EXPLAIN SELECT 1", true},
		{"EXPLAIN of DML produces rows", "EXPLAIN UPDATE user SET x=1", true},
		{"PRAGMA produces rows", "PRAGMA table_info(user)", true},
		{"WITH feeding SELECT produces rows", "WITH c AS (SELECT 1 AS id) SELECT * FROM c", true},
		{"WITH feeding VALUES produces rows", "WITH c AS (SELECT 1) VALUES (1)", true},
		{"lowercase select produces rows", "select 1", true},
		{"leading line comment then SELECT", "-- header\nSELECT 1", true},
		{"leading block comment then SELECT", "/* header */ SELECT 1", true},
		{"leading BOM then SELECT", "\ufeffSELECT 1", true},
		// RETURNING turns DML into a rowset.
		{"INSERT RETURNING produces rows", "INSERT INTO t(id) VALUES (1) RETURNING id", true},
		{"UPDATE RETURNING produces rows", "UPDATE t SET x=1 RETURNING *", true},
		{"DELETE RETURNING produces rows", "DELETE FROM t RETURNING *", true},
		{"WITH ... UPDATE RETURNING produces rows", "WITH s AS (SELECT 1 AS id) UPDATE t SET x=1 WHERE id IN (SELECT id FROM s) RETURNING *", true},

		// Non-rowset statements run on the exec path.
		{"INSERT without RETURNING is exec", "INSERT INTO t(id) VALUES (1)", false},
		{"UPDATE without RETURNING is exec", "UPDATE t SET x=1", false},
		{"DELETE without RETURNING is exec", "DELETE FROM t", false},
		{"REPLACE without RETURNING is exec", "REPLACE INTO t(id) VALUES (1)", false},
		{"WITH ... UPDATE without RETURNING is exec", "WITH s AS (SELECT 1 AS id) UPDATE t SET x=1 WHERE id IN (SELECT id FROM s)", false},
		{"WITH ... INSERT without RETURNING is exec", "WITH s AS (SELECT 2 AS id, 'b' AS name) INSERT INTO t SELECT * FROM s", false},
		{"WITH ... DELETE without RETURNING is exec", "WITH d AS (SELECT 1 AS id) DELETE FROM t WHERE id IN (SELECT id FROM d)", false},
		{"CREATE is exec", "CREATE TABLE t(x)", false},
		{"DROP is exec", "DROP TABLE t", false},
		{"ALTER is exec", "ALTER TABLE t ADD COLUMN y", false},
		{"BEGIN is exec", "BEGIN", false},
		{"COMMIT is exec", "COMMIT", false},
		{"ATTACH is exec", "ATTACH DATABASE ':memory:' AS aux", false},
		{"ANALYZE is exec", "ANALYZE", false},
		// A literal 'returning' inside a string is not the RETURNING clause.
		{"INSERT with literal returning value is exec", "INSERT INTO t(note) VALUES ('returning soon')", false},
	}

	sql := NewSQL()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.producesRowset(tt.stmt); got != tt.want {
				t.Errorf("producesRowset(%q) = %v, want %v", tt.stmt, got, tt.want)
			}
		})
	}
}

// TestUnsupportedStatementReason verifies that statements sqly cannot run under
// its per-statement transaction and in-memory model are flagged with a reason,
// while statements it can run (DML, DDL, ANALYZE, PRAGMA, queries) are not.
func TestUnsupportedStatementReason(t *testing.T) {
	t.Parallel()

	unsupported := []string{
		"BEGIN",
		"BEGIN IMMEDIATE",
		"BEGIN EXCLUSIVE",
		"COMMIT",
		"END",             // END is an alias for COMMIT
		"END TRANSACTION", // END TRANSACTION is an alias for COMMIT
		"end",             // lowercase
		"ROLLBACK",
		"ROLLBACK TO sp",
		"SAVEPOINT sp",
		"RELEASE sp",
		"VACUUM",
		"VACUUM INTO 'dump.db'",
		"ATTACH DATABASE ':memory:' AS aux",
		"DETACH DATABASE aux",
		"  attach database 'x' as y", // leading space, lowercase
	}
	for _, stmt := range unsupported {
		t.Run("rejected: "+stmt, func(t *testing.T) {
			t.Parallel()
			if reason := unsupportedStatementReason(stmt); reason == "" {
				t.Errorf("unsupportedStatementReason(%q) = \"\", want a non-empty reason", stmt)
			}
		})
	}

	supported := []string{
		"SELECT 1",
		"INSERT INTO t VALUES (1)",
		"UPDATE t SET x = 1",
		"DELETE FROM t",
		"CREATE TABLE t (id INTEGER)",
		"CREATE VIEW v AS SELECT 1",
		"CREATE TRIGGER tg AFTER UPDATE ON t BEGIN SELECT 1; END",
		"DROP TABLE t",
		"ALTER TABLE t ADD COLUMN c INTEGER",
		"REINDEX",
		"ANALYZE",
		"PRAGMA user_version = 1",
		"REPLACE INTO t VALUES (1)",
	}
	for _, stmt := range supported {
		t.Run("supported: "+stmt, func(t *testing.T) {
			t.Parallel()
			if reason := unsupportedStatementReason(stmt); reason != "" {
				t.Errorf("unsupportedStatementReason(%q) = %q, want \"\"", stmt, reason)
			}
		})
	}
}
