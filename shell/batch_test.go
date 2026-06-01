package shell

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
)

// TestRunBatchReaderLineByLine covers the batch parsing bugs where a helper
// command after a terminated SQL statement (#397) or after a leading SQL comment
// (#425) was absorbed into a pending SQL buffer instead of running as its own
// statement.
func TestRunBatchReaderLineByLine(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "helper command after a terminated statement runs on its own line",
			input: "SELECT 1 AS x;\n.mode csv\nSELECT 2 AS y;\n",
		},
		{
			name:  "helper command after a leading line comment runs on its own line",
			input: "-- header\n.mode json\nSELECT 1 AS x;\n",
		},
		{
			name:  "helper command after a leading block comment runs on its own line",
			input: "/* header */\n.mode json\nSELECT 1 AS x;\n",
		},
	}

	// A dot-line inside an open block comment must stay part of the comment, not be
	// executed as a helper command. Ref CodeRabbit review of #425.
	t.Run("dot line inside an open block comment is not executed as a command", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatalf("newShell: %v", err)
		}
		defer cleanup()
		backout, backerr := config.Stdout, config.Stderr
		var stderr bytes.Buffer
		config.Stdout = &bytes.Buffer{}
		config.Stderr = &stderr
		defer func() { config.Stdout, config.Stderr = backout, backerr }()

		// The ".mode csv" is inside the /* ... */ block, so it is a comment, not a
		// command; only the trailing SELECT runs.
		_, runErr := shell.runBatchReader(context.Background(), strings.NewReader("/* header\n.mode csv\n*/\nSELECT 1 AS x;\n"))
		if runErr != nil {
			t.Errorf("runBatchReader returned error: %v", runErr)
		}
		if strings.Contains(stderr.String(), "mode") || strings.Contains(stderr.String(), "batch statement") {
			t.Errorf("dot-line inside a block comment was executed: stderr=%q", stderr.String())
		}
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shell, cleanup, err := newShell(t, []string{"sqly"})
			if err != nil {
				t.Fatalf("newShell: %v", err)
			}
			defer cleanup()

			backout, backerr := config.Stdout, config.Stderr
			var stderr bytes.Buffer
			config.Stdout = &bytes.Buffer{}
			config.Stderr = &stderr
			defer func() { config.Stdout, config.Stderr = backout, backerr }()

			ranAny, runErr := shell.runBatchReader(context.Background(), strings.NewReader(tc.input))
			if runErr != nil {
				t.Errorf("runBatchReader returned error: %v", runErr)
			}
			if !ranAny {
				t.Errorf("runBatchReader did not run any statement")
			}
			// The bug merged ".mode" with the following SQL into one pseudo-command,
			// surfacing a "got N arguments" error. A clean line-by-line run never does.
			if strings.Contains(stderr.String(), "arguments") || strings.Contains(stderr.String(), "batch statement") {
				t.Errorf("helper command was not parsed on its own line: stderr=%q", stderr.String())
			}
		})
	}
}

// TestScriptModifiesData verifies that write-back intent detection is statement
// aware: an EXPLAIN of a DML statement is read-only (#402, #403), while a real
// DML or a WITH that feeds one is data-modifying.
func TestScriptModifiesData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{"SELECT is read-only", "SELECT * FROM t", false},
		{"EXPLAIN UPDATE is read-only", "EXPLAIN UPDATE t SET x=1", false},
		{"EXPLAIN DELETE is read-only", "EXPLAIN DELETE FROM t", false},
		{"UPDATE modifies", "UPDATE t SET x=1", true},
		{"INSERT modifies", "INSERT INTO t(x) VALUES (1)", true},
		{"DELETE modifies", "DELETE FROM t", true},
		{"REPLACE modifies", "REPLACE INTO t(x) VALUES (1)", true},
		{"WITH feeding UPDATE modifies", "WITH s AS (SELECT 1) UPDATE t SET x=1", true},
		{"WITH feeding SELECT is read-only", "WITH s AS (SELECT 1) SELECT * FROM s", false},
		{"identifier update_log does not match", "SELECT * FROM update_log", false},
		{"multiple statements, one modifies", "SELECT 1;\nUPDATE t SET x=1;", true},
		{"explain then select stays read-only", "EXPLAIN UPDATE t SET x=1;\nSELECT 1;", false},
		// A helper command line is not SQL, so it must not hide a following DML. Ref
		// CodeRabbit review of #397.
		{"helper command before UPDATE still detects modification", ".mode csv\nUPDATE t SET x=1;", true},
		{"helper command before SELECT stays read-only", ".mode csv\nSELECT 1;", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := scriptModifiesData(tt.script); got != tt.want {
				t.Errorf("scriptModifiesData(%q) = %v, want %v", tt.script, got, tt.want)
			}
		})
	}
}

