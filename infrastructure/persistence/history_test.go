package persistence

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/quick"

	"github.com/nao1215/sqly/domain/model"
)

// newTestHistoryRepo returns a repository over a file in a fresh temp dir,
// already initialized.
func newTestHistoryRepo(t *testing.T) *historyRepository {
	t.Helper()
	repo, ok := NewHistoryRepository(filepath.Join(t.TempDir(), "history")).(*historyRepository)
	if !ok {
		t.Fatal("NewHistoryRepository did not return *historyRepository")
	}
	if err := repo.Init(context.Background()); err != nil {
		t.Fatalf("Init = %v, want nil", err)
	}
	return repo
}

// TestHistoryLineCodecRoundTrips is the property the file format rests on: an
// entry survives being written on one line and read back. A statement typed
// across several lines is the case that makes it a property rather than a
// constant, since the newline is what the line format cannot carry literally.
func TestHistoryLineCodecRoundTrips(t *testing.T) {
	t.Parallel()

	roundTrip := func(request string) bool {
		if strings.TrimSpace(request) == "" {
			// A blank entry is not stored at all, which List reports by skipping it.
			return encodeHistoryLine(request) == ""
		}
		line := encodeHistoryLine(request)
		if strings.ContainsAny(line, "\n\r") {
			return false // the encoded form must be one line
		}
		got, ok := decodeHistoryLine(line)
		return ok && got == request
	}

	// A fixed seed so a failure is reproducible.
	cfg := &quick.Config{MaxCount: 500, Rand: rand.New(rand.NewSource(1))} //#nosec G404 -- test input generation
	if err := quick.Check(roundTrip, cfg); err != nil {
		t.Errorf("round trip property failed: %v", err)
	}

	// The cases a random string is unlikely to produce.
	for _, request := range []string{
		"SELECT 1",
		"SELECT *\nFROM user\nWHERE id = 1;",
		`SELECT '\n' AS literal_backslash_n`,
		"SELECT 'tab\there'",
		"SELECT '日本語'",
		`C:\Users\nao\data.csv`,
		"SELECT 1 -- trailing backslash \\",
		strings.Repeat("SELECT 1; ", 5000),
	} {
		line := encodeHistoryLine(request)
		if strings.ContainsAny(line, "\n\r") {
			t.Errorf("encodeHistoryLine(%q) spans several lines", request)
			continue
		}
		got, ok := decodeHistoryLine(line)
		if !ok || got != request {
			t.Errorf("round trip of %q = (%q, %v), want (%q, true)", request, got, ok, request)
		}
	}
}

// TestHistoryAppendAndList checks the whole path a session takes: entries go in
// as they are typed and come back in that order.
func TestHistoryAppendAndList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newTestHistoryRepo(t)

	want := []string{"SELECT 1", "SELECT *\nFROM user", "SELECT '日本語'"}
	for _, request := range want {
		if err := repo.Append(ctx, model.NewHistory(0, request)); err != nil {
			t.Fatalf("Append(%q) = %v, want nil", request, err)
		}
	}
	// A blank entry is not history; it must not take a line.
	if err := repo.Append(ctx, model.NewHistory(0, "   ")); err != nil {
		t.Fatalf("Append(blank) = %v, want nil", err)
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List = %v, want nil", err)
	}
	if len(got) != len(want) {
		t.Fatalf("List returned %d entries, want %d: %v", len(got), len(want), got.ToStringList())
	}
	for i, request := range want {
		if got[i].Request != request {
			t.Errorf("entry %d = %q, want %q", i, got[i].Request, request)
		}
		if got[i].ID != i+1 {
			t.Errorf("entry %d has id %d, want %d (order is the identity)", i, got[i].ID, i+1)
		}
	}
}

// TestHistoryListOnAMissingFile pins that a session that has never written
// history starts with an empty one rather than an error.
func TestHistoryListOnAMissingFile(t *testing.T) {
	t.Parallel()

	repo, ok := NewHistoryRepository(filepath.Join(t.TempDir(), "absent")).(*historyRepository)
	if !ok {
		t.Fatal("NewHistoryRepository did not return *historyRepository")
	}
	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List on a missing file = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("List on a missing file returned %d entries, want 0", len(got))
	}
}

// TestHistorySkipsALineItCannotRead checks that a damaged line costs that entry
// and not the session's history. A process killed mid-append, or a file someone
// edited, is what produces one.
func TestHistorySkipsALineItCannotRead(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history")
	// A blank line and a whitespace-only line are the shapes a torn write leaves.
	body := "SELECT 1\n\n   \nSELECT 2\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, ok := NewHistoryRepository(path).(*historyRepository)
	if !ok {
		t.Fatal("NewHistoryRepository did not return *historyRepository")
	}

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List = %v, want nil", err)
	}
	want := []string{"SELECT 1", "SELECT 2"}
	if len(got) != len(want) {
		t.Fatalf("List returned %v, want %v", got.ToStringList(), want)
	}
	for i := range want {
		if got[i].Request != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i].Request, want[i])
		}
	}
}

