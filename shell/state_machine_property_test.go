package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"pgregory.net/rapid"
)

// TestShellSchemaCacheStateMachine drives one shell session through random
// sequences of schema and transaction statements and, after every one, compares
// what completion and highlighting would show with what SQLite itself reports.
//
// The session database is the oracle, so the test needs no model of what each
// statement does: whatever order the statements run in, the cached schema must
// equal a fresh read. Single-statement tests cannot cover this, because the
// cache only goes wrong for some orders of otherwise correct statements.
//
// Not parallel: exec writes its result line to config.Stdout, which the test
// swaps for io.Discard.
func TestShellSchemaCacheStateMachine(t *testing.T) {
	backup := config.Stdout
	config.Stdout = io.Discard
	t.Cleanup(func() { config.Stdout = backup })

	rapid.Check(t, func(rt *rapid.T) {
		s, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			rt.Fatal(err)
		}
		defer cleanup()
		m := &schemaCacheMachine{s: s}
		rt.Repeat(m.actions())
	})
}

type schemaCacheMachine struct {
	s      *Shell
	serial int
}

// run executes a statement the way a typed line is executed. A statement SQLite
// rejects (ALTER on a table a rollback removed, COMMIT with no transaction) is
// part of the sequence too: a failure must leave the cache as correct as a
// success does.
func (m *schemaCacheMachine) run(rt *rapid.T, stmt string) {
	rt.Log(stmt)
	_ = m.s.exec(context.Background(), stmt)
}

func (m *schemaCacheMachine) tables(rt *rapid.T) []string {
	tables, err := m.s.usecases.metadata.TablesName(context.Background())
	if err != nil {
		rt.Fatalf("TablesName: %v", err)
	}
	names := make([]string, 0, len(tables))
	for _, tbl := range tables {
		names = append(names, tbl.Name())
	}
	return names
}

func (m *schemaCacheMachine) pickTable(rt *rapid.T) string {
	names := m.tables(rt)
	if len(names) == 0 {
		rt.Skip("no table")
	}
	return rapid.SampledFrom(names).Draw(rt, "table")
}

func (m *schemaCacheMachine) next(prefix string) string {
	m.serial++
	return fmt.Sprintf("%s%d", prefix, m.serial)
}

