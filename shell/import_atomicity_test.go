package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/testutil"
)

// An import of several files either loads all of them or none of them. These
// tests are the "none of them" half, and they check more than the error: a
// rollback that leaves a table behind, or a session that keeps a source record
// for a table that no longer exists, has failed in a way an exit code cannot
// show.
//
// The corrupt file is placed first, in the middle, and last. Middle and last are
// the positions that matter: they are the ones where files were read
// successfully before the failure, so they are the only ones that can prove
// anything was rolled back. A suite that only tested a corrupt first file would
// pass with no rollback at all.

// sessionState is everything an import may change about a session, gathered so
// a failed import can be checked against what it was before.
type sessionState struct {
	tables   []string
	sources  map[string]string
	dirMarks map[string]bool
	baseline map[string]string
	sourceBL map[string]string
	sheets   int
}

// captureSessionState reads the session and the database.
func captureSessionState(t *testing.T, s *Shell) sessionState {
	t.Helper()
	tables, err := s.usecases.metadata.TablesName(context.Background())
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	names := make([]string, 0, len(tables))
	for _, table := range tables {
		names = append(names, table.Name())
	}
	return sessionState{
		tables:   names,
		sources:  copyStringMap(s.tableSources),
		dirMarks: copyBoolMap(s.dirImported),
		baseline: copyStringMap(s.importBaseline),
		sourceBL: copyStringMap(s.sourceBaseline),
		sheets:   len(s.excelWorkbooks),
	}
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// assertUnchanged reports every way the session moved when it should not have.
func assertUnchanged(t *testing.T, before, after sessionState) {
	t.Helper()

	if strings.Join(sorted(before.tables), ",") != strings.Join(sorted(after.tables), ",") {
		t.Errorf("tables changed across a failed import:\n before %v\n after  %v", before.tables, after.tables)
	}
	if len(before.sources) != len(after.sources) {
		t.Errorf("table sources changed across a failed import:\n before %v\n after  %v", before.sources, after.sources)
	}
	for name, src := range after.sources {
		if before.sources[name] != src {
			t.Errorf("a failed import recorded %s as coming from %s", name, src)
		}
	}
	for name := range after.dirMarks {
		if !before.dirMarks[name] {
			t.Errorf("a failed import marked %s as a directory import", name)
		}
	}
	for name, fp := range after.baseline {
		if before.baseline[name] != fp {
			t.Errorf("a failed import moved the content baseline of %s", name)
		}
	}
	for name, fp := range after.sourceBL {
		if before.sourceBL[name] != fp {
			t.Errorf("a failed import moved the source baseline of %s", name)
		}
	}
	if after.sheets != before.sheets {
		t.Errorf("a failed import published workbook sheet metadata: %d records, was %d", after.sheets, before.sheets)
	}
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// writeGoodCSV writes a readable one-row CSV.
func writeGoodCSV(t *testing.T, dir, name, value string) string {
	t.Helper()
	path := filepath.Join(dir, name+".csv")
	if err := os.WriteFile(path, []byte("v\n"+value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestImportAtomicity_OneBrokenInputRollsBackTheWhole is the core of the
// contract, run for every way a file can be broken and every position it can
// take among its readable neighbors.
func TestImportAtomicity_OneBrokenInputRollsBackTheWhole(t *testing.T) {
	kinds := []testutil.CorruptKind{
		testutil.CorruptNotAZip,
		testutil.CorruptTruncatedZip,
		testutil.CorruptInnerXLSX,
		testutil.CorruptOuterCompression,
		testutil.CorruptOuterZstd,
		testutil.CorruptTruncatedJSONL,
		testutil.CorruptTrailingGarbageJSON,
		testutil.CorruptInvalidParquet,
	}
	positions := []struct {
		name  string
		index int
	}{
		{name: "first", index: 0},
		{name: "middle", index: 1},
		{name: "last", index: 2},
	}

	for _, kind := range kinds {
		for _, position := range positions {
			t.Run(kind.String()+"/"+position.name, func(t *testing.T) {
				s, cleanup, err := newShell(t, []string{"sqly"})
				if err != nil {
					t.Fatal(err)
				}
				defer cleanup()
				ctx := context.Background()

				dir := t.TempDir()
				// A table that existed before the import must survive it.
				preexisting := writeGoodCSV(t, dir, "kept", "before")
				if err := s.commands.importCommand(ctx, s, []string{preexisting}); err != nil {
					t.Fatalf("set up the pre-existing table: %v", err)
				}
				before := captureSessionState(t, s)

				broken := testutil.WriteCorruptFixture(t, dir, "broken", kind)
				inputs := []string{
					writeGoodCSV(t, dir, "alpha", "1"),
					writeGoodCSV(t, dir, "beta", "2"),
				}
				// Splice the broken file into the requested position.
				argv := make([]string, 0, 3)
				argv = append(argv, inputs[:position.index]...)
				argv = append(argv, broken)
				argv = append(argv, inputs[position.index:]...)

				importErr := captureStderrString(t, func() error {
					return s.commands.importCommand(ctx, s, argv)
				})
				if importErr == nil {
					t.Fatalf("importCommand succeeded with a %s among its inputs", kind)
				}
				if got := ExitCode(importErr); got != ExitInput {
					t.Errorf("ExitCode = %d, want %d (%v)", got, ExitInput, importErr)
				}
				if !strings.Contains(importErr.Error(), filepath.Base(broken)) {
					t.Errorf("error %q should name the broken input %q", importErr, filepath.Base(broken))
				}

				after := captureSessionState(t, s)
				assertUnchanged(t, before, after)

				// The readable inputs specifically: neither the one before the
				// broken file nor the one after it may have survived.
				for _, table := range []string{"alpha", "beta"} {
					for _, name := range after.tables {
						if name == table {
							t.Errorf("table %q from a readable input survived a failed import", table)
						}
					}
				}
				// And the table that was there first is still there.
				found := false
				for _, name := range after.tables {
					if name == "kept" {
						found = true
					}
				}
				if !found {
					t.Error("a failed import removed a table the session already held")
				}
			})
		}
	}
}

// TestImportAtomicity_ReadableWorkbookLeavesNoSheetRecord covers what the
// CSV-only cases above cannot. A workbook is the one input that records
// something beyond a table — the sheets it holds and which of them were
// imported, which --inspect reports — and that record is a claim about a
// database. A failed import that published it would describe sheets of a
// workbook whose rows were rolled back.
func TestImportAtomicity_ReadableWorkbookLeavesNoSheetRecord(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx := context.Background()

	dir := t.TempDir()
	workbook := writeWorkbook(t, dir, "book.xlsx",
		sheetSpec{name: "Visible", value: "shown"},
		sheetSpec{name: "Internal", hidden: true, value: "hidden"},
	)
	broken := testutil.WriteCorruptFixture(t, dir, "broken", testutil.CorruptTruncatedZip)

	before := captureSessionState(t, s)
	if before.sheets != 0 {
		t.Fatalf("the session already holds %d workbook records", before.sheets)
	}

	// The workbook comes first, so it is read before the failure. Its sheet
	// record, its tables, and its baselines must all be rolled back with it.
	importErr := captureStderrString(t, func() error {
		return s.commands.importCommand(ctx, s, []string{workbook, broken})
	})
	if importErr == nil {
		t.Fatal("the import succeeded with a broken workbook among its inputs")
	}

	after := captureSessionState(t, s)
	assertUnchanged(t, before, after)
	if after.sheets != 0 {
		t.Errorf("a failed import published %d workbook sheet records", after.sheets)
	}
	if len(s.excelSheetReports()) != 0 {
		t.Errorf("--inspect would report sheets for a workbook that was rolled back: %v", s.excelSheetReports())
	}
	for _, name := range after.tables {
		if strings.HasPrefix(name, "book") {
			t.Errorf("table %q from the readable workbook survived a failed import", name)
		}
	}

	// And the same import without the broken file does record them, so the
	// check above is not passing because nothing ever records anything.
	if err := captureStderrString(t, func() error {
		return s.commands.importCommand(ctx, s, []string{workbook})
	}); err != nil {
		t.Fatalf("the workbook alone failed to import: %v", err)
	}
	if len(s.excelSheetReports()) == 0 {
		t.Error("a successful workbook import recorded no sheets; the earlier assertion proves nothing")
	}
}

// TestImportAtomicity_RetryAfterRepairMatchesACleanRun is the other half of
// "nothing was left behind": if the failure really left no trace, replacing the
// broken file and importing again in the same session has to reach the state a
// fresh session would.
func TestImportAtomicity_RetryAfterRepairMatchesACleanRun(t *testing.T) {
	ctx := context.Background()

	// The session that fails, is repaired, and retries.
	retried, cleanupRetried, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupRetried()

	dir := t.TempDir()
	alpha := writeGoodCSV(t, dir, "alpha", "1")
	beta := writeGoodCSV(t, dir, "beta", "2")
	broken := testutil.WriteCorruptFixture(t, dir, "middle", testutil.CorruptTruncatedZip)

	failed := captureStderrString(t, func() error {
		return retried.commands.importCommand(ctx, retried, []string{alpha, broken, beta})
	})
	if failed == nil {
		t.Fatal("the import with a broken file succeeded")
	}

	// Repair: replace the broken workbook with a readable CSV of the same table
	// name, and import exactly the same command again.
	repaired := filepath.Join(dir, "middle.csv")
	if err := os.WriteFile(repaired, []byte("v\n9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := retried.commands.importCommand(ctx, retried, []string{alpha, repaired, beta}); err != nil {
		t.Fatalf("the repaired import failed in the same session: %v", err)
	}

	// A session that never saw the broken file at all.
	clean, cleanupClean, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupClean()
	if err := clean.commands.importCommand(ctx, clean, []string{alpha, repaired, beta}); err != nil {
		t.Fatalf("the clean import failed: %v", err)
	}

	retriedState := captureSessionState(t, retried)
	cleanState := captureSessionState(t, clean)

	if strings.Join(sorted(retriedState.tables), ",") != strings.Join(sorted(cleanState.tables), ",") {
		t.Errorf("the retried session holds %v, the clean one %v", sorted(retriedState.tables), sorted(cleanState.tables))
	}
	if len(retriedState.sources) != len(cleanState.sources) {
		t.Errorf("the retried session records %d sources, the clean one %d", len(retriedState.sources), len(cleanState.sources))
	}
	for name, src := range cleanState.sources {
		if retriedState.sources[name] != src {
			t.Errorf("source of %s: retried %q, clean %q", name, retriedState.sources[name], src)
		}
	}
	// The rows too, not only the shape.
	for _, table := range []string{"alpha", "middle", "beta"} {
		if got, want := queryOne(t, retried, table), queryOne(t, clean, table); got != want {
			t.Errorf("table %s holds %q after the retry, %q in a clean run", table, got, want)
		}
	}
}

// TestImportAtomicity_CollisionIsRefusedBeforeAnythingLoads pins the other way a
// multi-input import can fail: two inputs that want the same table. Picking one
// would leave the other's rows missing with nothing said, so the import is
// refused — and refused before it has written anything.
func TestImportAtomicity_CollisionIsRefusedBeforeAnythingLoads(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		build func(t *testing.T, dir string) []string
		table string
	}{
		{
			name: "the same base name in two directories",
			build: func(t *testing.T, dir string) []string {
				t.Helper()
				a := filepath.Join(dir, "a")
				b := filepath.Join(dir, "b")
				for _, sub := range []string{a, b} {
					if err := os.MkdirAll(sub, 0o750); err != nil {
						t.Fatal(err)
					}
				}
				return []string{
					writeGoodCSV(t, a, "book", "from-a"),
					writeGoodCSV(t, b, "book", "from-b"),
				}
			},
			table: "book",
		},
		{
			name: "two names that sanitize to one table",
			build: func(t *testing.T, dir string) []string {
				t.Helper()
				a := filepath.Join(dir, "a")
				b := filepath.Join(dir, "b")
				for _, sub := range []string{a, b} {
					if err := os.MkdirAll(sub, 0o750); err != nil {
						t.Fatal(err)
					}
				}
				return []string{
					writeGoodCSV(t, a, "user-data", "hyphen"),
					writeGoodCSV(t, b, "user data", "space"),
				}
			},
			table: "user_data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, cleanup, err := newShell(t, []string{"sqly"})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			dir := t.TempDir()
			inputs := tt.build(t, dir)
			// A third, unrelated input, so the test can tell "refused everything"
			// from "refused the pair".
			inputs = append(inputs, writeGoodCSV(t, dir, "other", "3"))

			before := captureSessionState(t, s)
			importErr := captureStderrString(t, func() error {
				return s.commands.importCommand(ctx, s, inputs)
			})
			if importErr == nil {
				t.Fatal("importCommand succeeded on two inputs that want one table")
			}
			if got := ExitCode(importErr); got != ExitInput {
				t.Errorf("ExitCode = %d, want %d", got, ExitInput)
			}
			for _, want := range []string{inputs[0], inputs[1], tt.table} {
				if !strings.Contains(importErr.Error(), want) {
					t.Errorf("error %q should name %q", importErr, want)
				}
			}
			assertUnchanged(t, before, captureSessionState(t, s))
		})
	}
}

// TestImportAtomicity_SamePathTwiceIsOneInput checks the case that looks like a
// collision and is not: a file named twice, or a file named alongside the
// directory holding it, is one source and is read once.
func TestImportAtomicity_SamePathTwiceIsOneInput(t *testing.T) {
	ctx := context.Background()

	t.Run("the same path repeated", func(t *testing.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		dir := t.TempDir()
		path := writeGoodCSV(t, dir, "once", "1")
		if err := s.commands.importCommand(ctx, s, []string{path, path}); err != nil {
			t.Fatalf("a path named twice was rejected: %v", err)
		}
		if got := queryOne(t, s, "once"); got != "1" {
			t.Errorf("once holds %q, want %q", got, "1")
		}
	})

	t.Run("a file and the directory holding it", func(t *testing.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()

		dir := t.TempDir()
		path := writeGoodCSV(t, dir, "once", "1")
		if err := s.commands.importCommand(ctx, s, []string{path, dir}); err != nil {
			t.Fatalf("a file named alongside its directory was rejected: %v", err)
		}
		if got := queryOne(t, s, "once"); got != "1" {
			t.Errorf("once holds %q, want %q", got, "1")
		}
	})
}

// TestImportAtomicity_DirectoryImportRollsBack applies the contract to a
// directory: one unreadable file in it means none of its files load.
func TestImportAtomicity_DirectoryImportRollsBack(t *testing.T) {
	ctx := context.Background()

	// The name decides the position in the sorted walk, so this covers a broken
	// file first, in the middle, and last without depending on readdir order.
	for _, brokenName := range []string{"01-broken", "02-broken", "04-broken"} {
		t.Run(brokenName, func(t *testing.T) {
			s, cleanup, err := newShell(t, []string{"sqly"})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			dir := t.TempDir()
			writeGoodCSV(t, dir, "01-good", "1")
			writeGoodCSV(t, dir, "03-good", "3")
			broken := testutil.WriteCorruptFixture(t, dir, brokenName, testutil.CorruptTruncatedZip)

			before := captureSessionState(t, s)
			importErr := captureStderrString(t, func() error {
				return s.commands.importCommand(ctx, s, []string{dir})
			})
			if importErr == nil {
				t.Fatal("a directory holding an unreadable file imported successfully")
			}
			if !strings.Contains(importErr.Error(), filepath.Base(broken)) {
				t.Errorf("error %q should name the unreadable file", importErr)
			}
			assertUnchanged(t, before, captureSessionState(t, s))

			// Repairing the directory and importing it again must work in the
			// same session.
			if err := os.Remove(broken); err != nil {
				t.Fatal(err)
			}
			if err := captureStderrString(t, func() error {
				return s.commands.importCommand(ctx, s, []string{dir})
			}); err != nil {
				t.Fatalf("the repaired directory failed to import: %v", err)
			}
			// A name starting with a digit is not a bare SQL identifier, so the
			// sanitizer prefixes it.
			for _, table := range []string{"sheet_01_good", "sheet_03_good"} {
				if got := queryOne(t, s, table); got == "" {
					t.Errorf("table %s is missing after the repaired import", table)
				}
			}
		})
	}
}

// queryOne returns the single value in a one-row, one-column table, or "" when
// the table is not there.
func queryOne(t *testing.T, s *Shell, table string) string {
	t.Helper()
	result, err := s.usecases.query.Query(context.Background(),
		"SELECT v FROM "+s.usecases.importer.QuoteIdentifier(table))
	if err != nil {
		return ""
	}
	records := result.Records()
	if len(records) == 0 || len(records[0]) == 0 {
		return ""
	}
	return records[0][0]
}

// captureStderrString runs f with stderr swapped away, so an import's
// diagnostics do not flood the test log, and returns f's error.
func captureStderrString(t *testing.T, f func() error) error {
	t.Helper()
	var err error
	captureStderr(t, func() { err = f() })
	return err
}