// TestHistoryTrimsToTheCap checks that a file grown past the cap is cut down to
// the newest entries, and that the trim is written back rather than only
// applied in memory.
func TestHistoryTrimsToTheCap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history")

	var b strings.Builder
	const over = maxHistoryEntries + 10
	for i := range over {
		b.WriteString(encodeHistoryLine("SELECT " + strconv.Itoa(i)))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	repo, ok := NewHistoryRepository(path).(*historyRepository)
	if !ok {
		t.Fatal("NewHistoryRepository did not return *historyRepository")
	}
	if err := repo.Init(ctx); err != nil {
		t.Fatal(err)
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List = %v, want nil", err)
	}
	if len(got) != maxHistoryEntries {
		t.Fatalf("List returned %d entries, want %d", len(got), maxHistoryEntries)
	}
	// The newest survive: the first kept entry is the one at the cut.
	if want := "SELECT " + strconv.Itoa(over-maxHistoryEntries); got[0].Request != want {
		t.Errorf("oldest kept entry = %q, want %q", got[0].Request, want)
	}
	if want := "SELECT " + strconv.Itoa(over-1); got[len(got)-1].Request != want {
		t.Errorf("newest entry = %q, want %q", got[len(got)-1].Request, want)
	}

	// The file itself was trimmed, so the next session does not re-read the cut.
	data, err := os.ReadFile(path) //#nosec G304 -- test path under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(data), "\n"); lines != maxHistoryEntries {
		t.Errorf("file holds %d lines after the trim, want %d", lines, maxHistoryEntries)
	}
}

// TestHistoryConcurrentAppendsKeepEveryLineReadable is why the format is one
// line per entry: O_APPEND makes a single write atomic, so two sessions sharing
// a history file interleave whole lines rather than halves of them.
func TestHistoryConcurrentAppendsKeepEveryLineReadable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history")

	const writers = 8
	const perWriter = 50
	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			repo, ok := NewHistoryRepository(path).(*historyRepository)
			if !ok {
				return
			}
			if err := repo.Init(ctx); err != nil {
				return
			}
			for i := range perWriter {
				// A multi-line statement, so a torn write would be visible as a
				// line that does not decode to one of these.
				_ = repo.Append(ctx, model.NewHistory(0, "SELECT "+strconv.Itoa(w)+"\nFROM t"+strconv.Itoa(i)))
			}
		}(w)
	}
	wg.Wait()

	repo, ok := NewHistoryRepository(path).(*historyRepository)
	if !ok {
		t.Fatal("NewHistoryRepository did not return *historyRepository")
	}
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List = %v, want nil", err)
	}
	if len(got) != writers*perWriter {
		t.Errorf("List returned %d entries, want %d", len(got), writers*perWriter)
	}
	for _, h := range got {
		if !strings.HasPrefix(h.Request, "SELECT ") || !strings.Contains(h.Request, "\nFROM t") {
			t.Fatalf("a concurrent append produced a torn entry: %q", h.Request)
		}
	}
}

// TestHistoryUnwritablePathFailsInit checks that an unwritable location is
// reported at startup, which is what lets the shell warn once and carry on.
func TestHistoryUnwritablePathFailsInit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A directory, not a file: 0500 is what makes it unwritable while still
	// letting the test reach into it.
	if err := os.Chmod(dir, 0o500); err != nil { //#nosec G302 -- directory mode
		t.Skipf("cannot make the directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //#nosec G302 -- directory mode, restored so t.TempDir can clean up

	repo := NewHistoryRepository(filepath.Join(dir, "history"))
	if err := repo.Init(context.Background()); err == nil {
		t.Skip("the filesystem allowed the write (running as root?)")
	}
	// Append must refuse rather than half-write once Init has failed.
	if err := repo.Append(context.Background(), model.NewHistory(0, "SELECT 1")); err == nil {
		t.Error("Append after a failed Init = nil error, want error")
	}
}

// TestHistoryEmptyPathFailsInit pins the guard against a repository built with
// no path, which would otherwise create a file named after the empty string.
func TestHistoryEmptyPathFailsInit(t *testing.T) {
	t.Parallel()

	if err := NewHistoryRepository("").Init(context.Background()); err == nil {
		t.Error("Init with an empty path = nil error, want error")
	}
}