// TestSplitSQLStatements_TriggerBody verifies that a CREATE TRIGGER definition is
// kept as a single statement even though its BEGIN ... END body contains inner
// semicolons, while ordinary semicolon-terminated statements still split. Ref #468.
func TestSplitSQLStatements_TriggerBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantStmts []string
		wantRem   string
	}{
		{
			name:      "trigger with one inner statement stays whole",
			input:     "CREATE TRIGGER tg AFTER UPDATE ON t BEGIN\n  UPDATE t SET y=1 WHERE id=2;\nEND;\n",
			wantStmts: []string{"CREATE TRIGGER tg AFTER UPDATE ON t BEGIN\n  UPDATE t SET y=1 WHERE id=2;\nEND"},
			wantRem:   "\n",
		},
		{
			name:      "trigger with multiple inner statements stays whole",
			input:     "CREATE TRIGGER tg AFTER INSERT ON t BEGIN UPDATE t SET a=1; UPDATE t SET b=2; END;",
			wantStmts: []string{"CREATE TRIGGER tg AFTER INSERT ON t BEGIN UPDATE t SET a=1; UPDATE t SET b=2; END"},
			wantRem:   "",
		},
		{
			name:      "TEMP trigger stays whole",
			input:     "CREATE TEMP TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a=1; END;",
			wantStmts: []string{"CREATE TEMP TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a=1; END"},
			wantRem:   "",
		},
		{
			name:      "trigger body with CASE...END balances correctly",
			input:     "CREATE TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a = CASE WHEN x THEN 1 ELSE 2 END; END; SELECT 1;",
			wantStmts: []string{"CREATE TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a = CASE WHEN x THEN 1 ELSE 2 END; END", "SELECT 1"},
			wantRem:   "",
		},
		{
			name:      "trigger followed by a normal statement splits after END",
			input:     "CREATE TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a=1; END; INSERT INTO t VALUES (1);",
			wantStmts: []string{"CREATE TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a=1; END", "INSERT INTO t VALUES (1)"},
			wantRem:   "",
		},
		{
			name:      "ordinary statements still split on semicolons",
			input:     "UPDATE t SET a=1; UPDATE t SET b=2;",
			wantStmts: []string{"UPDATE t SET a=1", "UPDATE t SET b=2"},
			wantRem:   "",
		},
		{
			name:      "incomplete trigger is left in the remainder, not split",
			input:     "CREATE TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a=1;\n",
			wantStmts: nil,
			wantRem:   "CREATE TRIGGER tg AFTER UPDATE ON t BEGIN UPDATE t SET a=1;\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotStmts, gotRem := splitSQLStatements(tt.input)
			if len(gotStmts) != len(tt.wantStmts) {
				t.Fatalf("splitSQLStatements(%q) stmts = %#v, want %#v", tt.input, gotStmts, tt.wantStmts)
			}
			for i := range gotStmts {
				if gotStmts[i] != tt.wantStmts[i] {
					t.Errorf("stmt[%d] = %q, want %q", i, gotStmts[i], tt.wantStmts[i])
				}
			}
			if gotRem != tt.wantRem {
				t.Errorf("remainder = %q, want %q", gotRem, tt.wantRem)
			}
		})
	}
}

