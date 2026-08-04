package di

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/shell"
)

func TestMain(m *testing.M) {
	// The composition root opens real SQLite databases, so the driver has to be
	// registered exactly as main() registers it.
	config.InitSQLite3()
	os.Exit(m.Run())
}

// TestNewShell_BuildsAShellAndACleanup is the plain case: a valid command line
// produces a shell and the function that releases it, and nothing else.
func TestNewShell_BuildsAShellAndACleanup(t *testing.T) {
	historyPath := isolate(t)

	sqlyShell, cleanup, err := NewShell([]string{"sqly", "--version"})
	if err != nil {
		t.Fatalf("NewShell: %v", err)
	}
	if sqlyShell == nil {
		t.Fatal("NewShell returned a nil shell with a nil error")
	}
	if cleanup == nil {
		t.Fatal("NewShell returned a nil cleanup; the caller has no way to close the databases")
	}
	cleanup()

	if _, err := os.Stat(filepath.Dir(historyPath)); err != nil {
		t.Fatalf("the isolated config directory went missing: %v", err)
	}
}

// TestNewShell_RejectsABadCommandLine checks the argument parse is not
// swallowed. A composition root that ignored it would build a shell from a
// zero-valued Arg and fail much later, with a message about something else.
func TestNewShell_RejectsABadCommandLine(t *testing.T) {
	isolate(t)

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"sqly", "--no-such-flag"}},
		{name: "no argv at all", args: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sqlyShell, cleanup, err := NewShell(tt.args)
			if err == nil {
				if cleanup != nil {
					cleanup()
				}
				t.Fatalf("NewShell(%q) succeeded; a command line sqly cannot parse must not build a shell", tt.args)
			}
			if sqlyShell != nil || cleanup != nil {
				t.Errorf("NewShell(%q) returned shell=%v cleanup!=nil=%v alongside an error; a failure returns neither",
					tt.args, sqlyShell != nil, cleanup != nil)
			}
		})
	}

	// An unknown flag is a bad invocation rather than a startup failure, and
	// main.go prints and exits differently for the two.
	_, _, err := NewShell([]string{"sqly", "--no-such-flag"})
	var argErr *config.ArgError
	if !errors.As(err, &argErr) {
		t.Errorf("error %v is not a *config.ArgError; main.go would prefix it as a startup failure", err)
	}
}

