package interactor

import (
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestStripSQLNoise_UnterminatedBlockComment covers the branch where a leading
// block comment is never closed: nothing executable remains, so the whole input
// is dropped and "" is returned.
func TestStripSQLNoise_UnterminatedBlockComment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "block comment with no closing marker returns empty",
			in:   "/* this comment never ends",
			want: "",
		},
		{
			name: "whitespace then unterminated block comment returns empty",
			in:   "   \n\t/* still open",
			want: "",
		},
		{
			name: "line comment running to end of input returns empty",
			in:   "-- only a line comment",
			want: "",
		},
		{
			name: "block comment then statement keeps the statement",
			in:   "/* header */ SELECT 1",
			want: "SELECT 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := stripSQLNoise(tc.in); got != tc.want {
				t.Errorf("stripSQLNoise(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestMainStatementVerb_QuotedRegionsBeforeVerb covers the quoted-identifier scan
// branches in mainStatementVerb: a double-quoted, backtick-quoted, or
// bracket-quoted CTE name appears at depth 0 before the main verb, so the scanner
// must skip those quoted regions and still report the verb the CTEs feed.
func TestMainStatementVerb_QuotedRegionsBeforeVerb(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "double-quoted CTE name before SELECT yields SELECT",
			in:   `WITH "my cte" AS (SELECT 1) SELECT * FROM "my cte"`,
			want: sqlSELECT,
		},
		{
			name: "backtick-quoted CTE name before UPDATE yields UPDATE",
			in:   "WITH `my cte` AS (SELECT 1) UPDATE t SET a = 1",
			want: sqlUPDATE,
		},
		{
			name: "bracket-quoted CTE name before DELETE yields DELETE",
			in:   "WITH [my cte] AS (SELECT 1) DELETE FROM t",
			want: sqlDELETE,
		},
		{
			name: "single-quoted literal before INSERT yields INSERT",
			in:   "WITH c AS (SELECT 'returning') INSERT INTO t VALUES (1)",
			want: sqlINSERT,
		},
		{
			name: "line comment before verb is skipped",
			in:   "WITH c AS (SELECT 1) -- comment\n REPLACE INTO t VALUES (1)",
			want: sqlREPLACE,
		},
		{
			name: "no main verb found returns empty",
			in:   "WITH c AS (SELECT 1) VACUUM",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := mainStatementVerb(tc.in); got != tc.want {
				t.Errorf("mainStatementVerb(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestHasReturningClause_QuotedAndCommented locks the word-boundary and
// quoted-region handling of hasReturningClause: RETURNING inside a string
// literal, a quoted identifier, or a comment must not count, while a real
// RETURNING clause must.
func TestHasReturningClause_QuotedAndCommented(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{
			name: "real RETURNING clause is detected",
			in:   "INSERT INTO t VALUES (1) RETURNING id",
			want: true,
		},
		{
			name: "RETURNING inside a single-quoted literal is ignored",
			in:   "INSERT INTO t VALUES ('returning')",
			want: false,
		},
		{
			name: "RETURNING inside a double-quoted identifier is ignored",
			in:   `UPDATE t SET "returning" = 1`,
			want: false,
		},
		{
			name: "RETURNING inside a line comment is ignored",
			in:   "DELETE FROM t -- returning id\n",
			want: false,
		},
		{
			name: "RETURNING inside a block comment is ignored",
			in:   "DELETE FROM t /* returning id */",
			want: false,
		},
		{
			name: "word-boundary: RETURNING_AT is not a RETURNING clause",
			in:   "UPDATE t SET returning_at = 1",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := hasReturningClause(tc.in); got != tc.want {
				t.Errorf("hasReturningClause(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestDumpTable_InitCompressionError covers the withCompressedWriter branch where
// building the compression codec fails: bzip2 has no writer, so DumpTable must
// surface an "init compression" error after the output file is created.
func TestDumpTable_InitCompressionError(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("t", model.NewHeader([]string{"id"}), []model.Record{
		model.NewRecord([]string{"1"}),
	})
	out := filepath.Join(t.TempDir(), "out.csv.bz2")

	err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionBzip2)
	if err == nil {
		t.Fatal("DumpTable with bzip2 compression = nil error, want error (bzip2 has no writer)")
	}
}

// TestDumpTable_Parquet covers the ExportParquet case of DumpTable by exporting a
// non-empty table and asserting the file is written.
func TestDumpTable_Parquet(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("people", model.NewHeader([]string{"id", "name"}), []model.Record{
		model.NewRecord([]string{"1", "alice"}),
	})
	out := filepath.Join(t.TempDir(), "people.parquet")

	if err := exp.DumpTable(out, table, model.ExportParquet, model.CompressionNone); err != nil {
		t.Fatalf("DumpTable Parquet failed: %v", err)
	}
}

// TestDumpTable_ParquetEmpty covers the ExportParquet error path routed through
// DumpTable: an empty result cannot be written as Parquet.
func TestDumpTable_ParquetEmpty(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("empty", model.NewHeader([]string{"id"}), []model.Record{})
	out := filepath.Join(t.TempDir(), "empty.parquet")

	if err := exp.DumpTable(out, table, model.ExportParquet, model.CompressionNone); err == nil {
		t.Fatal("DumpTable Parquet on empty result = nil error, want error")
	}
}
