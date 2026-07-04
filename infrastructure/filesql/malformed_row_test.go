package filesql

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

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

func TestFileSQLAdapter_ImportMode_Fill(t *testing.T) {
	t.Parallel()
	adapter, path := newMalformedTestAdapter(t)
	adapter.SetMalformedRowPolicy(model.MalformedRowFill)

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
