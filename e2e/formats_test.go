//go:build smoke

package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
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

// exportExtension is the destination extension each writable format requires,
// so the round-trip test below can name a file --output will accept.
var exportExtension = map[string]string{
	"csv":     ".csv",
	"tsv":     ".tsv",
	"ltsv":    ".ltsv",
	"json":    ".json",
	"jsonl":   ".jsonl",
	"excel":   ".xlsx",
	"parquet": ".parquet",
}

// TestSmoke_ExportedHeadersReimport is the round-trip property for a header: an
// export that reports success has written a file sqly can read back.
//
// csv, tsv, and excel used to break it. An import reads two column names that
// differ only in case, or only in surrounding whitespace, as one column and
// refuses the file; those three writers compared nothing, so `SELECT id AS x,
// label AS x` exported at exit 0 and the file it produced failed to load at exit
// 3. json, jsonl, ltsv, and parquet already refused a repeat, which is what made
// the gap a difference between formats rather than one rule.
//
// The property is stated over every writable format rather than over the three
// that were wrong, and it does not say which side of the line a header falls on:
// each format is asked, and whichever answer it gives has to hold up — a success
// must re-import, and a refusal must leave no file behind. A format free to
// disagree about a header is not free to write one it cannot read.
func TestSmoke_ExportedHeadersReimport(t *testing.T) {
	headers := []struct {
		name    string
		columns []string
	}{
		{name: "a name repeated", columns: []string{"x", "x"}},
		{name: "names differing only by case", columns: []string{"a", "A"}},
		{name: "names differing only by surrounding whitespace", columns: []string{"x", " x"}},
		{name: "names differing by both, which an import tells apart", columns: []string{"a", " A"}},
		{name: "names differing by non-ASCII case, which SQLite tells apart", columns: []string{"ä", "Ä"}},
		{name: "distinct names", columns: []string{"a", "b"}},
	}

	for _, format := range []string{"csv", "tsv", "ltsv", "json", "jsonl", "excel", "parquet"} {
		t.Run(format, func(t *testing.T) {
			for _, header := range headers {
				t.Run(header.name, func(t *testing.T) {
					dir := t.TempDir()
					out := filepath.Join(dir, "out"+exportExtension[format])
					source := writeFixture(t, dir, "src.csv", "id\n1\n")

					selectList := make([]string, 0, len(header.columns))
					for i, column := range header.columns {
						selectList = append(selectList,
							strconv.Itoa(i+1)+` AS "`+column+`"`)
					}
					query := "SELECT " + strings.Join(selectList, ", ") + " FROM src"

					_, stderr, code := run(t, "", "--output-format", format, "--output", out, "--sql", query, source)
					if code != 0 {
						if code != 4 {
							t.Fatalf("export exit code = %d, want 0 or 4 (a header the format refuses)\nstderr: %s", code, stderr)
						}
						// A refused export leaves the destination alone, as every
						// other refusal in this file does.
						if _, err := os.Stat(out); !os.IsNotExist(err) {
							t.Errorf("refused export left %s behind, want no file", out)
						}
						return
					}

					// The query names no table, so it proves the file loaded rather
					// than anything about what the load produced.
					if _, stderr, code := run(t, "", "--sql", "SELECT 1", out); code != 0 {
						t.Errorf("export succeeded but the file it wrote does not load: exit %d\nstderr: %s", code, stderr)
					}
				})
			}
		})
	}
}

