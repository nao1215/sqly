//go:build smoke

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// typedFixture is the batch script that builds one row covering every type
// distinction the JSON contract has to keep. It is written as SQL rather than as
// a CSV so each column's declared type is chosen here rather than inferred by
// the importer, which is what makes "this TEXT column holds 123" expressible.
const typedFixture = `CREATE TABLE t (
  int_col INTEGER,
  real_col REAL,
  text_num TEXT,
  text_bool TEXT,
  leading_zero TEXT,
  empty_col TEXT,
  null_col TEXT,
  neg_col INTEGER,
  zero_col INTEGER
);
INSERT INTO t VALUES (42, 1.5, '123', 'true', '00123', '', NULL, -7, 0);
`

// decodeOneJSONRow runs a query in the given output format and decodes the
// single result row.
func decodeOneJSONRow(t *testing.T, format, query string) map[string]any {
	t.Helper()

	stdout, stderr, code := run(t, typedFixture+query+"\n", "--output-format", format)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%q)", code, stderr)
	}
	return decodeJSONRow(t, format, dropStatusLines(stdout))
}

// dropStatusLines removes the per-statement status lines batch mode prints for
// the CREATE TABLE and INSERT that set the fixture up, leaving only the query
// result. Those lines are the documented batch contract for a statement that
// returns no rowset; the assertions here are about the rowset.
func dropStatusLines(out string) string {
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "statement executed successfully":
			continue
		case strings.HasPrefix(line, "affected is ") && strings.HasSuffix(line, "row(s)"):
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// decodeJSONRow decodes one row out of a JSON array or an NDJSON stream.
func decodeJSONRow(t *testing.T, format, payload string) map[string]any {
	t.Helper()

	if format == "jsonl" {
		lines := nonEmptyLines(payload)
		if len(lines) != 1 {
			t.Fatalf("ndjson produced %d lines, want 1: %q", len(lines), payload)
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
			t.Fatalf("decode ndjson line: %v (%q)", err, lines[0])
		}
		return row
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(payload), &rows); err != nil {
		t.Fatalf("decode json: %v (%q)", err, payload)
	}
	if len(rows) != 1 {
		t.Fatalf("json produced %d rows, want 1: %q", len(rows), payload)
	}
	return rows[0]
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// TestSmoke_JSONTypeContract is the type table the v1.0.0 JSON contract
// promises, asserted on the decoded Go value rather than on the printed text.
// A string comparison would pass for `"int_col":"42"` as readily as for
// `"int_col":42`, which is the very confusion the contract exists to remove.
func TestSmoke_JSONTypeContract(t *testing.T) {
	for _, format := range []string{"json", "jsonl"} {
		t.Run(format, func(t *testing.T) {
			row := decodeOneJSONRow(t, format, "SELECT * FROM t;")

			checks := []struct {
				column string
				check  func(t *testing.T, got any)
			}{
				{"int_col", func(t *testing.T, got any) {
					n, ok := got.(float64)
					if !ok || n != 42 {
						t.Errorf("int_col = %#v (%T), want JSON number 42", got, got)
					}
				}},
				{"real_col", func(t *testing.T, got any) {
					n, ok := got.(float64)
					if !ok || n != 1.5 {
						t.Errorf("real_col = %#v (%T), want JSON number 1.5", got, got)
					}
				}},
				{"neg_col", func(t *testing.T, got any) {
					n, ok := got.(float64)
					if !ok || n != -7 {
						t.Errorf("neg_col = %#v (%T), want JSON number -7", got, got)
					}
				}},
				{"zero_col", func(t *testing.T, got any) {
					n, ok := got.(float64)
					if !ok || n != 0 {
						t.Errorf("zero_col = %#v (%T), want JSON number 0", got, got)
					}
				}},
				{"text_num", func(t *testing.T, got any) {
					if got != "123" {
						t.Errorf("text_num = %#v (%T), want the string \"123\"", got, got)
					}
				}},
				{"text_bool", func(t *testing.T, got any) {
					if got != "true" {
						t.Errorf("text_bool = %#v (%T), want the string \"true\"", got, got)
					}
				}},
				{"leading_zero", func(t *testing.T, got any) {
					if got != "00123" {
						t.Errorf("leading_zero = %#v (%T), want \"00123\" with its zeros", got, got)
					}
				}},
				{"empty_col", func(t *testing.T, got any) {
					if got != "" {
						t.Errorf("empty_col = %#v (%T), want the empty string", got, got)
					}
				}},
				{"null_col", func(t *testing.T, got any) {
					if got != nil {
						t.Errorf("null_col = %#v (%T), want JSON null", got, got)
					}
				}},
			}
			for _, c := range checks {
				t.Run(c.column, func(t *testing.T) { c.check(t, row[c.column]) })
			}

			// NULL and "" decode to different Go values, and both are present:
			// the key must exist even when its value is null.
			if _, ok := row["null_col"]; !ok {
				t.Error("null_col key is missing; a NULL must be emitted, not skipped")
			}
			if row["null_col"] == row["empty_col"] {
				t.Error("NULL and the empty string decoded to the same value")
			}
		})
	}
}

// TestSmoke_JSONNumbersAreExactWithUseNumber re-decodes with UseNumber, so a
// number is compared as the literal sqly wrote rather than after float64
// rounding. An INTEGER larger than 2^53 survives this check only if it was
// emitted as a JSON number literal and not reformatted through a float.
func TestSmoke_JSONNumbersAreExactWithUseNumber(t *testing.T) {
	script := "CREATE TABLE big (id INTEGER, ratio REAL);\n" +
		"INSERT INTO big VALUES (9007199254740993, 0.1);\n" +
		"SELECT * FROM big;\n"
	stdout, stderr, code := run(t, script, "--output-format", "jsonl")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
	}

	dec := json.NewDecoder(bytes.NewReader([]byte(dropStatusLines(stdout))))
	dec.UseNumber()
	var row map[string]any
	if err := dec.Decode(&row); err != nil {
		t.Fatalf("decode: %v (%q)", err, stdout)
	}
	id, ok := row["id"].(json.Number)
	if !ok {
		t.Fatalf("id = %#v (%T), want json.Number", row["id"], row["id"])
	}
	if id.String() != "9007199254740993" {
		t.Errorf("id = %s, want 9007199254740993 exactly (no float rounding)", id)
	}
	ratio, ok := row["ratio"].(json.Number)
	if !ok {
		t.Fatalf("ratio = %#v (%T), want json.Number", row["ratio"], row["ratio"])
	}
	if ratio.String() != "0.1" {
		t.Errorf("ratio = %s, want 0.1", ratio)
	}
}

// TestSmoke_NDJSONKeepsTypesOnEveryLine checks that the type contract holds per
// line, not only on the first one. A per-row formatter that reused state could
// pass a single-row test and still degrade later rows.
func TestSmoke_NDJSONKeepsTypesOnEveryLine(t *testing.T) {
	script := "CREATE TABLE rows_t (n INTEGER, s TEXT);\n" +
		"INSERT INTO rows_t VALUES (1, '001'), (2, '002'), (3, NULL);\n" +
		"SELECT * FROM rows_t ORDER BY n;\n"
	stdout, stderr, code := run(t, script, "--output-format", "jsonl")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
	}

	lines := nonEmptyLines(dropStatusLines(stdout))
	if len(lines) != 3 {
		t.Fatalf("got %d ndjson lines, want 3: %q", len(lines), stdout)
	}
	wantStrings := []any{"001", "002", nil}
	for i, line := range lines {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("line %d: decode: %v (%q)", i, err, line)
		}
		if n, ok := row["n"].(float64); !ok || int(n) != i+1 {
			t.Errorf("line %d: n = %#v (%T), want JSON number %d", i, row["n"], row["n"], i+1)
		}
		if row["s"] != wantStrings[i] {
			t.Errorf("line %d: s = %#v, want %#v", i, row["s"], wantStrings[i])
		}
	}
}

