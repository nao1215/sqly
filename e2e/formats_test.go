//go:build smoke

package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSmoke_MixedFormatInputs loads three formats in one invocation and treats
// the resulting tables as peers. Joining across formats is what sqly is for, so
// the case worth pinning is not that each file parses but that a CSV row and a
// JSON row meet in one result with their values intact.
//
// This is the cold-import half of the multi-input story; the atomicity half is
// below.
func TestSmoke_MixedFormatInputs(t *testing.T) {
	dir := t.TempDir()
	people := writeFixture(t, dir, "people.csv", "id,name,score\n1,alice,9.5\n2,bob,\n")
	cities := writeFixture(t, dir, "cities.json",
		`[{"id":1,"city":"東京"},{"id":2,"city":"osaka"}]`)
	teams := writeFixture(t, dir, "teams.tsv", "id\tteam\n1\tred\n2\tblue\n")

	t.Run("each input becomes a queryable table", func(t *testing.T) {
		for table, wantRows := range map[string]int{"people": 2, "cities": 2, "teams": 2} {
			out := runQueryAs(t, "count "+table, "csv",
				"SELECT COUNT(*) AS n FROM "+table, people, cities, teams)
			records := parseCSVRecords(t, table, out, ',')
			if len(records) != 2 || records[1][0] != strconv.Itoa(wantRows) {
				t.Errorf("%s row count = %v, want %d", table, records, wantRows)
			}
		}
	})

	// A JSON document keeps its shape in a `data` column rather than being
	// flattened into guessed columns, so a query reaches its fields with
	// json_extract. This is the idiom the formats page and the cookbook show,
	// so the join below doubles as a check that those examples still work.
	t.Run("a join across all three formats keeps every value", func(t *testing.T) {
		out := runQueryAs(t, "three-way join", "json",
			`SELECT p.name, p.score,
			        json_extract(c.data, '$.city') AS city,
			        t.team
			 FROM people p
			 JOIN cities c ON p.id = json_extract(c.data, '$.id')
			 JOIN teams  t ON p.id = t.id
			 ORDER BY p.id`,
			people, cities, teams)

		rows := parseJSONRows(t, "three-way join", out)
		if len(rows) != 2 {
			t.Fatalf("joined %d rows, want 2", len(rows))
		}
		if rows[0]["name"] != "alice" || rows[0]["team"] != "red" {
			t.Errorf("row 0 = %#v, want alice/red", rows[0])
		}
		// The JSON input's non-ASCII value survives the join and the encoding.
		if rows[0]["city"] != "東京" {
			t.Errorf("city = %#v, want 東京", rows[0]["city"])
		}
		// A REAL stays a number, and the blank CSV cell stays distinguishable
		// from a value: this is where a record/cell divergence would show.
		if score, ok := rows[0]["score"].(float64); !ok || score != 9.5 {
			t.Errorf("score = %#v (%T), want the number 9.5", rows[0]["score"], rows[0]["score"])
		}
		if rows[1]["score"] != nil && rows[1]["score"] != "" {
			t.Errorf("missing score = %#v, want null or the empty string", rows[1]["score"])
		}
	})

	t.Run("a later bad input rolls back the earlier ones", func(t *testing.T) {
		broken := writeFixture(t, dir, "broken.json", "{ this is not json")
		outPath := filepath.Join(dir, "must-not-exist.csv")

		stdout, stderr, code := run(t, "",
			"--output-format", "csv",
			"--sql", "SELECT COUNT(*) FROM people",
			"--output", outPath,
			people, cities, broken)

		if code == 0 {
			t.Fatalf("exit code = 0 with a broken third input (stdout=%q)", stdout)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Errorf("stdout = %q, want it empty on a failed import", stdout)
		}
		if !strings.Contains(stderr, "broken") {
			t.Errorf("stderr = %q, want it to name the input that failed", stderr)
		}
		assertNoPanic(t, stdout, stderr)
		if fileExists(outPath) {
			t.Error("--output file was written even though the import failed")
		}

		// The two good inputs still load on their own, so the failure left
		// nothing behind that a retry trips over.
		out := runQueryAs(t, "retry after rollback", "csv",
			"SELECT COUNT(*) AS n FROM people", people, cities)
		if records := parseCSVRecords(t, "retry", out, ','); records[1][0] != "2" {
			t.Errorf("retry row count = %v, want 2", records)
		}
	})
}