// TestStatementResultMessage verifies that only a data-modifying statement reports
// an affected-row count; a no-rowset DDL/PRAGMA/maintenance statement reports
// neutral success instead of a misleading row count. Ref #439.
func TestStatementResultMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stmt     string
		affected int64
		want     string
	}{
		{"UPDATE reports affected count", "UPDATE t SET x=1", 3, "affected is 3 row(s)\n"},
		{"INSERT reports affected count", "INSERT INTO t VALUES (1)", 1, "affected is 1 row(s)\n"},
		{"DELETE reports affected count", "DELETE FROM t", 2, "affected is 2 row(s)\n"},
		{"WITH feeding UPDATE reports affected count", "WITH s AS (SELECT 1) UPDATE t SET x=1", 5, "affected is 5 row(s)\n"},
		{"CREATE VIEW reports neutral success", "CREATE VIEW v AS SELECT 1", 1, msgStatementExecuted},
		{"CREATE TABLE reports neutral success", "CREATE TABLE t (id INTEGER)", 0, msgStatementExecuted},
		{"DROP TABLE reports neutral success", "DROP TABLE t", 1, msgStatementExecuted},
		{"PRAGMA reports neutral success", "PRAGMA user_version = 1", 1, msgStatementExecuted},
		{"ANALYZE reports neutral success", "ANALYZE", 1, msgStatementExecuted},
		{"REINDEX reports neutral success", "REINDEX", 0, msgStatementExecuted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := statementResultMessage(tt.stmt, tt.affected); got != tt.want {
				t.Errorf("statementResultMessage(%q, %d) = %q, want %q", tt.stmt, tt.affected, got, tt.want)
			}
		})
	}
}

// TestFirstSaveIncompatibleStatement verifies that a non-interactive save run is
// allowed only for read-only queries and row-modifying DML; any DDL, schema, or
// maintenance statement is reported as save-incompatible so the run fails loudly
// instead of exiting 0 while leaving the source unchanged. Ref #433-#437,
// #469-#484.
func TestFirstSaveIncompatibleStatement(t *testing.T) {
	t.Parallel()

	compatible := []string{
		"SELECT * FROM user",
		"UPDATE user SET first_name='Z' WHERE id=1",
		"INSERT INTO user VALUES (1)",
		"DELETE FROM user WHERE id=1",
		"REPLACE INTO user VALUES (1)",
		"WITH s AS (SELECT 1) UPDATE user SET x=1",
		".import testdata/user.csv\nUPDATE user SET first_name='Z' WHERE id=1;",
		"-- a comment\nUPDATE user SET x=1;",
	}
	for _, script := range compatible {
		t.Run("compatible: "+firstLine(script), func(t *testing.T) {
			t.Parallel()
			if got := firstSaveIncompatibleStatement(script); got != "" {
				t.Errorf("firstSaveIncompatibleStatement(%q) = %q, want \"\"", script, got)
			}
		})
	}

	incompatible := []string{
		"ALTER TABLE user RENAME COLUMN first_name TO fname",
		"DROP TABLE user",
		"CREATE TABLE backup (id INTEGER)",
		"CREATE TABLE backup AS SELECT * FROM user",
		"CREATE VIEW v AS SELECT user_name FROM user",
		"CREATE INDEX idx ON user(identifier)",
		"DROP VIEW v",
		"DROP INDEX idx",
		"REINDEX",
		"ANALYZE",
		"CREATE TRIGGER tg AFTER UPDATE ON user BEGIN UPDATE user SET x=1; END;",
		"CREATE TABLE backup AS SELECT * FROM user;\nUPDATE user SET first_name='Z' WHERE id=1;",
	}
	for _, script := range incompatible {
		t.Run("incompatible: "+firstLine(script), func(t *testing.T) {
			t.Parallel()
			if got := firstSaveIncompatibleStatement(script); got == "" {
				t.Errorf("firstSaveIncompatibleStatement(%q) = \"\", want a non-empty incompatible statement", script)
			}
		})
	}
}

// TestScriptImportsInput verifies detection of a .import helper command, which
// lets save preflight defer write-back validation until after the import runs.
// Ref #456.
func TestScriptImportsInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{"import then DML", ".import testdata/user.csv\nUPDATE user SET x=1;", true},
		{"import with leading spaces", "   .import testdata/user.csv\n", true},
		{"no import", "UPDATE user SET x=1;", false},
		{"other dot command only", ".mode csv\nSELECT 1;", false},
		{"importer-like prefix is not .import", ".importance 1\n", false},
		{"import inside a SQL value is not a command", "INSERT INTO t VALUES ('.import x');", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := scriptImportsInput(tt.script); got != tt.want {
				t.Errorf("scriptImportsInput(%q) = %v, want %v", tt.script, got, tt.want)
			}
		})
	}
}

// firstLine returns the first line of s, used to build readable subtest names.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