// TestSmoke_EncodingRefusesBytesItCannotDecode is the encoding half of the rule
// that a text input sqly cannot read is refused rather than loaded as mojibake.
// Without --encoding that has been true since rc8; with it, the x/text decoders
// substituted U+FFFD for bytes the declared encoding has no meaning for, so the
// same corruption loaded at exit 0. Naming an encoding must not turn the check
// off — it changes which bytes are valid, not whether they are checked.
func TestSmoke_EncodingRefusesBytesItCannotDecode(t *testing.T) {
	for _, tt := range []struct {
		name     string
		encoding string
		content  []byte
	}{
		{name: "a byte that begins nothing in shift-jis", encoding: "shift-jis", content: []byte("a\n\xff\xfe\x01\n")},
		{name: "a byte that begins nothing in euc-jp", encoding: "euc-jp", content: []byte("a\n\xff\xff\n")},
		// ISO-2022-JP is a 7-bit encoding, so a byte with the high bit set is
		// outside it whatever shift state the stream is in.
		{name: "a high bit set in iso-2022-jp", encoding: "iso-2022-jp", content: []byte("a\n\xff\xfe\n")},
		{name: "a utf-16le code unit cut in half", encoding: "utf-16le", content: []byte("a\x00\n\x00\x41")},
		{name: "a utf-16le surrogate with no partner", encoding: "utf-16le", content: []byte("a\x00\n\x00\x00\xd8\x41\x00")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "bad.csv")
			if err := os.WriteFile(path, tt.content, 0o600); err != nil {
				t.Fatal(err)
			}

			stdout, stderr, code := run(t, "", "--encoding", tt.encoding, "--sql", "SELECT 1", path)
			if code != 3 {
				t.Errorf("exit code = %d, want 3 (an input that could not be read)\nstderr: %s", code, stderr)
			}
			if !strings.Contains(stderr, tt.encoding) {
				t.Errorf("stderr = %q, want it to name the declared encoding", stderr)
			}
			// Import is all-or-nothing, so the refusal lands before any table.
			if !strings.Contains(stderr, "no table was created or changed") {
				t.Errorf("stderr = %q, want the all-or-nothing framing", stderr)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("stdout = %q, want it empty", stdout)
			}
		})
	}

	t.Run("the stdin dataset is checked the same way", func(t *testing.T) {
		_, stderr, code := run(t, "a\n\xff\xfe\x01\n",
			"--stdin-format", "csv", "--encoding", "shift-jis", "--sql", "SELECT 1")
		if code != 3 {
			t.Errorf("exit code = %d, want 3\nstderr: %s", code, stderr)
		}
		if !strings.Contains(stderr, "shift-jis") {
			t.Errorf("stderr = %q, want it to name the declared encoding", stderr)
		}
	})
}

// TestSmoke_EncodingStillReadsWhatItShould is the other side: the check rejects
// malformed input rather than the encodings themselves, so every valid decode
// still loads.
func TestSmoke_EncodingStillReadsWhatItShould(t *testing.T) {
	const utf8Source = "name,city\n山田,東京\n"

	for _, tt := range []struct {
		name     string
		encoding string
		encoder  transform.Transformer
	}{
		{name: "shift-jis", encoding: "shift-jis", encoder: japanese.ShiftJIS.NewEncoder()},
		{name: "euc-jp", encoding: "euc-jp", encoder: japanese.EUCJP.NewEncoder()},
		{name: "iso-2022-jp", encoding: "iso-2022-jp", encoder: japanese.ISO2022JP.NewEncoder()},
		{name: "utf-16le", encoding: "utf-16le", encoder: unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()},
		{name: "utf-16be", encoding: "utf-16be", encoder: unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "people.csv")
			encoded, _, err := transform.Bytes(tt.encoder, []byte(utf8Source))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, encoded, 0o600); err != nil {
				t.Fatal(err)
			}

			stdout, stderr, code := run(t, "",
				"--encoding", tt.encoding, "--output-format", "jsonl", "--sql", "SELECT city FROM people", path)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
			}
			if !strings.Contains(stdout, "東京") {
				t.Errorf("output = %q, want the decoded value", stdout)
			}
		})
	}

	t.Run("a genuine replacement character is data, not a failed decode", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "fffd.csv")
		// "a\n" then U+FFFD, as little-endian UTF-16 code units.
		if err := os.WriteFile(path, []byte("a\x00\n\x00\xfd\xff"), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := run(t, "",
			"--encoding", "utf-16le", "--output-format", "jsonl", "--sql", "SELECT a FROM fffd", path)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0: U+FFFD is representable in UTF-16\nstderr: %s", code, stderr)
		}
		if !strings.Contains(stdout, "�") {
			t.Errorf("output = %q, want the replacement character the file holds", stdout)
		}
	})
}

// TestSmoke_OutputExtensionRules pins what --output does with each kind of
// destination extension: a known one must agree with the chosen format, an
// unknown one is written as given, and a missing one gets the format's own.
//
// Only the first was documented. The formats page said the extension "must agree
// with the chosen format" without qualification, so the two escape hatches were
// behavior nothing described and nothing held in place.
func TestSmoke_OutputExtensionRules(t *testing.T) {
	dir := t.TempDir()
	source := writeFixture(t, dir, "src.csv", "id\n1\n")

	t.Run("a known extension must agree with the format", func(t *testing.T) {
		out := filepath.Join(dir, "disagree.json")
		_, stderr, code := run(t, "", "--output-format", "csv", "--output", out, "--sql", "SELECT 1", source)
		if code != 2 {
			t.Errorf("exit code = %d, want 2\nstderr: %s", code, stderr)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Errorf("a refused export created %s", out)
		}
	})

	t.Run("an unknown extension is written as given", func(t *testing.T) {
		out := filepath.Join(dir, "report.txt")
		if _, stderr, code := run(t, "", "--output-format", "csv", "--output", out, "--sql", "SELECT 1 AS n", source); code != 0 {
			t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
		}
		if _, err := os.Stat(out); err != nil {
			t.Errorf("want %s written at the path given: %v", out, err)
		}
	})

	t.Run("no extension gets the format's own", func(t *testing.T) {
		out := filepath.Join(dir, "report")
		if _, stderr, code := run(t, "", "--output-format", "tsv", "--output", out, "--sql", "SELECT 1 AS n", source); code != 0 {
			t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
		}
		if _, err := os.Stat(out + ".tsv"); err != nil {
			t.Errorf("want %s.tsv written: %v", out, err)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Errorf("the extensionless path %s should not also be written", out)
		}
	})
}