// TestNewShell_ImportsIntoTheDatabaseItQueries is the one assertion that a
// diagram cannot replace: filesql loads a file into the database the query then
// runs against. Give the adapter a database of its own and the import still
// "succeeds" while the query cannot find the table.
func TestNewShell_ImportsIntoTheDatabaseItQueries(t *testing.T) {
	isolate(t)

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "user.csv")
	if err := os.WriteFile(csvPath, []byte("id,name\n1,gopher\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t)

	sqlyShell, cleanup, err := NewShell([]string{"sqly", "--sql", "SELECT name FROM user", csvPath})
	if err != nil {
		t.Fatalf("NewShell: %v", err)
	}
	defer cleanup()

	if err := sqlyShell.Run(context.Background()); err != nil {
		t.Fatalf("run a batch query over an imported file: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "gopher") {
		t.Errorf("query output %q does not contain the imported row; the importer and the query are not looking at the same database", got)
	}
}

// TestNewShell_StartsTheInspectAndBatchEntryPoints covers the other two ways in.
// They are separate paths through Shell.Run, and a miswired usecase shows up as
// a nil dereference rather than as an error.
func TestNewShell_StartsTheInspectAndBatchEntryPoints(t *testing.T) {
	isolate(t)

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "user.csv")
	if err := os.WriteFile(csvPath, []byte("id,name\n1,gopher\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "inspect", args: []string{"sqly", "--inspect", csvPath}, want: "user"},
		{name: "batch query", args: []string{"sqly", "--sql", "SELECT COUNT(*) AS n FROM user", csvPath}, want: "1"},
		{name: "help", args: []string{"sqly", "--help"}, want: "sqly"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout := captureStdout(t)

			sqlyShell, cleanup, err := NewShell(tt.args)
			if err != nil {
				t.Fatalf("NewShell(%q): %v", tt.args, err)
			}
			defer cleanup()

			if err := sqlyShell.Run(context.Background()); err != nil {
				t.Fatalf("run %q: %v", tt.args, err)
			}
			if got := stdout.String(); !strings.Contains(got, tt.want) {
				t.Errorf("output of %q = %q, want it to contain %q", tt.args, got, tt.want)
			}
		})
	}
}

// TestNewShell_HistoryAndSessionAreSeparateDatabases pins the split. History is
// a file that outlives the session; the tables live in memory and do not. One
// database for both would put every imported table in the user's config
// directory, and a re-run would see the previous run's tables.
func TestNewShell_HistoryAndSessionAreSeparateDatabases(t *testing.T) {
	historyPath := isolate(t)

	// Record the database each constructor was handed, so the two can be
	// compared by identity rather than by what they happen to contain.
	origInMem, origHistory := newInMemDB, newHistoryDB
	t.Cleanup(func() { newInMemDB, newHistoryDB = origInMem, origHistory })

	var session, history *sql.DB
	var configuredPath string
	newInMemDB = func() (config.MemoryDB, func(), error) {
		db, release, err := origInMem()
		session = (*sql.DB)(db)
		return db, release, err
	}
	newHistoryDB = func(c *config.Config) (config.HistoryDB, func(), error) {
		configuredPath = c.HistoryDBPath
		db, release, err := origHistory(c)
		history = (*sql.DB)(db)
		return db, release, err
	}

	sqlyShell, cleanup, err := NewShell([]string{"sqly", "--version"})
	if err != nil {
		t.Fatalf("NewShell: %v", err)
	}
	defer cleanup()
	if sqlyShell == nil {
		t.Fatal("NewShell returned a nil shell")
	}

	if session == nil || history == nil {
		t.Fatal("one of the two databases was never opened")
	}
	if session == history {
		t.Error("history and the session are the same database; every imported table would be written to the user's config directory and outlive the run")
	}
	if configuredPath != historyPath {
		t.Errorf("history database path = %q, want the configured %q", configuredPath, historyPath)
	}
}

// TestNewShell_ReleasesWhatItOpenedWhenTheHistoryDatabaseFails is the first
// failure path. Only the session database is open at that point, and it has to
// be closed before the error is returned.
func TestNewShell_ReleasesWhatItOpenedWhenTheHistoryDatabaseFails(t *testing.T) {
	isolate(t)

	released := recordReleases(t)
	wantErr := errors.New("history database refused to open")
	newHistoryDB = func(*config.Config) (config.HistoryDB, func(), error) {
		return nil, nil, wantErr
	}

	sqlyShell, cleanup, err := NewShell([]string{"sqly", "--version"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if sqlyShell != nil || cleanup != nil {
		t.Fatalf("a failed NewShell returned shell!=nil=%v cleanup!=nil=%v; the caller has nothing to run", sqlyShell != nil, cleanup != nil)
	}
	if got := released.names(); len(got) != 1 || got[0] != "memory" {
		t.Errorf("released %v, want exactly [memory]; the session database was open and is now unreachable", got)
	}
}

// TestNewShell_ReleasesBothDatabasesWhenTheShellFails is the second failure
// path, and the one where forgetting a single line leaks the history file
// handle for the life of the process.
func TestNewShell_ReleasesBothDatabasesWhenTheShellFails(t *testing.T) {
	isolate(t)

	released := recordReleases(t)
	wantErr := errors.New("shell refused to start")
	newSqlyShell = func(*config.Arg, *config.Config, shell.CommandList, shell.Usecases) (*shell.Shell, error) {
		return nil, wantErr
	}

	sqlyShell, cleanup, err := NewShell([]string{"sqly", "--version"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if sqlyShell != nil || cleanup != nil {
		t.Fatalf("a failed NewShell returned shell!=nil=%v cleanup!=nil=%v", sqlyShell != nil, cleanup != nil)
	}
	if got := released.names(); len(got) != 2 || got[0] != "history" || got[1] != "memory" {
		t.Errorf("released %v, want [history memory]: both databases, closed in the reverse of the order they were opened", got)
	}
}

// TestNewShell_NothingIsOpenWhenTheSessionDatabaseFails is the earliest failure.
// There is nothing to release yet, and a cleanup call here would run against a
// resource that was never created.
func TestNewShell_NothingIsOpenWhenTheSessionDatabaseFails(t *testing.T) {
	isolate(t)

	released := recordReleases(t)
	wantErr := errors.New("session database refused to open")
	newInMemDB = func() (config.MemoryDB, func(), error) {
		return nil, nil, wantErr
	}

	if _, _, err := NewShell([]string{"sqly", "--version"}); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if got := released.names(); len(got) != 0 {
		t.Errorf("released %v, want nothing; no database had been opened", got)
	}
}

// TestNewShell_SuccessCleanupReleasesEverythingOnce is the success contract.
// Both databases close, in the reverse of the order they were opened, and a
// second call to the same cleanup is not how they get closed twice.
func TestNewShell_SuccessCleanupReleasesEverythingOnce(t *testing.T) {
	isolate(t)

	released := recordReleases(t)

	_, cleanup, err := NewShell([]string{"sqly", "--version"})
	if err != nil {
		t.Fatalf("NewShell: %v", err)
	}
	if got := released.names(); len(got) != 0 {
		t.Fatalf("released %v before the caller asked; a successful NewShell releases nothing", got)
	}

	cleanup()
	if got := released.names(); len(got) != 2 || got[0] != "history" || got[1] != "memory" {
		t.Fatalf("released %v, want [history memory]", got)
	}
}

// releaseLog records which resources were released and in what order.
type releaseLog struct {
	released []string
}

func (l *releaseLog) names() []string { return l.released }

func (l *releaseLog) record(name string, release func()) func() {
	return func() {
		l.released = append(l.released, name)
		release()
	}
}

// recordReleases wraps the two database constructors so their cleanups report
// themselves, and restores the originals when the test ends. The real
// constructors still run, so the shell under test is the real one.
func recordReleases(t *testing.T) *releaseLog {
	t.Helper()

	origInMem, origHistory, origShell := newInMemDB, newHistoryDB, newSqlyShell
	t.Cleanup(func() {
		newInMemDB, newHistoryDB, newSqlyShell = origInMem, origHistory, origShell
	})

	log := &releaseLog{}
	newInMemDB = func() (config.MemoryDB, func(), error) {
		db, release, err := origInMem()
		if err != nil {
			return nil, nil, err
		}
		return db, log.record("memory", release), nil
	}
	newHistoryDB = func(c *config.Config) (config.HistoryDB, func(), error) {
		db, release, err := origHistory(c)
		if err != nil {
			return nil, nil, err
		}
		return db, log.record("history", release), nil
	}
	return log
}

// isolate points the history database and the config directory at a temp
// directory, so a test never touches the developer's real sqly config, and
// returns the history path it configured.
func isolate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	historyPath := filepath.Join(dir, "history.db")
	t.Setenv("SQLY_HISTORY_DB_PATH", historyPath)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	return historyPath
}

// captureStdout redirects the writer the shell prints through for one test.
func captureStdout(t *testing.T) *bytes.Buffer {
	t.Helper()

	orig := config.Stdout
	buf := &bytes.Buffer{}
	config.Stdout = buf
	t.Cleanup(func() { config.Stdout = orig })
	return buf
}
