//go:build smoke

package e2e

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
)

// residueDiagnostics are the messages a leaked ATTACH, a half-finished
// transaction, or a partial restore produces. They are checked by absence
// rather than by presence: each one names a state the current implementation
// must never leave behind, and each was reachable before this branch.
//
// A run's exit code alone does not catch them. A second run can succeed while
// still printing "database sqly_cache is already in use" on the way, which is
// the shape the attachment leak took.
var residueDiagnostics = []string{
	"already in use",
	"unknown database",
	"no such table",
	"database is locked",
	"transaction has already been committed or rolled back",
}

// assertNoResidueDiagnostics fails when stderr mentions any state left over
// from an earlier run. label says which run produced it, so a failure names the
// step rather than only the string.
func assertNoResidueDiagnostics(t *testing.T, label, stderr string) {
	t.Helper()

	lower := strings.ToLower(stderr)
	for _, phrase := range residueDiagnostics {
		if strings.Contains(lower, phrase) {
			t.Errorf("%s: stderr mentions %q, which points at state left over from an earlier run:\n%s",
				label, phrase, stderr)
		}
	}
}

// parseCSVRecords parses delimited output into records, so a comparison does
// not depend on quoting or on where the writer chose to put a newline. comma
// selects CSV or TSV.
func parseCSVRecords(t *testing.T, label, payload string, comma rune) [][]string {
	t.Helper()

	r := csv.NewReader(strings.NewReader(dropStatusLines(payload)))
	r.Comma = comma
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("%s: parse delimited output: %v\n%s", label, err, payload)
	}
	return records
}

// parseJSONRows decodes JSON array output into rows, so a comparison depends on
// values rather than on key order or whitespace.
func parseJSONRows(t *testing.T, label, payload string) []map[string]any {
	t.Helper()

	var rows []map[string]any
	if err := json.Unmarshal([]byte(dropStatusLines(payload)), &rows); err != nil {
		t.Fatalf("%s: decode json: %v\n%s", label, err, payload)
	}
	return rows
}

// parseTableCells parses sqly's ASCII table output into data rows, dropping the
// rule lines and the header. It exists so the table format can be compared by
// value with the machine-readable ones rather than by substring search, which
// would pass even when a value landed in the wrong column.
//
// A value containing a newline is split across physical lines by the renderer,
// so a caller comparing such a fixture should compare the joined cell text
// rather than expect one line per record.
func parseTableCells(t *testing.T, label, payload string) [][]string {
	t.Helper()

	var rows [][]string
	for _, line := range strings.Split(dropStatusLines(payload), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue // a rule line, or the blank tail
		}
		fields := strings.Split(strings.Trim(trimmed, "|"), "|")
		cells := make([]string, 0, len(fields))
		for _, f := range fields {
			cells = append(cells, strings.TrimSpace(f))
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		t.Fatalf("%s: no table rows found in:\n%s", label, payload)
	}
	return rows[1:] // drop the header row
}

// runQueryAs runs one query through the binary in the given format and fails
// the test when it does not succeed cleanly.
func runQueryAs(t *testing.T, label, format, query string, inputs ...string) string {
	t.Helper()

	args := append([]string{"--output-format", format, "--sql", query}, inputs...)
	stdout, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("%s (%s): exit code = %d\nstderr: %s", label, format, code, stderr)
	}
	assertNoResidueDiagnostics(t, label+" ("+format+")", stderr)
	return stdout
}
