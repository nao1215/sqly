package filesql

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	libfilesql "github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

// reimportRowCount writes the parquet file back into a fresh database via
// filesql and returns the row count of the given table.
func reimportRowCount(t *testing.T, parquetPath, tableName string) int {
	t.Helper()
	db, err := libfilesql.OpenContext(context.Background(), parquetPath)
	if err != nil {
		t.Fatalf("reimport parquet: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+tableName).Scan(&n); err != nil {
		t.Fatalf("count after reimport: %v", err)
	}
	return n
}

// TestDumpTableToParquet_RoundTrip locks the export target: a table
// written to Parquet must re-import into sqly with the same rows and columns.
func TestDumpTableToParquet_RoundTrip(t *testing.T) {
	t.Parallel()

	table := model.NewTable("people", model.Header{"id", "name"}, []model.Record{
		{"1", "alice"},
		{"2", "bob"},
		{"3", "carol"},
	})
	out := filepath.Join(t.TempDir(), "people.parquet")

	if err := DumpTableToParquet(out, table); err != nil {
		t.Fatalf("DumpTableToParquet: %v", err)
	}

	if got := reimportRowCount(t, out, "people"); got != 3 {
		t.Errorf("reimported rows = %d, want 3", got)
	}

	// Schema fidelity: the reimported table exposes the same columns.
	db, err := libfilesql.OpenContext(context.Background(), out)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info('people')")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 || cols[0] != "id" || cols[1] != "name" {
		t.Errorf("reimported columns = %v, want [id name]", cols)
	}

	// Cell fidelity: the reimported name column carries the same string values, not
	// just the same row and column counts.
	names := reimportStringColumn(t, out, "people", "name")
	if len(names) != 3 || names[0] != "alice" || names[1] != "bob" || names[2] != "carol" {
		t.Errorf("reimported names = %v, want [alice bob carol]", names)
	}
}

