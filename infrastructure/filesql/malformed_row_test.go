package filesql

import (
	"compress/gzip"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

func writeGzipFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // test writes a temporary fixture path
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(content)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// The malformed CSV used across the policy tests: row 3 is missing the zip field.
const raggedCSV = "id,name,zip\n1,alice,01234\n2,bob,00123\n3,caro\n4,dave,99999\n"

// newMalformedTestAdapter writes the ragged CSV to a file and returns an adapter
// bound to a fresh shared in-memory database. The pool is pinned to a single
// connection because a bare ":memory:" database is private per connection.
func newMalformedTestAdapter(t *testing.T) (*FileSQLAdapter, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.csv")
	if err := os.WriteFile(path, []byte(raggedCSV), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return NewFileSQLAdapter(db), path
}

func TestFileSQLAdapter_ImportMode_Stop(t *testing.T) {
	t.Parallel()
	adapter, path := newMalformedTestAdapter(t)
	adapter.SetMalformedRowPolicy(model.MalformedRowStop)

	err := adapter.LoadFile(context.Background(), path)
	if err == nil {
		t.Fatal("expected an error for a ragged row under the stop policy, got nil")
	}
	// The table must not be left behind as an empty stub.
	if _, qerr := adapter.Query(context.Background(), "SELECT * FROM malformed"); qerr == nil {
		t.Fatal("expected no table to be created under the stop policy")
	}
}

func TestFileSQLAdapter_ImportMode_Skip(t *testing.T) {
	t.Parallel()
	adapter, path := newMalformedTestAdapter(t)
	adapter.SetMalformedRowPolicy(model.MalformedRowSkip)

	if err := adapter.LoadFile(context.Background(), path); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	got, err := adapter.Query(context.Background(), "SELECT name FROM malformed ORDER BY id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	names := make([]string, 0, len(got.Records()))
	for _, r := range got.Records() {
		names = append(names, r[0])
	}
	want := []string{"alice", "bob", "dave"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
}

func TestFileSQLAdapter_ImportMode_Pad(t *testing.T) {
	t.Parallel()
	adapter, path := newMalformedTestAdapter(t)
	adapter.SetMalformedRowPolicy(model.MalformedRowPad)

	if err := adapter.LoadFile(context.Background(), path); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	// Every row is kept, and the missing zip of row 3 is an empty value.
	got, err := adapter.Query(context.Background(), "SELECT COALESCE(zip, '') FROM malformed ORDER BY id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got.Records()) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(got.Records()))
	}
	if got.Records()[2][0] != "" {
		t.Fatalf("row 3 zip = %q, want empty string", got.Records()[2][0])
	}
}

func TestFileSQLAdapter_ImportMode_PadRejectsLongRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "long.csv")
	if err := os.WriteFile(path, []byte("id,name\n1,alice,unexpected\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	adapter := NewFileSQLAdapter(db)
	adapter.SetMalformedRowPolicy(model.MalformedRowPad)

	if err := adapter.LoadFile(context.Background(), path); err == nil {
		t.Fatal("expected pad to reject a long row instead of truncating it")
	}
	if _, err := adapter.Query(context.Background(), "SELECT * FROM long"); err == nil {
		t.Fatal("expected no table to be created after a rejected long row")
	}
}

func TestFileSQLAdapter_ImportMode_PadStreamsGzipCSV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "quoted.csv.gz")
	writeGzipFile(t, path, "id,name,note\n1,alice,\"line one\nline two\"\n2,bob\n3,carol,ok\n")

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	adapter := NewFileSQLAdapter(db)
	adapter.SetMalformedRowPolicy(model.MalformedRowPad)

	if err := adapter.LoadFile(context.Background(), path); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	got, err := adapter.Query(context.Background(), "SELECT id, name, note FROM quoted ORDER BY id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	want := [][]string{{"1", "alice", "line one\nline two"}, {"2", "bob", ""}, {"3", "carol", "ok"}}
	if len(got.Records()) != len(want) {
		t.Fatalf("records = %#v, want %#v", got.Records(), want)
	}
	for i := range want {
		if got.Records()[i].Equal(model.Record(want[i])) == false {
			t.Errorf("record %d = %#v, want %#v", i, got.Records()[i], want[i])
		}
	}

	longPath := filepath.Join(dir, "long.csv.gz")
	writeGzipFile(t, longPath, "id,name\n1,alice,unexpected\n")
	if err := adapter.LoadFile(context.Background(), longPath); err == nil {
		t.Fatal("expected gzip pad preflight to reject a long row")
	}
}

func TestRejectLongDelimitedRowsHandlesDirectoriesAndCSVErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a dataset"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "ok.csv"), []byte("id,name\n1,alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rejectLongDelimitedRows([]string{dir}); err != nil {
		t.Fatalf("directory preflight: %v", err)
	}

	empty := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rejectLongDelimitedRows([]string{empty}); err != nil {
		t.Fatalf("empty CSV preflight: %v", err)
	}

	badHeader := filepath.Join(dir, "bad-header.csv")
	if err := os.WriteFile(badHeader, []byte("\"unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rejectLongDelimitedRows([]string{badHeader}); err == nil {
		t.Fatal("malformed CSV header returned nil error")
	}

	badRow := filepath.Join(dir, "bad-row.csv")
	if err := os.WriteFile(badRow, []byte("id,name\n1,\"unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rejectLongDelimitedRows([]string{badRow}); err == nil {
		t.Fatal("malformed CSV row returned nil error")
	}
	missing := filepath.Join(dir, "missing.csv")
	if _, err := delimitedInputFiles([]string{missing}); err == nil {
		t.Fatal("missing CSV returned nil error")
	}
	if err := rejectLongDelimitedRows([]string{missing}); err == nil {
		t.Fatal("rejectLongDelimitedRows missing CSV returned nil error")
	}
	if err := rejectLongDelimitedFile(missing); err == nil {
		t.Fatal("rejectLongDelimitedFile missing CSV returned nil error")
	}
}

func TestRejectLongDelimitedRowsHandlesTSV(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "short.tsv")
	if err := os.WriteFile(path, []byte("id\tname\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rejectLongDelimitedFile(path); err != nil {
		t.Fatalf("TSV preflight: %v", err)
	}
}

func TestFileSQLAdapter_ImportMode_PadPreflightsBeforeEmptyJSONTable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	emptyJSON := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(emptyJSON, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	longCSV := filepath.Join(dir, "long.csv")
	if err := os.WriteFile(longCSV, []byte("id,name\n1,alice,unexpected\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	adapter := NewFileSQLAdapter(db)
	adapter.SetMalformedRowPolicy(model.MalformedRowPad)

	if err := adapter.LoadFiles(context.Background(), emptyJSON, longCSV); err == nil {
		t.Fatal("expected pad to reject the mixed import")
	}
	if _, err := adapter.Query(context.Background(), "SELECT * FROM empty"); err == nil {
		t.Fatal("expected pad validation failure to leave no empty JSON table behind")
	}
}

func TestFileSQLAdapter_LoadFilesIsAtomicAcrossInputFormats(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		badName    string
		badContent string
	}{
		{name: "broken JSON", badName: "broken.json", badContent: `[{"id":`},
		{name: "invalid CSV", badName: "broken.csv", badContent: "id,name\n1,\"unterminated\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			emptyJSON := filepath.Join(dir, "existing.json")
			if err := os.WriteFile(emptyJSON, []byte("[]\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			bad := filepath.Join(dir, tc.badName)
			if err := os.WriteFile(bad, []byte(tc.badContent), 0o600); err != nil {
				t.Fatal(err)
			}

			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = db.Close() })
			ctx := context.Background()
			if _, err := db.ExecContext(ctx, `CREATE TABLE existing (value TEXT); INSERT INTO existing VALUES ('sentinel')`); err != nil {
				t.Fatal(err)
			}

			if err := NewFileSQLAdapter(db).LoadFiles(ctx, emptyJSON, bad); err == nil {
				t.Fatal("mixed import returned nil, want an error")
			}
			var value string
			if err := db.QueryRowContext(ctx, `SELECT value FROM existing`).Scan(&value); err != nil {
				t.Fatalf("pre-existing table was removed: %v", err)
			}
			if value != "sentinel" {
				t.Fatalf("pre-existing value = %q, want sentinel", value)
			}
			if _, err := db.ExecContext(ctx, `SELECT * FROM existing`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `SELECT * FROM broken`); err == nil {
				t.Fatal("broken input created a partial table")
			}
		})
	}
}

func TestFileSQLAdapter_LoadFilesPreservesInputOrderForLastWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	firstCSV := filepath.Join(dir, "first", "same.csv")
	secondCSV := filepath.Join(dir, "second", "same.csv")
	emptyJSON := filepath.Join(dir, "same.json")
	for path, content := range map[string]string{
		firstCSV:  "id\nfirst\n",
		secondCSV: "id\nsecond\n",
		emptyJSON: "[]\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	load := func(t *testing.T, paths ...string) (count int, value string, header string) {
		t.Helper()
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = db.Close() })
		ctx := context.Background()
		if err := NewFileSQLAdapter(db).LoadFiles(ctx, paths...); err != nil {
			t.Fatalf("LoadFiles: %v", err)
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "same"`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			if err := db.QueryRowContext(ctx, `SELECT id FROM "same"`).Scan(&value); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.QueryRowContext(ctx, `SELECT name FROM pragma_table_info('same')`).Scan(&header); err != nil {
			t.Fatal(err)
		}
		return count, value, header
	}

	if count, _, header := load(t, firstCSV, emptyJSON); count != 0 || header != "data" {
		t.Fatalf("CSV then empty JSON = count %d, header %q; want empty data table", count, header)
	}
	if count, value, _ := load(t, emptyJSON, firstCSV); count != 1 || value != "first" {
		t.Fatalf("empty JSON then CSV = count %d, value %q; want CSV row", count, value)
	}
	if count, value, _ := load(t, firstCSV, secondCSV); count != 1 || value != "second" {
		t.Fatalf("same-format last-wins = count %d, value %q; want second row", count, value)
	}
}

func TestFileSQLAdapter_LoadFilesRollsBackWhenLaterApplyFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	paths := map[string]string{
		"first.csv":   "id\n1\n",
		"same.csv":    "id\n2\n",
		"blocked.csv": "id\n3\n",
	}
	for name, content := range paths {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE same (id INTEGER);
		INSERT INTO same VALUES (999);
		CREATE VIEW blocked AS SELECT 999 AS id;
	`); err != nil {
		t.Fatal(err)
	}

	adapter := NewFileSQLAdapter(db)
	err = adapter.LoadFiles(ctx,
		filepath.Join(dir, "first.csv"),
		filepath.Join(dir, "same.csv"),
		filepath.Join(dir, "blocked.csv"),
	)
	if err == nil {
		t.Fatal("LoadFiles returned nil, want view collision error")
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='first'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("first table count = %d, want 0 after rollback", count)
	}
	var sameID int
	if err := db.QueryRowContext(ctx, `SELECT id FROM same`).Scan(&sameID); err != nil {
		t.Fatalf("same table was not restored: %v", err)
	}
	if sameID != 999 {
		t.Fatalf("same.id = %d, want original 999", sameID)
	}
	var blockedID int
	if err := db.QueryRowContext(ctx, `SELECT id FROM blocked`).Scan(&blockedID); err != nil {
		t.Fatalf("blocked view was not preserved: %v", err)
	}
	if blockedID != 999 {
		t.Fatalf("blocked.id = %d, want 999", blockedID)
	}
}

func TestFileSQLAdapter_LoadFilesEmptyJSONViewCollision(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	emptyJSON := filepath.Join(dir, "blocked.json")
	if err := os.WriteFile(emptyJSON, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE VIEW blocked AS SELECT 999 AS id`); err != nil {
		t.Fatal(err)
	}

	if err := NewFileSQLAdapter(db).LoadFiles(ctx, emptyJSON); err == nil {
		t.Fatal("LoadFiles returned nil, want empty-table/view collision error")
	}
	var id int
	if err := db.QueryRowContext(ctx, `SELECT id FROM blocked`).Scan(&id); err != nil {
		t.Fatalf("view was not preserved: %v", err)
	}
	if id != 999 {
		t.Fatalf("blocked.id = %d, want 999", id)
	}
}

func TestFileSQLAdapter_ImportMode_DefaultIsStop(t *testing.T) {
	t.Parallel()
	adapter, path := newMalformedTestAdapter(t)
	// No SetMalformedRowPolicy: the zero value must behave as stop.
	if adapter.MalformedRowPolicy() != model.MalformedRowStop {
		t.Fatalf("default policy = %v, want stop", adapter.MalformedRowPolicy())
	}
	if err := adapter.LoadFile(context.Background(), path); err == nil {
		t.Fatal("expected an error under the default (stop) policy")
	}
}