// agreementFixture is one row per awkward value class, so a divergence between
// the display strings and the native cells shows up somewhere in it. Each column
// is a value that a naive formatter gets wrong in its own way.
const agreementFixture = `CREATE TABLE t (
  empty_col   TEXT,
  null_col    TEXT,
  int_col     INTEGER,
  real_col    REAL,
  unicode_col TEXT,
  comma_col   TEXT,
  tab_col     TEXT,
  newline_col TEXT,
  jsonish_col TEXT,
  padded_col  TEXT
);
INSERT INTO t VALUES ('', NULL, 42, -1.25, '東京 🗾', 'a,b', 'a' || char(9) || 'b',
                      'line1' || char(10) || 'line2', '{"k":1}', '00123');
`

// TestSmoke_OutputFormatsAgreeOnAwkwardValues is the CLI-level detector for the
// record/cell divergence the Table ownership rules exist to prevent.
//
// The three formats reach the values by different routes: CSV and table read
// the display strings, JSON reads the native cells. If those two representations
// ever drift apart again, one of these columns disagrees. The fixture is
// deliberately awkward — a value that needs quoting in CSV, one that cannot be
// spelled in a single table line, one that is itself JSON — because a single
// well-behaved string column would agree no matter what broke.
func TestSmoke_OutputFormatsAgreeOnAwkwardValues(t *testing.T) {
	query := "SELECT * FROM t;\n"

	csvOut := mustRunScript(t, "csv", agreementFixture+query, "csv")
	tsvOut := mustRunScript(t, "tsv", agreementFixture+query, "tsv")
	jsonOut := mustRunScript(t, "json", agreementFixture+query, "json")
	tableOut := mustRunScript(t, "table", agreementFixture+query, "table")

	csvRecords := parseCSVRecords(t, "csv", csvOut, ',')
	tsvRecords := parseCSVRecords(t, "tsv", tsvOut, '\t')
	jsonRows := parseJSONRows(t, "json", jsonOut)

	if len(csvRecords) != 2 || len(tsvRecords) != 2 || len(jsonRows) != 1 {
		t.Fatalf("row counts: csv=%d tsv=%d json=%d, want 2/2/1",
			len(csvRecords), len(tsvRecords), len(jsonRows))
	}
	columns := csvRecords[0]
	csvRow, tsvRow, jsonRow := csvRecords[1], tsvRecords[1], jsonRows[0]

	// The display string every text format must agree on, per column.
	wantDisplay := map[string]string{
		"empty_col":   "",
		"null_col":    "", // a NULL has no string spelling; it renders blank
		"int_col":     "42",
		"real_col":    "-1.25",
		"unicode_col": "東京 🗾",
		"comma_col":   "a,b",
		"tab_col":     "a\tb",
		"newline_col": "line1\nline2",
		"jsonish_col": `{"k":1}`,
		"padded_col":  "00123",
	}

	for i, column := range columns {
		want, ok := wantDisplay[column]
		if !ok {
			t.Fatalf("unexpected column %q", column)
		}
		if csvRow[i] != want {
			t.Errorf("%s: csv = %q, want %q", column, csvRow[i], want)
		}
		if tsvRow[i] != want {
			t.Errorf("%s: tsv = %q, want %q", column, tsvRow[i], want)
		}
	}

	// JSON reaches the same values through the native cells, so its types carry
	// information the text formats cannot: a NULL is null, a number is a number,
	// and everything else is the same string CSV printed.
	if jsonRow["null_col"] != nil {
		t.Errorf("null_col = %#v, want JSON null", jsonRow["null_col"])
	}
	if jsonRow["empty_col"] != "" {
		t.Errorf("empty_col = %#v, want the empty string, distinct from null", jsonRow["empty_col"])
	}
	if n, ok := jsonRow["int_col"].(float64); !ok || n != 42 {
		t.Errorf("int_col = %#v (%T), want the number 42", jsonRow["int_col"], jsonRow["int_col"])
	}
	if n, ok := jsonRow["real_col"].(float64); !ok || n != -1.25 {
		t.Errorf("real_col = %#v (%T), want the number -1.25", jsonRow["real_col"], jsonRow["real_col"])
	}
	for _, column := range []string{"unicode_col", "comma_col", "tab_col", "newline_col", "jsonish_col", "padded_col"} {
		if got, ok := jsonRow[column].(string); !ok || got != wantDisplay[column] {
			t.Errorf("%s: json = %#v, want the string %q", column, jsonRow[column], wantDisplay[column])
		}
	}

	// The table renderer is checked by value too, not by substring: a value
	// landing in the wrong column would pass a `strings.Contains` check. The
	// newline column is excluded because the renderer wraps it onto a second
	// physical line by design.
	tableRows := parseTableCells(t, "table", tableOut)
	joined := strings.Join(tableRows[0], "\x00")
	for _, column := range []string{"int_col", "real_col", "unicode_col", "comma_col", "jsonish_col", "padded_col"} {
		if !strings.Contains("\x00"+joined+"\x00", "\x00"+wantDisplay[column]+"\x00") {
			t.Errorf("%s: table output has no cell equal to %q; cells = %q",
				column, wantDisplay[column], tableRows[0])
		}
	}
}