// reimportStringColumn writes the parquet file back via filesql and returns the
// named column's values as plain strings, so a test can assert exact cell
// fidelity after a round-trip.
func reimportStringColumn(t *testing.T, parquetPath, tableName, column string) []string {
	t.Helper()
	db, err := libfilesql.OpenContext(context.Background(), parquetPath)
	if err != nil {
		t.Fatalf("reimport parquet: %v", err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(context.Background(), "SELECT "+column+" FROM "+tableName) //nolint:gosec // test uses fixed fixture identifiers
	if err != nil {
		t.Fatalf("select %s: %v", column, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan %s: %v", column, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestDumpTableToParquet_PreservesNumericLookingText locks issue #687: numeric
// looking text such as leading-zero codes ("007") and decimal strings ("1.00")
// must survive a parquet round-trip verbatim instead of being coerced to a
// number by the staging column's affinity.
func TestDumpTableToParquet_PreservesNumericLookingText(t *testing.T) {
	t.Parallel()

	table := model.NewTable("codes", model.Header{"code", "amount"}, []model.Record{
		{"007", "1.00"},
		{"010", "2.50"},
	})
	out := filepath.Join(t.TempDir(), "codes.parquet")

	if err := DumpTableToParquet(out, table); err != nil {
		t.Fatalf("DumpTableToParquet: %v", err)
	}

	codes := reimportStringColumn(t, out, "codes", "code")
	if len(codes) != 2 || codes[0] != "007" || codes[1] != "010" {
		t.Errorf("code column = %v, want [007 010] (leading zeros preserved)", codes)
	}
	amounts := reimportStringColumn(t, out, "codes", "amount")
	if len(amounts) != 2 || amounts[0] != "1.00" || amounts[1] != "2.50" {
		t.Errorf("amount column = %v, want [1.00 2.50] (decimal text preserved)", amounts)
	}
}

// reimportColumn writes the parquet file back via filesql and returns the named
// column's values as sql.NullString, so a test can tell a SQL NULL apart from an
// empty string after a round-trip.
func reimportColumn(t *testing.T, parquetPath, tableName, column string) []sql.NullString {
	t.Helper()
	db, err := libfilesql.OpenContext(context.Background(), parquetPath)
	if err != nil {
		t.Fatalf("reimport parquet: %v", err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(context.Background(), "SELECT "+column+" FROM "+tableName) //nolint:gosec // test uses fixed fixture identifiers
	if err != nil {
		t.Fatalf("select %s: %v", column, err)
	}
	defer func() { _ = rows.Close() }()
	var out []sql.NullString
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan %s: %v", column, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestDumpTableToParquet_PreservesNull locks issue #686: a SQL NULL cell must
// stay NULL after a parquet round-trip instead of collapsing to an empty string,
// so NULL and "" remain distinguishable in machine-readable output.
func TestDumpTableToParquet_PreservesNull(t *testing.T) {
	t.Parallel()

	table, err := model.NewTableFromCells("nulls", model.Header{"id", "name"}, [][]model.Cell{
		{model.NewCell(nil), model.NewCell("A")}, // id is SQL NULL
		{model.NewCell(""), model.NewCell("B")},  // id is an empty string
		{model.NewCell(int64(1)), model.NewCell("C")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}
	out := filepath.Join(t.TempDir(), "nulls.parquet")

	if err := DumpTableToParquet(out, table); err != nil {
		t.Fatalf("DumpTableToParquet: %v", err)
	}

	ids := reimportColumn(t, out, "nulls", "id")
	if len(ids) != 3 {
		t.Fatalf("reimported %d rows, want 3", len(ids))
	}
	if ids[0].Valid {
		t.Errorf("row 0 id = %q, want SQL NULL", ids[0].String)
	}
	if !ids[1].Valid || ids[1].String != "" {
		t.Errorf("row 1 id = %#v, want an empty string (not NULL)", ids[1])
	}
	if !ids[2].Valid || ids[2].String != "1" {
		t.Errorf("row 2 id = %#v, want \"1\"", ids[2])
	}
}

// TestDumpTableToParquet_TextStagedColumnKeepsTheDisplayedNumber pins that a
// column staged as TEXT — because it mixes numbers with something that is not a
// number — reaches Parquet holding the text sqly showed, not SQLite's rendering
// of the bound value.
//
// Staging as TEXT is what keeps "007" and "1.00" intact; binding the driver's
// float64 into it let SQLite render the value, and a query that printed 100000
// exported "100000.0".
func TestDumpTableToParquet_TextStagedColumnKeepsTheDisplayedNumber(t *testing.T) {
	t.Parallel()

	// An empty string alongside the numbers is what forces TEXT staging: SQLite
	// types values rather than columns, and a column holding text has to keep it.
	table, err := model.NewTableFromCells("mixed", model.Header{"n"}, [][]model.Cell{
		{model.NewCell(float64(100000))},
		{model.NewCell(float64(2.5))},
		{model.NewCell("")},
	})
	if err != nil {
		t.Fatalf("NewTableFromCells: %v", err)
	}
	out := filepath.Join(t.TempDir(), "mixed.parquet")

	if err := DumpTableToParquet(out, table); err != nil {
		t.Fatalf("DumpTableToParquet: %v", err)
	}

	got := reimportStringColumn(t, out, "mixed", "n")
	want := []string{"100000", "2.5", ""}
	if len(got) != len(want) {
		t.Fatalf("reimported %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d n = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDumpTableToParquet_EmptyResult covers the empty-result behavior: Parquet
// needs at least one row to infer its schema, so exporting an empty result
// returns a clear error rather than writing an unreadable file.
func TestDumpTableToParquet_EmptyResult(t *testing.T) {
	t.Parallel()

	table := model.NewTable("empty", model.Header{"a", "b"}, []model.Record{})
	out := filepath.Join(t.TempDir(), "empty.parquet")

	err := DumpTableToParquet(out, table)
	if err == nil {
		t.Fatal("DumpTableToParquet on empty result = nil error, want error")
	}
	if !strings.Contains(err.Error(), "empty result") {
		t.Errorf("error = %q, want it to mention the empty result", err.Error())
	}
}

// TestDumpTableToParquet_PreservesValuesSQLCannotParse covers values that are
// data to the driver but syntax to the SQL parser. The staging INSERT used to be
// assembled as SQL text with every value quoted into it, so a NUL byte — which
// SQLite's tokenizer treats as the end of the statement — left the literal
// unclosed and the export failed with "unrecognized token". The same value
// exports through every other format, so nothing about it is unexportable.
func TestDumpTableToParquet_PreservesValuesSQLCannotParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "a NUL byte ends the SQL literal but not the value", value: "A\x00B"},
		{name: "an apostrophe is data, not the end of a literal", value: "it's"},
		{name: "a doubled apostrophe is two characters", value: "it''s"},
		{name: "a backslash is not an escape", value: `back\slash`},
		{name: "a statement separator is data", value: "one; DROP TABLE t; --"},
		{name: "a newline inside a value", value: "line\nbreak"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			table, err := model.NewTableFromCells("odd", model.Header{"v"}, [][]model.Cell{
				{model.NewCell(tt.value)},
			})
			if err != nil {
				t.Fatalf("NewTableFromCells: %v", err)
			}
			out := filepath.Join(t.TempDir(), "odd.parquet")

			if err := DumpTableToParquet(out, table); err != nil {
				t.Fatalf("DumpTableToParquet: %v", err)
			}

			got := reimportColumn(t, out, "odd", "v")
			if len(got) != 1 {
				t.Fatalf("reimported %d rows, want 1", len(got))
			}
			if !got[0].Valid || got[0].String != tt.value {
				t.Errorf("value = %#v, want %q", got[0], tt.value)
			}
		})
	}
}