func (m *schemaCacheMachine) actions() map[string]func(*rapid.T) {
	return map[string]func(*rapid.T){
		"create": func(rt *rapid.T) {
			m.run(rt, fmt.Sprintf("CREATE TABLE %s (id INTEGER)", m.next("t")))
		},
		"addColumn": func(rt *rapid.T) {
			m.run(rt, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT", m.pickTable(rt), m.next("c")))
		},
		"renameColumn": func(rt *rapid.T) {
			table := m.pickTable(rt)
			m.run(rt, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN id TO %s", table, m.next("id")))
		},
		"drop": func(rt *rapid.T) {
			m.run(rt, "DROP TABLE "+m.pickTable(rt))
		},
		"insert": func(rt *rapid.T) {
			m.run(rt, fmt.Sprintf("INSERT INTO %s DEFAULT VALUES", m.pickTable(rt)))
		},
		"begin":      func(rt *rapid.T) { m.run(rt, "BEGIN") },
		"commit":     func(rt *rapid.T) { m.run(rt, "COMMIT") },
		"rollback":   func(rt *rapid.T) { m.run(rt, "ROLLBACK") },
		"savepoint":  func(rt *rapid.T) { m.run(rt, "SAVEPOINT sp") },
		"rollbackTo": func(rt *rapid.T) { m.run(rt, "ROLLBACK TO sp") },
		"release":    func(rt *rapid.T) { m.run(rt, "RELEASE sp") },
		"": func(rt *rapid.T) {
			m.checkCompletionSchema(rt)
			m.checkHighlightNames(rt)
		},
	}
}

// fresh reads every table's columns straight from the session database,
// bypassing the cache.
func (m *schemaCacheMachine) fresh(rt *rapid.T) map[string][]string {
	out := map[string][]string{}
	for _, name := range m.tables(rt) {
		header, err := m.s.usecases.metadata.Header(context.Background(), name)
		if err != nil {
			rt.Fatalf("Header(%s): %v", name, err)
		}
		out[name] = header.Header()
	}
	return out
}

func (m *schemaCacheMachine) checkCompletionSchema(rt *rapid.T) {
	want := m.fresh(rt)
	got := m.s.schemaForCompletion(context.Background())
	if len(got.tables) != len(want) {
		rt.Fatalf("completion tables = %v, database has %v", got.tables, want)
	}
	for _, name := range got.tables {
		if !slices.Equal(got.columns[name], want[name]) {
			rt.Fatalf("completion columns of %s = %v, database has %v", name, got.columns[name], want[name])
		}
	}
}

func (m *schemaCacheMachine) checkHighlightNames(rt *rapid.T) {
	want := m.fresh(rt)
	got := m.s.schemaNames(context.Background())
	wantColumns := 0
	seen := map[string]bool{}
	for _, cols := range want {
		for _, c := range cols {
			if !seen[c] {
				seen[c] = true
				wantColumns++
			}
		}
	}
	if len(got.tables) != len(want) || len(got.columns) != wantColumns {
		rt.Fatalf("highlight names: %d tables, %d columns; database has %d tables, %d columns (%v)",
			len(got.tables), len(got.columns), len(want), wantColumns, want)
	}
}

// TestShellWriteBackStateMachine edits an imported file through random
// sequences of DML and saves, and after every save reads the written file into a
// second session and compares it with the table the first session holds.
//
// The re-read is the oracle: a save is correct when the file it leaves behind
// loads back as the table that was saved, whatever edits and earlier saves came
// before it. Not parallel, for the same reason as the schema cache test.
func TestShellWriteBackStateMachine(t *testing.T) {
	backupOut, backupErr := config.Stdout, config.Stderr
	config.Stdout, config.Stderr = io.Discard, io.Discard
	t.Cleanup(func() { config.Stdout, config.Stderr = backupOut, backupErr })

	rapid.Check(t, func(rt *rapid.T) {
		format := rapid.SampledFrom([]string{"csv", "tsv", "ltsv"}).Draw(rt, "format")
		dir := t.TempDir()
		source := filepath.Join(dir, "people."+format)
		if err := os.WriteFile(source, []byte(writeBackSeed(format)), 0o600); err != nil {
			rt.Fatal(err)
		}
		s, cleanup, err := newShell(t, []string{"sqly", source})
		if err != nil {
			rt.Fatal(err)
		}
		defer cleanup()
		if err := s.init(context.Background()); err != nil {
			rt.Fatal(err)
		}
		m := &writeBackMachine{s: s, source: source, outDir: filepath.Join(dir, "out"), t: t}
		rt.Repeat(m.actions())
	})
}

func writeBackSeed(format string) string {
	switch format {
	case "tsv":
		return "id\tname\tnote\n1\talice\tx\n2\tbob\ty\n"
	case "ltsv":
		return "id:1\tname:alice\tnote:x\nid:2\tname:bob\tnote:y\n"
	default:
		return "id,name,note\n1,alice,x\n2,bob,y\n"
	}
}

// writeBackValues are cell values a file format has to quote, escape or keep
// from being re-typed on the way back in.
var writeBackValues = []string{
	"", "007", "1.50", "1e3", "-0", "0x10", "9223372036854775808",
	"a,b", "a\tb", "line\nbreak", "a:b", " lead", "trail ", `"q"`, "ünï",
	"NULL", "true", "=1+1", "\\N",
}

type writeBackMachine struct {
	t      *testing.T
	s      *Shell
	source string
	outDir string
	saves  int
}

func (m *writeBackMachine) run(rt *rapid.T, line string) error {
	rt.Log(line)
	return m.s.exec(context.Background(), line)
}

func sqlLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func (m *writeBackMachine) actions() map[string]func(*rapid.T) {
	columns := []string{"name", "note"}
	return map[string]func(*rapid.T){
		"update": func(rt *rapid.T) {
			col := rapid.SampledFrom(columns).Draw(rt, "column")
			v := rapid.SampledFrom(writeBackValues).Draw(rt, "value")
			_ = m.run(rt, fmt.Sprintf("UPDATE people SET %s = %s WHERE rowid = (SELECT MIN(rowid) FROM people)", col, sqlLiteral(v)))
		},
		"setNull": func(rt *rapid.T) {
			col := rapid.SampledFrom(columns).Draw(rt, "column")
			_ = m.run(rt, fmt.Sprintf("UPDATE people SET %s = NULL WHERE rowid = (SELECT MAX(rowid) FROM people)", col))
		},
		"insert": func(rt *rapid.T) {
			v := rapid.SampledFrom(writeBackValues).Draw(rt, "value")
			_ = m.run(rt, fmt.Sprintf("INSERT INTO people (id, name, note) VALUES (9, %s, 'z')", sqlLiteral(v)))
		},
		"delete": func(rt *rapid.T) {
			_ = m.run(rt, "DELETE FROM people WHERE rowid = (SELECT MAX(rowid) FROM people)")
		},
		"saveInPlace": func(rt *rapid.T) {
			before, err := os.ReadFile(m.source)
			if err != nil {
				rt.Fatal(err)
			}
			if err := m.run(rt, ".save --in-place"); err != nil {
				m.checkRefusal(rt, err, m.source, before)
				return
			}
			m.checkFile(rt, m.source)
		},
		"saveDir": func(rt *rapid.T) {
			// .save DIR refuses to overwrite an earlier export, so each save
			// gets a directory of its own.
			m.saves++
			dir := filepath.Join(m.outDir, strconv.Itoa(m.saves))
			if err := m.run(rt, ".save "+dir); err != nil {
				m.checkRefusal(rt, err, filepath.Join(dir, filepath.Base(m.source)), nil)
				return
			}
			if _, err := os.Stat(filepath.Join(dir, filepath.Base(m.source))); err == nil {
				m.checkFile(rt, filepath.Join(dir, filepath.Base(m.source)))
			}
		},
	}
}

// checkRefusal accepts a save that refused what its format cannot hold (a tab or
// a newline in TSV or LTSV, an LTSV table with no rows) and requires that the
// refusal left the destination as it was: before is its previous content, or nil
// when there was none.
func (m *writeBackMachine) checkRefusal(rt *rapid.T, err error, dest string, before []byte) {
	if !strings.Contains(err.Error(), "cannot represent") && !strings.Contains(err.Error(), "cannot export a result with no rows") {
		rt.Fatalf("save failed: %v", err)
	}
	after, readErr := os.ReadFile(dest) //nolint:gosec // the path is the temp file under test
	if before == nil {
		if readErr == nil {
			rt.Fatalf("a refused save created %s:\n%q", dest, after)
		}
		return
	}
	if readErr != nil || !bytes.Equal(after, before) {
		rt.Fatalf("a refused save changed %s: before %q, after %q (%v)", dest, before, after, readErr)
	}
}

// checkFile loads path into a fresh session and compares its rows with the
// table the session under test holds. A CSV cell cannot tell NULL from an empty
// string, so both sides are compared by their printed value.
func (m *writeBackMachine) checkFile(rt *rapid.T, path string) {
	want, err := m.s.usecases.metadata.List(context.Background(), "people")
	if err != nil {
		rt.Fatalf("List: %v", err)
	}
	other, cleanup, err := newShell(m.t, []string{"sqly", path})
	if err != nil {
		rt.Fatal(err)
	}
	defer cleanup()
	if err := other.init(context.Background()); err != nil {
		rt.Fatalf("re-import %s: %v\n%s", path, err, readFileForLog(path))
	}
	got, err := other.usecases.metadata.List(context.Background(), "people")
	if err != nil {
		rt.Fatalf("List after re-import: %v", err)
	}
	if !slices.Equal(got.Header(), want.Header()) {
		rt.Fatalf("header after re-import = %v, saved %v", got.Header(), want.Header())
	}
	if got.RowCount() != want.RowCount() {
		rt.Fatalf("rows after re-import = %d, saved %d\n%s", got.RowCount(), want.RowCount(), readFileForLog(path))
	}
	for r := range want.RowCount() {
		for c := range want.ColumnCount() {
			if !sameCellValue(got.ValueAt(r, c), want.ValueAt(r, c)) {
				rt.Fatalf("row %d column %s after re-import = %q, saved %q\n%s",
					r, want.ColumnName(c), got.ValueAt(r, c), want.ValueAt(r, c), readFileForLog(path))
			}
		}
	}
}

// sameCellValue compares a saved value with the one read back. The file keeps
// the text as written, but a re-import infers a numeric column's type, so "1.50"
// read back from a column of numbers prints as 1.5. Two values that are the same
// number are the same value here; everything else must match exactly.
func sameCellValue(got, want string) bool {
	if got == want {
		return true
	}
	g, errG := strconv.ParseFloat(got, 64)
	w, errW := strconv.ParseFloat(want, 64)
	return errG == nil && errW == nil && g == w
}

func readFileForLog(path string) string {
	b, err := os.ReadFile(path) //nolint:gosec // the path is the temp file under test
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("file %s:\n%q", filepath.Base(path), b)
}
