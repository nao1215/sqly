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
	rows, err := db.QueryContext(context.Background(), "SELECT "+column+" FROM "+tableName)
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
	rows, err := db.QueryContext(context.Background(), "SELECT "+column+" FROM "+tableName)
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

	table := model.NewTable("nulls", model.Header{"id", "name"}, []model.Record{
		{"", "A"}, // id is SQL NULL
		{"", "B"}, // id is an empty string
		{"1", "C"},
	})
	table.SetNulls([][]bool{
		{true, false},
		{false, false},
		{false, false},
	})
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
