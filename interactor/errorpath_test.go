package interactor

import (
	"os"
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

func TestDumpTable_PreservesExistingFileOnFailure(t *testing.T) {
	t.Parallel()

	t.Run("preserves existing file and cleans up temp files when LTSV dump fails due to tab", func(t *testing.T) {
		t.Parallel()

		exp := newTestExportInteractor()
		// Table with a tab character in a value, which is invalid in LTSV format
		table := model.NewTable("test", model.NewHeader([]string{"id", "name"}), []model.Record{
			model.NewRecord([]string{"1", "alice\tbob"}),
		})

		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "original.ltsv")

		// Create an existing file with content
		originalContent := []byte("original data")
		if err := os.WriteFile(outPath, originalContent, 0o600); err != nil { //nolint:gosec // test file with controlled path
			t.Fatalf("failed to write original file: %v", err)
		}

		// Try to dump invalid table to the file path, which should fail
		err := exp.DumpTable(outPath, table, model.ExportLTSV, model.CompressionNone)
		if err == nil {
			t.Fatal("expected DumpTable to fail due to tab in LTSV value, but it succeeded")
		}

		// Verify that the file still contains its original data and has not been truncated
		currentContent, err := os.ReadFile(outPath) //nolint:gosec // test file with controlled path
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(currentContent) != string(originalContent) {
			t.Errorf("file was altered! got %q, want %q", string(currentContent), string(originalContent))
		}
	})
}

func TestDumpTable_PreservesFilePermissions(t *testing.T) {
	t.Parallel()

	t.Run("preserves file permissions on successful overwrite", func(t *testing.T) {
		t.Parallel()

		exp := newTestExportInteractor()
		table := model.NewTable("test", model.NewHeader([]string{"id", "name"}), []model.Record{
			model.NewRecord([]string{"1", "alice"}),
		})

		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "output.ltsv")

		// Create file with custom permission (0o600)
		const customPerm = os.FileMode(0o600)
		if err := os.WriteFile(outPath, []byte("old content"), customPerm); err != nil { //nolint:gosec // test file with controlled path
			t.Fatalf("failed to write original file: %v", err)
		}

		infoBefore, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("failed to stat file before: %v", err)
		}

		// Perform successful dump
		if err := exp.DumpTable(outPath, table, model.ExportLTSV, model.CompressionNone); err != nil {
			t.Fatalf("failed to dump table: %v", err)
		}

		infoAfter, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("failed to stat file after: %v", err)
		}

		if infoBefore.Mode().Perm() != infoAfter.Mode().Perm() {
			t.Errorf("file permissions changed! got %v, want %v", infoAfter.Mode().Perm(), infoBefore.Mode().Perm())
		}
	})
}