// TestSmoke_TextFormatsUnchanged pins the exact CSV, TSV, and table rendering of
// the typed row. The Cell representation derives these strings from the same
// value JSON encodes, so this is the assertion that would catch a change to that
// derivation leaking into the human-facing formats.
func TestSmoke_TextFormatsUnchanged(t *testing.T) {
	query := "SELECT int_col, real_col, text_num, leading_zero, empty_col, null_col FROM t;\n"

	t.Run("csv", func(t *testing.T) {
		stdout, stderr, code := run(t, typedFixture+query, "--output-format", "csv")
		if code != 0 {
			t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
		}
		want := "int_col,real_col,text_num,leading_zero,empty_col,null_col\n42,1.5,123,00123,,\n"
		if got := dropStatusLines(stdout); got != want {
			t.Errorf("csv =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("tsv", func(t *testing.T) {
		stdout, stderr, code := run(t, typedFixture+query, "--output-format", "tsv")
		if code != 0 {
			t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
		}
		want := "int_col\treal_col\ttext_num\tleading_zero\tempty_col\tnull_col\n42\t1.5\t123\t00123\t\t\n"
		if got := dropStatusLines(stdout); got != want {
			t.Errorf("tsv =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("table keeps the leading zeros and shows NULL as blank", func(t *testing.T) {
		stdout, stderr, code := run(t, typedFixture+query, "--output-format", "table")
		if code != 0 {
			t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
		}
		if !strings.Contains(stdout, "00123") {
			t.Errorf("table output lost the leading zeros:\n%s", stdout)
		}
		if strings.Contains(stdout, "NULL") {
			t.Errorf("table output printed the literal NULL, which would be indistinguishable from the string:\n%s", stdout)
		}
		for _, want := range []string{"int_col", "42", "1.5"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("table output missing %q:\n%s", want, stdout)
			}
		}
	})
}

// TestSmoke_StdinImportKeepsTypes checks the piped-input path. The importer
// gives a CSV column the type its values imply, and sqly then reports whatever
// SQLite holds: a plain integer column becomes a JSON number, while a
// zero-padded code and a boolean-looking word stay TEXT and therefore stay JSON
// strings. Emitting 00123 as the number 123 is the failure this pins — it is
// silent, and it destroys the identifier.
func TestSmoke_StdinImportKeepsTypes(t *testing.T) {
	stdin := "code,qty,flag\n00123,42,true\n"
	stdout, stderr, code := run(t, stdin,
		"--stdin-format", "csv", "--output-format", "jsonl", "--sql", "SELECT * FROM stdin")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
	}
	row := decodeJSONRow(t, "jsonl", dropStatusLines(stdout))

	if row["code"] != "00123" {
		t.Errorf("code = %#v (%T), want the string \"00123\" with its leading zeros", row["code"], row["code"])
	}
	if row["flag"] != "true" {
		t.Errorf("flag = %#v (%T), want the string \"true\" (SQLite has no boolean type)", row["flag"], row["flag"])
	}
	if n, ok := row["qty"].(float64); !ok || n != 42 {
		t.Errorf("qty = %#v (%T), want JSON number 42 for an inferred INTEGER column", row["qty"], row["qty"])
	}
}

// TestSmoke_PipedOutputIsPureData checks stdout purity for a pipeline. Progress
// banners, warnings, and mode changes all belong on stderr; a single stray line
// on stdout breaks `sqly ... | jq`.
func TestSmoke_PipedOutputIsPureData(t *testing.T) {
	csv := filepath.Join("testdata", "user.csv")
	stdout, _, code := run(t, ".mode jsonl\nSELECT first_name FROM user ORDER BY first_name LIMIT 2;\n", csv)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	// No dropStatusLines here on purpose: a pure-SELECT session must put nothing
	// but data on stdout, so every line has to parse as JSON exactly as read.
	for i, line := range nonEmptyLines(stdout) {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Errorf("stdout line %d is not JSON: %q", i, line)
		}
	}
}

// TestSmoke_UnicodeAndSymbolNames checks that a file name with a space, a
// symbol, and non-ASCII characters imports and queries, and that the resulting
// JSON keys survive encoding.
func TestSmoke_UnicodeAndSymbolNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "売上 データ-2024.csv")
	if err := os.WriteFile(path, []byte("商品名,個数\nりんご,3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "--output-format", "jsonl",
		"--sql", `SELECT "商品名", "個数" FROM 売上_データ_2024`, path)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	row := decodeJSONRow(t, "jsonl", dropStatusLines(stdout))
	if row["商品名"] != "りんご" {
		t.Errorf("商品名 = %#v, want りんご", row["商品名"])
	}
	if n, ok := row["個数"].(float64); !ok || n != 3 {
		t.Errorf("個数 = %#v (%T), want JSON number 3", row["個数"], row["個数"])
	}
}

// TestSmoke_RepeatedRunsAreIdempotent runs the same command twice and requires
// byte-identical stdout. Output that embeds a timestamp, a random table name, or
// a map iteration order would differ here.
func TestSmoke_RepeatedRunsAreIdempotent(t *testing.T) {
	csv := filepath.Join("testdata", "user.csv")
	args := []string{"--output-format", "json", "--sql", "SELECT * FROM user ORDER BY identifier", csv}

	first, _, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("first run exit code = %d", code)
	}
	second, _, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("second run exit code = %d", code)
	}
	if first != second {
		t.Errorf("repeated runs differ:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if strings.Contains(first, "query_result_") {
		t.Errorf("output leaked an internal generated table name:\n%s", first)
	}
}
