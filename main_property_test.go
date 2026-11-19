package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/config"
	"pgregory.net/rapid"
)

// generatedTable is a header and rows of plain cells: integers, decimals, short
// words and blanks, with no tab, quote, comma, colon or newline, so CSV, TSV
// and LTSV can all carry them as written and no format has a quoting rule to
// disagree about.
type generatedTable struct {
	header []string
	rows   [][]string
}

func drawGeneratedTable(t *rapid.T) generatedTable {
	cols := rapid.IntRange(1, 4).Draw(t, "cols")
	rowCount := rapid.IntRange(1, 15).Draw(t, "rows")
	header := make([]string, cols)
	kinds := make([]int, cols)
	for c := range cols {
		header[c] = "c" + strconv.Itoa(c)
		kinds[c] = rapid.IntRange(0, 2).Draw(t, "kind")
	}
	word := rapid.SampledFrom([]string{"apple", "Banana", "cherry", "日本", "x y", "zz", "A"})
	rows := make([][]string, rowCount)
	for r := range rowCount {
		row := make([]string, cols)
		for c := range cols {
			// c0 is never blank, so every row keeps a cell and the
			// queries that order and group by it have a value to use.
			if c > 0 && rapid.IntRange(0, 7).Draw(t, "blank") == 0 {
				continue
			}
			switch kinds[c] {
			case 0:
				row[c] = strconv.Itoa(rapid.IntRange(-1000, 1000).Draw(t, "int"))
			case 1:
				row[c] = strconv.FormatFloat(float64(rapid.IntRange(-100000, 100000).Draw(t, "cents"))/100, 'f', -1, 64)
			default:
				row[c] = word.Draw(t, "word")
			}
		}
		rows[r] = row
	}
	return generatedTable{header: header, rows: rows}
}

func (g generatedTable) csv() []byte {
	var b strings.Builder
	b.WriteString(strings.Join(g.header, ",") + "\n")
	for _, row := range g.rows {
		b.WriteString(strings.Join(row, ",") + "\n")
	}
	return []byte(b.String())
}

func (g generatedTable) tsv() []byte {
	var b strings.Builder
	b.WriteString(strings.Join(g.header, "\t") + "\n")
	for _, row := range g.rows {
		b.WriteString(strings.Join(row, "\t") + "\n")
	}
	return []byte(b.String())
}

func (g generatedTable) ltsv() []byte {
	var b strings.Builder
	for _, row := range g.rows {
		fields := make([]string, len(row))
		for c, v := range row {
			fields[c] = g.header[c] + ":" + v
		}
		b.WriteString(strings.Join(fields, "\t") + "\n")
	}
	return []byte(b.String())
}

func gzipped(t *rapid.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// generatedQueries are the questions asked of the generated table t. c0 is
// always there; ordering on every column keeps each answer deterministic.
func generatedQueries(cols int) []string {
	all := make([]string, cols)
	for c := range cols {
		all[c] = "c" + strconv.Itoa(c)
	}
	order := strings.Join(all, ", ")
	return []string{
		"SELECT * FROM t ORDER BY " + order,
		"SELECT c0, COUNT(*) AS n FROM t GROUP BY c0 ORDER BY c0",
		"SELECT COUNT(*), MIN(c0), MAX(c0) FROM t",
		"SELECT DISTINCT c0 FROM t ORDER BY c0 DESC",
		"SELECT * FROM t WHERE c0 > 0 ORDER BY " + order + " LIMIT 3",
	}
}

// runSqly runs sqly on one file and returns what it printed as CSV.
func runSqly(t *rapid.T, dir, name string, data []byte, query string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	backup := config.Stdout
	var out bytes.Buffer
	config.Stdout = &out
	code := run([]string{"sqly", "--output-format", "csv", "--sql", query, path})
	config.Stdout = backup
	if code != 0 {
		t.Fatalf("sqly exited %d on %s for %q", code, name, query)
	}
	return out.String()
}

// TestMetamorphic_TheSameTableAnswersTheSameFromEveryFormat writes one
// generated table as CSV, TSV, LTSV and gzip-compressed CSV and asks sqly the
// same question of each. The format is only how the file is written, so every
// answer has to be the one the plain CSV gives.
//
// It is not parallel: run prints through the config.Stdout global.
func TestMetamorphic_TheSameTableAnswersTheSameFromEveryFormat(t *testing.T) {
	base := t.TempDir()
	draw := 0
	rapid.Check(t, func(t *rapid.T) {
		table := drawGeneratedTable(t)
		query := rapid.SampledFrom(generatedQueries(len(table.header))).Draw(t, "query")
		draw++
		dir := filepath.Join(base, strconv.Itoa(draw))
		if err := os.Mkdir(dir, 0o750); err != nil {
			t.Fatal(err)
		}

		want := runSqly(t, dir, "t.csv", table.csv(), query)
		variants := map[string][]byte{
			"t.tsv":    table.tsv(),
			"t.ltsv":   table.ltsv(),
			"t.csv.gz": gzipped(t, table.csv()),
		}
		for name, data := range variants {
			sub := filepath.Join(dir, strings.ReplaceAll(name, ".", "_"))
			if err := os.Mkdir(sub, 0o750); err != nil {
				t.Fatal(err)
			}
			if got := runSqly(t, sub, name, data, query); got != want {
				t.Fatalf("%s answers %q differently from the CSV\nCSV:\n%s\n%s:\n%s", name, query, want, name, got)
			}
		}
	})
}

// TestDifferential_SqlyPrintsWhatTheDatabaseAnswers asks sqly a question and
// asks the same question of the database filesql loads from the same file,
// reading the rows through database/sql. sqly adds the shell, the argument
// handling and the CSV printer on top, and none of those may change a value,
// drop a row or reorder the columns.
//
// It is not parallel: run prints through the config.Stdout global.
func TestDifferential_SqlyPrintsWhatTheDatabaseAnswers(t *testing.T) {
	base := t.TempDir()
	draw := 0
	rapid.Check(t, func(t *rapid.T) {
		table := drawGeneratedTable(t)
		query := rapid.SampledFrom(generatedQueries(len(table.header))).Draw(t, "query")
		draw++
		dir := filepath.Join(base, strconv.Itoa(draw))
		if err := os.Mkdir(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		got := runSqly(t, dir, "t.csv", table.csv(), query)

		ctx := context.Background()
		db, err := filesql.Open(ctx, filepath.Join(dir, "t.csv"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}()
		want := queryAsCSV(t, db, query)
		if got != want {
			t.Fatalf("sqly prints other rows than the database answers for %q\nsqly:\n%s\ndatabase:\n%s", query, got, want)
		}
	})
}

// queryAsCSV runs query and writes the header and rows the way a CSV holds
// them: integers in decimal, text as it is.
func queryAsCSV(t *rapid.T, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(columns); err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			t.Fatal(err)
		}
		record := make([]string, len(values))
		for i, v := range values {
			switch v := v.(type) {
			case nil:
				record[i] = ""
			case []byte:
				record[i] = string(v)
			default:
				record[i] = fmt.Sprint(v)
			}
		}
		if err := w.Write(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