// TestSmoke_SavePreservesTheSourceEncoding is the write-back half of --encoding.
// A save used to write UTF-8 whatever the source was, so `.save --in-place` on a
// Shift-JIS file quietly converted it, and the user's own next run of the same
// command decoded UTF-8 as Shift-JIS and returned mojibake at exit 0. Compression
// has always been preserved by a write-back; the encoding is the same promise.
func TestSmoke_SavePreservesTheSourceEncoding(t *testing.T) {
	for _, tt := range []struct {
		name     string
		encoding string
		encoder  transform.Transformer
		decoder  transform.Transformer
	}{
		{
			name: "shift-jis", encoding: "shift-jis",
			encoder: japanese.ShiftJIS.NewEncoder(), decoder: japanese.ShiftJIS.NewDecoder(),
		},
		{
			name: "euc-jp", encoding: "euc-jp",
			encoder: japanese.EUCJP.NewEncoder(), decoder: japanese.EUCJP.NewDecoder(),
		},
		{
			// The UTF-16 writers emit a byte-order mark, so this case also pins
			// that what the save produces is what the read side recognizes.
			name: "utf-16le", encoding: "utf-16le",
			encoder: unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder(),
			decoder: unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder(),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "people.csv")
			encoded, _, err := transform.Bytes(tt.encoder, []byte("name,city\n山田,東京\n"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, encoded, 0o600); err != nil {
				t.Fatal(err)
			}

			script := "UPDATE people SET city = '京都' WHERE name = '山田';\n.save --in-place\n"
			if _, stderr, code := run(t, script, "--encoding", tt.encoding, path); code != 0 {
				t.Fatalf("save exit code = %d, want 0\nstderr: %s", code, stderr)
			}

			// The bytes on disk still decode in the source's own encoding.
			written, err := os.ReadFile(path) //nolint:gosec // path built by the test
			if err != nil {
				t.Fatal(err)
			}
			decoded, _, err := transform.Bytes(tt.decoder, written)
			if err != nil {
				t.Fatalf("the saved file does not decode as %s: %v", tt.encoding, err)
			}
			if !strings.Contains(string(decoded), "京都") {
				t.Errorf("decoded file = %q, want the updated value", decoded)
			}

			// The user's own pipeline, run again unchanged, is what the bug broke.
			stdout, stderr, code := run(t, "",
				"--encoding", tt.encoding, "--output-format", "jsonl", "--sql", "SELECT city FROM people", path)
			if code != 0 {
				t.Fatalf("re-read exit code = %d, want 0\nstderr: %s", code, stderr)
			}
			if !strings.Contains(stdout, "京都") {
				t.Errorf("re-read = %q, want 京都 rather than mojibake", stdout)
			}
		})
	}

	t.Run("a value the encoding cannot write refuses the save", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "people.csv")
		encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte("name\n山田\n"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path) //nolint:gosec // path built by the test
		if err != nil {
			t.Fatal(err)
		}

		// An emoji has no Shift-JIS spelling, so writing it would substitute.
		script := "UPDATE people SET name = '🎌';\n.save --in-place\n"
		_, stderr, code := run(t, script, "--encoding", "shift-jis", path)
		if code != 4 {
			t.Errorf("exit code = %d, want 4 (a value the destination cannot represent)\nstderr: %s", code, stderr)
		}
		if !strings.Contains(stderr, "shift-jis") {
			t.Errorf("stderr = %q, want it to name the encoding", stderr)
		}

		after, err := os.ReadFile(path) //nolint:gosec // path built by the test
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Errorf("a refused save changed the file: %q, want it left as it was", after)
		}
	})
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
	if !strings.Contains(stderr, "XLSX cannot represent") || !strings.Contains(stderr, "U+0001") {
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