// mustRunScript runs a batch script in one output format and returns stdout.
func mustRunScript(t *testing.T, label, script, format string) string {
	t.Helper()

	stdout, stderr, code := run(t, script, "--output-format", format)
	if code != 0 {
		t.Fatalf("%s: exit code = %d\nstderr: %s", label, code, stderr)
	}
	assertNoResidueDiagnostics(t, label, stderr)
	return stdout
}

// fileExists reports whether path exists, without caring why it does not.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TestSmoke_DocumentedOutputFormatExamples runs the example from the formats
// page and compares it with what is published there. The page shows one query
// in three formats and claims they agree, which is a claim that rots silently:
// nothing else notices when the renderer changes and the documentation does not.
//
// The fixture and the expected output are the ones in
// website/content/formats.md ("Output formats").
func TestSmoke_DocumentedOutputFormatExamples(t *testing.T) {
	dir := t.TempDir()
	input := writeFixture(t, dir, "t.csv", "code,qty,note\n007,42,\n")

	t.Run("table", func(t *testing.T) {
		got := runQueryAs(t, "documented table example", "table", "SELECT * FROM t", input)
		want := "+------+-----+------+\n" +
			"| code | qty | note |\n" +
			"+------+-----+------+\n" +
			"|  007 |  42 |      |\n" +
			"+------+-----+------+\n"
		if dropStatusLines(got) != want {
			t.Errorf("table output does not match the formats page:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("csv", func(t *testing.T) {
		got := runQueryAs(t, "documented csv example", "csv", "SELECT * FROM t", input)
		want := "code,qty,note\n007,42,\n"
		if dropStatusLines(got) != want {
			t.Errorf("csv output does not match the formats page:\ngot:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("json", func(t *testing.T) {
		got := runQueryAs(t, "documented json example", "json", "SELECT * FROM t", input)
		rows := parseJSONRows(t, "documented json example", got)
		if len(rows) != 1 {
			t.Fatalf("decoded %d rows, want 1", len(rows))
		}
		// The page's claims, one assertion each.
		if rows[0]["code"] != "007" {
			t.Errorf(`code = %#v, want the string "007" with its leading zeros`, rows[0]["code"])
		}
		if n, ok := rows[0]["qty"].(float64); !ok || n != 42 {
			t.Errorf("qty = %#v (%T), want the number 42", rows[0]["qty"], rows[0]["qty"])
		}
		// An empty CSV field is an empty string, not a NULL; the page says so.
		if rows[0]["note"] != "" {
			t.Errorf(`note = %#v, want the empty string ""`, rows[0]["note"])
		}
	})
}

// TestSmoke_ParquetExportKeepsValuesSQLCannotParse covers a value that is data
// to the engine but syntax to the SQL parser. The Parquet export stages the
// result in a temporary SQLite database, and it used to build that INSERT as SQL
// text with every value quoted into it. A NUL byte ends a statement as far as
// SQLite's tokenizer is concerned, so a CSV carrying one — which every other
// format exports without complaint — failed the export with an "unrecognized
// token" error naming a token the user never typed.
func TestSmoke_ParquetExportKeepsValuesSQLCannotParse(t *testing.T) {
	dir := t.TempDir()
	// A NUL between two letters, plus an apostrophe and a statement separator:
	// three things a quoted-into-SQL value gets wrong.
	source := writeFixture(t, dir, "odd.csv", "id,payload\n1,A\x00B\n2,it's; DROP TABLE t; --\n")
	out := filepath.Join(dir, "odd.parquet")

	_, stderr, code := run(t, "", "--output-format", "parquet", "--output", out, "--sql", "SELECT * FROM odd", source)
	if code != 0 {
		t.Fatalf("parquet export exit code = %d, want 0\nstderr: %s", code, stderr)
	}

	// Reading it back through sqly is the check that matters: the bytes are
	// compared as hex so a NUL is visible rather than swallowed by the terminal.
	got := runQueryAs(t, "reimport", "csv", "SELECT hex(payload) AS h FROM odd ORDER BY id", out)
	records := parseCSVRecords(t, "reimport", got, ',')
	if len(records) != 3 {
		t.Fatalf("reimported %d records (with header), want 3:\n%s", len(records), got)
	}
	if want := strings.ToUpper(hexOf("A\x00B")); records[1][0] != want {
		t.Errorf("row 1 payload = %s, want %s (the NUL byte must survive)", records[1][0], want)
	}
	if want := strings.ToUpper(hexOf("it's; DROP TABLE t; --")); records[2][0] != want {
		t.Errorf("row 2 payload = %s, want %s", records[2][0], want)
	}
}

// hexOf renders s the way SQLite's hex() does, so a test can state the bytes it
// expects without embedding an unreadable literal.
func hexOf(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		b.WriteString(strconv.FormatInt(int64(c), 16))
		if c < 16 {
			// FormatInt drops the leading zero hex() keeps.
			last := b.String()
			b.Reset()
			b.WriteString(last[:len(last)-1] + "0" + last[len(last)-1:])
		}
	}
	return b.String()
}

// TestSmoke_ExcelExportRefusesValuesXLSXCannotCarry covers the one format that
// used to answer an unrepresentable value by changing it. XLSX is XML, and XML
// 1.0 has no way to write most control characters, so the writer substituted
// U+FFFD: the export succeeded, the file appeared, and the value was gone. The
// documented contract for a format that cannot represent a value is to refuse
// and leave the destination alone, which is what LTSV already did.
func TestSmoke_ExcelExportRefusesValuesXLSXCannotCarry(t *testing.T) {
	dir := t.TempDir()
	source := writeFixture(t, dir, "ctl.csv", "id,v\n1,A\x01B\n")
	out := filepath.Join(dir, "out.xlsx")
	if err := os.WriteFile(out, []byte("PRECIOUS"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "--output-format", "excel", "--output", out, "--sql", "SELECT * FROM ctl", source)
	if code != 4 {
		t.Errorf("exit code = %d, want 4 (a value the format cannot represent)\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "control character") || !strings.Contains(stderr, "U+0001") {
		t.Errorf("stderr = %q, want it to name the character it cannot write", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}

	// The destination is untouched, as it is for every other refused export.
	got, err := os.ReadFile(out) //nolint:gosec // path built by the test
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "PRECIOUS" {
		t.Errorf("destination = %q, want it left exactly as it was", got)
	}

	// A value XLSX can carry still exports, so the check refuses only what it
	// must: tab, newline, and carriage return are the control characters XML keeps.
	keepable := writeFixture(t, dir, "keep.csv", "id,v\n1,\"a\tb\"\n")
	kept := filepath.Join(dir, "keep.xlsx")
	if _, stderr, code := run(t, "", "--output-format", "excel", "--output", kept, "--sql", "SELECT * FROM keep", keepable); code != 0 {
		t.Errorf("exporting a tab exit code = %d, want 0\nstderr: %s", code, stderr)
	}
}
