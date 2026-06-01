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
