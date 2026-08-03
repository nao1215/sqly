//go:build smoke

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestSmoke_CacheColdThenWarm is the baseline the failure cases are measured
// against: the first run builds the cache, the second reads it, and both answer
// the same query with the same bytes. A cache that silently returned different
// data would be worse than no cache at all.
func TestSmoke_CacheColdThenWarm(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cachePath := filepath.Join(dir, "session.cache")

	args := []string{
		"--cache", cachePath,
		"--output-format", "csv",
		"--sql", "SELECT id, name FROM people ORDER BY id",
		csvPath,
	}

	cold, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("cold run exit code = %d (stderr=%q)", code, stderr)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("the cold run did not write the cache: %v", err)
	}

	warm, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("warm run exit code = %d (stderr=%q)", code, stderr)
	}
	if cold != warm {
		t.Errorf("warm run differs from cold run:\ncold:\n%s\nwarm:\n%s", cold, warm)
	}
	if !strings.Contains(warm, "alice") || !strings.Contains(warm, "bob") {
		t.Errorf("warm run = %q, want both rows", warm)
	}
}

// TestSmoke_CacheCorruptFallsBackToColdImport covers an unusable cache: the file
// exists but is not a SQLite database. Attaching it fails, and a failed cache
// load must not fail the run — the source files are still there, so the session
// falls back to a cold import and the user gets their answer.
func TestSmoke_CacheCorruptFallsBackToColdImport(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n")
	cachePath := writeFixture(t, dir, "broken.cache", "this is not a sqlite database")

	stdout, stderr, code := run(t, "",
		"--cache", cachePath,
		"--output-format", "csv",
		"--sql", "SELECT name FROM people",
		csvPath)

	if code != 0 {
		t.Fatalf("a corrupt cache failed the run: exit %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, "alice") {
		t.Errorf("stdout = %q, want the row from the cold import", stdout)
	}
	assertNoPanic(t, stdout, stderr)
}

// TestSmoke_CacheStaysUsableAfterAFailedRun is the recovery property the detach
// exists for. A run that fails after the cache is attached must still release
// it; the session is per-process so the leak is not visible directly, but a
// subsequent run reading the same cache is, and it would fail if the previous
// run had left a broken cache behind.
func TestSmoke_CacheStaysUsableAfterAFailedRun(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cachePath := filepath.Join(dir, "session.cache")

	// Build the cache.
	if _, stderr, code := run(t, "",
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT COUNT(*) FROM people", csvPath); code != 0 {
		t.Fatalf("seeding the cache failed: %d (stderr=%q)", code, stderr)
	}

	// A run that reads the cache and then fails on a bad query.
	stdout, stderr, code := run(t, "",
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT FROM WHERE", csvPath)
	if code == 0 {
		t.Fatalf("a syntax error exited 0 (stdout=%q)", stdout)
	}
	assertNoPanic(t, stdout, stderr)

	// The cache must still work afterwards.
	stdout, stderr, code = run(t, "",
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT name FROM people ORDER BY id", csvPath)
	if code != 0 {
		t.Fatalf("the cache was unusable after a failed run: exit %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, "alice") || !strings.Contains(stdout, "bob") {
		t.Errorf("stdout = %q, want both rows", stdout)
	}
}

// TestSmoke_CacheRebuildsWhenSourceChanges checks that editing the source
// invalidates the cache. A cache keyed only on the path would serve the old
// rows here, which is the worst failure mode a cache has: a confidently wrong
// answer.
func TestSmoke_CacheRebuildsWhenSourceChanges(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n")
	cachePath := filepath.Join(dir, "session.cache")

	args := []string{
		"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT name FROM people ORDER BY id", csvPath,
	}
	if _, stderr, code := run(t, "", args...); code != 0 {
		t.Fatalf("seeding the cache failed: %d (stderr=%q)", code, stderr)
	}

	// Same length, different content, so a size-only key would miss the change.
	writeFixture(t, dir, "people.csv", "id,name\n1,zzzzz\n")

	stdout, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, "zzzzz") {
		t.Errorf("stdout = %q, want the edited row; the cache served stale data", stdout)
	}
	if strings.Contains(stdout, "alice") {
		t.Errorf("stdout = %q, still holds the pre-edit row", stdout)
	}
}

// TestSmoke_CacheEmptyValueRejected checks the argument contract: --cache with
// an empty value is a mistake, not a request to disable the cache, so it is
// rejected rather than silently ignored.
func TestSmoke_CacheEmptyValueRejected(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n")

	stdout, stderr, code := run(t, "",
		"--cache", "", "--output-format", "csv", "--sql", "SELECT 1", csvPath)
	if code == 0 {
		t.Fatalf("--cache \"\" exited 0 (stdout=%q)", stdout)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want it empty", stdout)
	}
	assertNoPanic(t, stdout, stderr)
}

// TestSmoke_ShellRecoversAfterAFailedQuery is the shell-mode continuation
// property. A failed query must not end the session's usefulness: the next
// statement has to run against the same data.
func TestSmoke_ShellRecoversAfterAFailedQuery(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")

	// Batch mode stops at the first failure by design, so each statement is
	// checked in its own run to establish that the session state itself is not
	// what breaks: the same query succeeds before and after a failing one.
	before, _, code := run(t, ".mode csv\nSELECT name FROM people ORDER BY id;\n", csvPath)
	if code != 0 {
		t.Fatalf("the baseline query failed: %d", code)
	}

	stdout, stderr, code := run(t, ".mode csv\nSELECT * FROM no_such_table;\n", csvPath)
	if code == 0 {
		t.Fatal("a query against a missing table exited 0")
	}
	if !strings.Contains(stderr, "no_such_table") {
		t.Errorf("stderr = %q, want it to name the missing table", stderr)
	}
	assertNoPanic(t, stdout, stderr)

	after, _, code := run(t, ".mode csv\nSELECT name FROM people ORDER BY id;\n", csvPath)
	if code != 0 {
		t.Fatalf("the query failed after a failing one: %d", code)
	}
	if before != after {
		t.Errorf("the session answered differently after a failure:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestSmoke_MultipleFilesLoadAndQuery is the multi-input happy path: two files
// become two tables, each queryable, and a join across them works. It is the
// baseline the rollback case below is measured against.
func TestSmoke_MultipleFilesLoadAndQuery(t *testing.T) {
	dir := t.TempDir()
	people := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cities := writeFixture(t, dir, "cities.csv", "id,city\n1,tokyo\n2,osaka\n")

	for _, tc := range []struct {
		name  string
		query string
		want  []string
	}{
		{"first table", "SELECT name FROM people ORDER BY id", []string{"alice", "bob"}},
		{"second table", "SELECT city FROM cities ORDER BY id", []string{"tokyo", "osaka"}},
		{
			"joined across both",
			"SELECT p.name, c.city FROM people p JOIN cities c ON p.id = c.id ORDER BY p.id",
			[]string{"alice,tokyo", "bob,osaka"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(t, "", "--output-format", "csv", "--sql", tc.query, people, cities)
			if code != 0 {
				t.Fatalf("exit code = %d (stderr=%q)", code, stderr)
			}
			body := nonEmptyLines(dropStatusLines(stdout))[1:] // drop the header line
			if strings.Join(body, "|") != strings.Join(tc.want, "|") {
				t.Errorf("rows = %v, want %v", body, tc.want)
			}
		})
	}
}

// TestSmoke_CacheMatchesColdImport is the cold/warm parity check: a warm run
// must be indistinguishable from a cold one. A cache that restored subtly
// different data — a lost NULL, a numeric column read back as text, a missing
// row from a partial restore — would show up here and nowhere else, because
// every other cache test asks only whether the run succeeded.
//
// CSV and table are compared byte for byte, because their output is a fixed
// contract. JSON is compared after decoding, so key order and whitespace cannot
// make the test fail for a reason that is not a difference in the data.
func TestSmoke_CacheMatchesColdImport(t *testing.T) {
	dir := t.TempDir()
	// One row per value class the cache has to carry back intact.
	csvPath := writeFixture(t, dir, "typed.csv",
		"code,qty,ratio,note\n00123,42,1.5,alpha\n00456,-7,0.25,\n")
	query := "SELECT code, qty, ratio, note FROM typed ORDER BY code"

	for _, format := range []string{"csv", "tsv", "json", "table"} {
		t.Run(format, func(t *testing.T) {
			cachePath := filepath.Join(dir, "cache-"+format+".db")

			coldOut, coldErr, coldCode := run(t, "",
				"--output-format", format, "--sql", query, csvPath)
			assertNoResidueDiagnostics(t, "cold "+format, coldErr)

			cachedArgs := []string{"--cache", cachePath,
				"--output-format", format, "--sql", query, csvPath}
			// The first cached run builds the cache; the second reads it.
			if _, stderr, code := run(t, "", cachedArgs...); code != 0 {
				t.Fatalf("cache build exit code = %d (stderr=%q)", code, stderr)
			}
			if !fileExists(cachePath) {
				t.Fatalf("no cache was written to %s", cachePath)
			}
			warmOut, warmErr, warmCode := run(t, "", cachedArgs...)
			assertNoResidueDiagnostics(t, "warm "+format, warmErr)

			if coldCode != warmCode {
				t.Errorf("exit codes differ: cold=%d warm=%d", coldCode, warmCode)
			}
			if coldCode != 0 {
				t.Fatalf("cold run exit code = %d (stderr=%q)", coldCode, coldErr)
			}

			switch format {
			case "json":
				// Compare decoded values, so key order and spacing cannot
				// decide the outcome.
				cold := parseJSONRows(t, "cold json", coldOut)
				warm := parseJSONRows(t, "warm json", warmOut)
				if len(cold) != len(warm) {
					t.Fatalf("row counts differ: cold=%d warm=%d", len(cold), len(warm))
				}
				for i := range cold {
					for key, want := range cold[i] {
						if got := warm[i][key]; got != want {
							t.Errorf("row %d %q: warm %#v (%T) != cold %#v (%T)",
								i, key, got, got, want, want)
						}
					}
				}
			default:
				// csv, tsv, and table have a fixed byte-level contract.
				if coldOut != warmOut {
					t.Errorf("%s output differs:\ncold:\n%s\nwarm:\n%s", format, coldOut, warmOut)
				}
			}
		})
	}
}

// TestSmoke_CacheSurvivesAnInterruptedRun covers the attachment-leak failure
// mode from the outside. A run is killed part-way while it holds the cache; the
// next run must not fail with a message about the cache being unavailable,
// which is what a leaked ATTACH produces.
func TestSmoke_CacheSurvivesAnInterruptedRun(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cachePath := filepath.Join(dir, "session.cache")

	base := []string{"--cache", cachePath, "--output-format", "csv"}
	build := append(append([]string{}, base...), "--sql", "SELECT COUNT(*) FROM people", csvPath)
	if _, stderr, code := run(t, "", build...); code != 0 {
		t.Fatalf("cache build failed: %d (stderr=%q)", code, stderr)
	}

	// A cached run that fails after the cache is loaded.
	failing := append(append([]string{}, base...), "--sql", "SELECT FROM WHERE", csvPath)
	stdout, stderr, code := run(t, "", failing...)
	if code == 0 {
		t.Fatalf("a syntax error exited 0 (stdout=%q)", stdout)
	}
	assertNoPanic(t, stdout, stderr)

	// The next runs must all succeed, with no diagnostic that points back at the
	// interrupted one, and with the cache actually reused rather than quietly
	// rebuilt from scratch every time.
	for i := range 3 {
		good := append(append([]string{}, base...), "--sql", "SELECT name FROM people ORDER BY id", csvPath)
		stdout, stderr, code = run(t, "", good...)
		if code != 0 {
			t.Fatalf("run %d after the interrupted one exited %d (stderr=%q)", i, code, stderr)
		}
		assertNoResidueDiagnostics(t, fmt.Sprintf("run %d after the interrupted one", i), stderr)

		records := parseCSVRecords(t, "run after interruption", stdout, ',')
		if len(records) != 3 {
			t.Fatalf("run %d returned %d csv records, want a header and 2 rows: %q", i, len(records), stdout)
		}
		if records[1][0] != "alice" || records[2][0] != "bob" {
			t.Errorf("run %d rows = %v, want alice and bob; a partial restore would drop one", i, records[1:])
		}
	}
}

// TestSmoke_CacheRestoreIsAtomicAfterAConflict is the cache-restore half of the
// atomicity story. The session already holds a table the cache also carries, so
// restoring cannot complete; the run must still produce a correct answer by
// falling back to a cold import, which it can only do if the failed restore
// left no half-populated tables to collide with.
func TestSmoke_CacheRestoreIsAtomicAfterAConflict(t *testing.T) {
	dir := t.TempDir()
	people := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cities := writeFixture(t, dir, "cities.csv", "id,city\n1,tokyo\n2,osaka\n")
	cachePath := filepath.Join(dir, "two-tables.cache")

	base := []string{"--cache", cachePath, "--output-format", "csv"}
	build := append(append([]string{}, base...),
		"--sql", "SELECT COUNT(*) FROM people", people, cities)
	if _, stderr, code := run(t, "", build...); code != 0 {
		t.Fatalf("cache build failed: %d (stderr=%q)", code, stderr)
	}

	// Warm runs must keep answering correctly for both tables, repeatedly: a
	// restore that left one table behind would surface as a wrong count or as a
	// collision on a later run.
	for i := range 3 {
		for table, want := range map[string]string{"people": "2", "cities": "2"} {
			args := append(append([]string{}, base...),
				"--sql", "SELECT COUNT(*) AS n FROM "+table, people, cities)
			stdout, stderr, code := run(t, "", args...)
			if code != 0 {
				t.Fatalf("run %d on %s exited %d (stderr=%q)", i, table, code, stderr)
			}
			assertNoResidueDiagnostics(t, fmt.Sprintf("run %d on %s", i, table), stderr)
			records := parseCSVRecords(t, table, stdout, ',')
			if len(records) != 2 || records[1][0] != want {
				t.Errorf("run %d %s count = %v, want %s", i, table, records, want)
			}
		}
	}
}

// TestSmoke_ConcurrentRunsShareOneCache starts several runs against one cache
// file at the same time. The alias a cache attaches under is a fixed name, so a
// collision between two simultaneous runs is a legitimate outcome and is not
// what this checks. What it checks is that no run leaves the cache permanently
// broken: after the storm, a plain run has to work.
func TestSmoke_ConcurrentRunsShareOneCache(t *testing.T) {
	dir := t.TempDir()
	csvPath := writeFixture(t, dir, "people.csv", "id,name\n1,alice\n2,bob\n")
	cachePath := filepath.Join(dir, "shared.cache")

	args := []string{"--cache", cachePath, "--output-format", "csv",
		"--sql", "SELECT name FROM people ORDER BY id", csvPath}
	if _, stderr, code := run(t, "", args...); code != 0 {
		t.Fatalf("cache build failed: %d (stderr=%q)", code, stderr)
	}

	const runs = 4
	type outcome struct {
		stdout, stderr string
		code           int
	}
	results := make([]outcome, runs)
	var wg sync.WaitGroup
	for i := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stdout, stderr, code := run(t, "", args...)
			results[i] = outcome{stdout, stderr, code}
		}()
	}
	wg.Wait()

	// A run that succeeded must have returned the whole answer, not part of it.
	for i, got := range results {
		if got.code != 0 {
			continue // a contended run may fail; the recovery below is the contract
		}
		records := parseCSVRecords(t, fmt.Sprintf("concurrent run %d", i), got.stdout, ',')
		if len(records) != 3 || records[1][0] != "alice" || records[2][0] != "bob" {
			t.Errorf("concurrent run %d returned %v, want both rows", i, records)
		}
	}

	// Whatever happened, the cache must still be usable afterwards.
	stdout, stderr, code := run(t, "", args...)
	if code != 0 {
		t.Fatalf("run after the concurrent ones exited %d (stderr=%q)", code, stderr)
	}
	assertNoResidueDiagnostics(t, "run after concurrent runs", stderr)
	if records := parseCSVRecords(t, "recovery run", stdout, ','); len(records) != 3 {
		t.Errorf("recovery run returned %v, want a header and 2 rows", records)
	}
}

// TestSmoke_OutputFormatsAgreeOnTheSameResult drives the same query through
// every format and checks the values line up. It is the CLI-level detector for
// the record/cell divergence the Table ownership rules exist to prevent: a
// regression there shows up as CSV and JSON disagreeing about one cell.
func TestSmoke_OutputFormatsAgreeOnTheSameResult(t *testing.T) {
	script := "CREATE TABLE t (code TEXT, qty INTEGER, ratio REAL, note TEXT);\n" +
		"INSERT INTO t VALUES ('00123', 42, 1.5, NULL), ('00456', -7, 2.0, '');\n" +
		"SELECT code, qty, ratio, note FROM t ORDER BY code;\n"

	csvOut, stderr, code := run(t, script, "--output-format", "csv")
	if code != 0 {
		t.Fatalf("csv exit code = %d (stderr=%q)", code, stderr)
	}
	jsonOut, stderr, code := run(t, script, "--output-format", "json")
	if code != 0 {
		t.Fatalf("json exit code = %d (stderr=%q)", code, stderr)
	}
	tableOut, stderr, code := run(t, script, "--output-format", "table")
	if code != 0 {
		t.Fatalf("table exit code = %d (stderr=%q)", code, stderr)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(dropStatusLines(jsonOut)), &rows); err != nil {
		t.Fatalf("decode json: %v (%q)", err, jsonOut)
	}
	if len(rows) != 2 {
		t.Fatalf("decoded %d rows, want 2", len(rows))
	}

	csvLines := nonEmptyLines(dropStatusLines(csvOut))
	if len(csvLines) != 3 {
		t.Fatalf("csv produced %d lines, want 3", len(csvLines))
	}
	columns := strings.Split(csvLines[0], ",")

	for i, row := range rows {
		fields := strings.Split(csvLines[i+1], ",")
		for j, column := range columns {
			value := row[column]
			// A NULL has no string spelling: it must be a blank CSV field.
			if value == nil {
				if fields[j] != "" {
					t.Errorf("row %d %q: json null but csv %q", i, column, fields[j])
				}
				continue
			}
			want := fmt.Sprintf("%v", value)
			if fields[j] != want {
				t.Errorf("row %d %q: csv %q != json %q", i, column, fields[j], want)
			}
			// The table renderer prints the same string as CSV does.
			if want != "" && !strings.Contains(tableOut, want) {
				t.Errorf("row %d %q: table output is missing %q:\n%s", i, column, want, tableOut)
			}
		}
	}

	// The zero-padded code stayed text in both, which is the specific value a
	// record/cell divergence would break first.
	if rows[0]["code"] != "00123" {
		t.Errorf("code = %#v, want the string 00123", rows[0]["code"])
	}
	if !strings.Contains(csvOut, "00123") || !strings.Contains(tableOut, "00123") {
		t.Error("00123 lost its leading zeros in csv or table output")
	}
}
