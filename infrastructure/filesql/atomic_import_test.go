package filesql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	libfilesql "github.com/nao1215/filesql"
	infra "github.com/nao1215/sqly/infrastructure"
)

// fakeTx is a two-method stand-in for *sql.Tx that fails commit or rollback on
// demand. A real database cannot be made to fail a rollback on cue, so the only
// way to assert what happens when cleanup itself fails is to inject it.
//
// It follows *sql.Tx's state machine: the first Commit or Rollback ends the
// transaction, and any later call answers sql.ErrTxDone whatever the first one
// returned. Keeping that rule here is what makes this fake able to fail a test
// that rolls back after a commit — the earlier version returned its configured
// error forever, so such code looked correct against it.
type fakeTx struct {
	commitErr   error
	rollbackErr error
	commits     int
	rollbacks   int
	done        bool
}

func (tx *fakeTx) Commit() error {
	tx.commits++
	if tx.done {
		return sql.ErrTxDone
	}
	tx.done = true
	return tx.commitErr
}

func (tx *fakeTx) Rollback() error {
	tx.rollbacks++
	if tx.done {
		return sql.ErrTxDone
	}
	tx.done = true
	return tx.rollbackErr
}

// fakeBeginner hands out one fakeTx, or fails to begin at all.
type fakeBeginner struct {
	tx       *fakeTx
	beginErr error
	begins   int
}

func (b *fakeBeginner) BeginTx(_ context.Context, _ *sql.TxOptions) (*fakeTx, error) {
	b.begins++
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	return b.tx, nil
}

// spyPublisher records whether the registry it stands for was ever published.
// Asserting on this flag — rather than on a call count — is what makes "the
// database rolled back but the registry kept the entry" a detectable state.
type spyPublisher struct {
	name      string
	published bool
	order     *[]string
}

func (p *spyPublisher) PublishRegistries() {
	p.published = true
	if p.order != nil {
		*p.order = append(*p.order, p.name)
	}
}

// TestAtomicImportFailureMatrix drives every failure the import transaction can
// hit, including the two that only appear in combination, and asserts on both
// halves of the outcome: which errors the caller can still see, and whether any
// registry became visible to the rest of the process.
func TestAtomicImportFailureMatrix(t *testing.T) {
	t.Parallel()

	errBegin := errors.New("begin refused")
	errStage := errors.New("malformed input")
	errCommit := errors.New("disk full")
	errRollback := errors.New("connection lost")

	tests := []struct {
		name string
		// injected failures
		beginErr    error
		stageErrAt  int // 1-based index of the path whose staging fails; 0 = none
		commitErr   error
		rollbackErr error
		// expectations
		wantErrs []error // every error the caller must still be able to see
		// wantNotErrs are errors the result must NOT carry, which is how a case
		// pins the absence of a manufactured cleanup error.
		wantNotErrs   []error
		wantPublished bool
		wantCommits   int
		wantRollbacks int
	}{
		{
			name:          "success publishes every staged registry",
			wantPublished: true,
			wantCommits:   1,
			wantRollbacks: 0,
		},
		{
			name:     "begin failure never stages or publishes",
			beginErr: errBegin,
			wantErrs: []error{errBegin},
		},
		{
			name:          "first input fails",
			stageErrAt:    1,
			wantErrs:      []error{errStage},
			wantRollbacks: 1,
		},
		{
			name:          "last input fails, so the earlier ones roll back too",
			stageErrAt:    3,
			wantErrs:      []error{errStage},
			wantRollbacks: 1,
		},
		{
			// A commit that failed is still a commit: the transaction is over,
			// so no rollback is attempted and the caller is told about the
			// commit and nothing else.
			name:          "commit failure leaves the registry unpublished",
			commitErr:     errCommit,
			wantErrs:      []error{errCommit},
			wantNotErrs:   []error{infra.ErrRollback, sql.ErrTxDone},
			wantCommits:   1,
			wantRollbacks: 0,
		},
		{
			name:          "rollback failure alone still surfaces",
			stageErrAt:    0,
			rollbackErr:   errRollback,
			wantPublished: true, // commit succeeded, so rollback is never attempted
			wantCommits:   1,
			wantRollbacks: 0,
		},
		{
			name:          "import failure and rollback failure are both reported",
			stageErrAt:    2,
			rollbackErr:   errRollback,
			wantErrs:      []error{errStage, errRollback, infra.ErrRollback},
			wantRollbacks: 1,
		},
		{
			// The rollback failure is configured but unreachable, because no
			// rollback follows a commit. The commit failure must not acquire a
			// second, invented cause.
			name:          "a rollback failure cannot attach itself to a commit failure",
			commitErr:     errCommit,
			rollbackErr:   errRollback,
			wantErrs:      []error{errCommit},
			wantNotErrs:   []error{errRollback, infra.ErrRollback, sql.ErrTxDone},
			wantCommits:   1,
			wantRollbacks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tx := &fakeTx{commitErr: tt.commitErr, rollbackErr: tt.rollbackErr}
			beginner := &fakeBeginner{tx: tx, beginErr: tt.beginErr}

			paths := []string{"a.csv", "b.csv", "c.csv"}
			var publishOrder []string
			publishers := make([]*spyPublisher, len(paths))
			for i, p := range paths {
				publishers[i] = &spyPublisher{name: p, order: &publishOrder}
			}

			staged := 0
			importer := atomicImport[*fakeTx]{
				beginner: beginner,
				stage: func(_ context.Context, _ *fakeTx, path string) (registryPublisher, error) {
					staged++
					if tt.stageErrAt == staged {
						return nil, errStage
					}
					for i, p := range paths {
						if p == path {
							return publishers[i], nil
						}
					}
					t.Fatalf("unexpected path %q", path)
					return nil, nil
				},
			}

			err := importer.run(t.Context(), paths)

			if len(tt.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("run() = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Fatalf("run() = nil, want an error wrapping %v", tt.wantErrs)
				}
				for _, want := range tt.wantErrs {
					if !errors.Is(err, want) {
						t.Errorf("errors.Is(err, %v) = false; err = %v", want, err)
					}
				}
			}
			for _, unwanted := range tt.wantNotErrs {
				if errors.Is(err, unwanted) {
					t.Errorf("errors.Is(err, %v) = true, want false; err = %v", unwanted, err)
				}
			}

			for _, p := range publishers {
				if p.published != tt.wantPublished {
					t.Errorf("registry %q published = %v, want %v (err = %v)", p.name, p.published, tt.wantPublished, err)
				}
			}
			if tx.commits != tt.wantCommits {
				t.Errorf("commits = %d, want %d", tx.commits, tt.wantCommits)
			}
			if tx.rollbacks != tt.wantRollbacks {
				t.Errorf("rollbacks = %d, want %d", tx.rollbacks, tt.wantRollbacks)
			}
			if tt.wantPublished {
				// Publication replays staging order, so the last file to claim a
				// base name is the one write-back resolves to.
				if strings.Join(publishOrder, ",") != strings.Join(paths, ",") {
					t.Errorf("publish order = %v, want %v", publishOrder, paths)
				}
			}
		})
	}
}

// TestAtomicImportNeverRollsBackAfterCommit pins the state machine: a committed
// transaction must not be rolled back afterwards. Rolling back a committed
// transaction returns sql.ErrTxDone, and code that unconditionally ignores that
// error cannot tell this bug from a benign no-op.
func TestAtomicImportNeverRollsBackAfterCommit(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{rollbackErr: sql.ErrTxDone}
	importer := atomicImport[*fakeTx]{
		beginner: &fakeBeginner{tx: tx},
		stage: func(_ context.Context, _ *fakeTx, _ string) (registryPublisher, error) {
			return &spyPublisher{}, nil
		},
	}
	if err := importer.run(t.Context(), []string{"a.csv"}); err != nil {
		t.Fatalf("run() = %v, want nil", err)
	}
	if tx.rollbacks != 0 {
		t.Errorf("rollbacks after a successful commit = %d, want 0", tx.rollbacks)
	}
}

// TestAtomicImportReportsTxDoneRollback checks the other half of that rule: when
// a rollback is genuinely needed and reports ErrTxDone, something else ended the
// transaction. That is a defect, so it is reported rather than swallowed.
func TestAtomicImportReportsTxDoneRollback(t *testing.T) {
	t.Parallel()

	stageErr := errors.New("bad input")
	tx := &fakeTx{rollbackErr: sql.ErrTxDone}
	importer := atomicImport[*fakeTx]{
		beginner: &fakeBeginner{tx: tx},
		stage: func(_ context.Context, _ *fakeTx, _ string) (registryPublisher, error) {
			return nil, stageErr
		},
	}
	err := importer.run(t.Context(), []string{"a.csv"})
	if !errors.Is(err, stageErr) {
		t.Errorf("errors.Is(err, stageErr) = false; err = %v", err)
	}
	if !errors.Is(err, sql.ErrTxDone) {
		t.Errorf("errors.Is(err, sql.ErrTxDone) = false; err = %v", err)
	}
}

// TestLoadFilesRollsBackEveryEarlierInput is the same guarantee through the real
// adapter and a real database: the third input is broken, so the two tables the
// first two inputs created must not exist afterwards, and a table that existed
// before the import must survive untouched.
func TestLoadFilesRollsBackEveryEarlierInput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	good1 := filepath.Join(dir, "first.csv")
	good2 := filepath.Join(dir, "second.csv")
	broken := filepath.Join(dir, "third.csv")
	if err := os.WriteFile(good1, []byte("id,name\n1,a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good2, []byte("id,name\n2,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A row wider than the header, which the default stop policy rejects.
	if err := os.WriteFile(broken, []byte("id,name\n3,c,extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(t.Context(), `CREATE TABLE preexisting (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO preexisting VALUES ('kept')`); err != nil {
		t.Fatal(err)
	}

	adapter := NewFileSQLAdapter(db)
	if err := adapter.LoadFiles(t.Context(), good1, good2, broken); err == nil {
		t.Fatal("LoadFiles with a broken last input = nil error, want an error")
	}

	names := tableNames(t, db)
	for _, gone := range []string{"first", "second", "third"} {
		if names[gone] {
			t.Errorf("table %q survived a rolled-back import; tables = %v", gone, names)
		}
	}
	if !names["preexisting"] {
		t.Errorf("rollback removed a table that existed before the import; tables = %v", names)
	}
	var kept string
	if err := db.QueryRowContext(t.Context(), `SELECT v FROM preexisting`).Scan(&kept); err != nil {
		t.Fatalf("preexisting table unreadable after rollback: %v", err)
	}
	if kept != "kept" {
		t.Errorf("preexisting row = %q, want %q", kept, "kept")
	}

	// The failed import must not poison the session: importing the good inputs
	// afterwards has to work.
	if err := adapter.LoadFiles(t.Context(), good1, good2); err != nil {
		t.Fatalf("re-import after a rolled-back import failed: %v", err)
	}
	names = tableNames(t, db)
	if !names["first"] || !names["second"] {
		t.Errorf("re-import did not create the tables; tables = %v", names)
	}
}

// TestLoadFilesRollbackLeavesNoACHRegistryEntry is the integration form of the
// rule that the fake-based matrix states in the abstract: an ACH import that is
// rolled back must not leave write-back metadata behind. Registering it before
// the commit produced the one state the design forbids — the DB has no tables,
// but DumpACHFile still believes it can reconstruct the file from them.
//
// It inspects filesql's process-global registry, so it must not run in parallel
// with the other ACH tests.
func TestLoadFilesRollbackLeavesNoACHRegistryEntry(t *testing.T) {
	achFile := filepath.Join("..", "..", "testdata", "ppd-debit.ach")
	if _, err := os.Stat(achFile); os.IsNotExist(err) {
		t.Skip("ACH test data not available")
	}
	libfilesql.UnregisterACHTableSet("ppd_debit")
	t.Cleanup(func() { libfilesql.UnregisterACHTableSet("ppd_debit") })

	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.csv")
	if err := os.WriteFile(broken, []byte("id,name\n1,a,extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	adapter := NewFileSQLAdapter(db)
	// The ACH file imports cleanly; the CSV after it does not, so the whole
	// import rolls back.
	if err := adapter.LoadFiles(t.Context(), achFile, broken); err == nil {
		t.Fatal("LoadFiles = nil error, want the broken CSV to fail the import")
	}

	for _, info := range libfilesql.GetACHTableInfos() {
		if info.BaseName == "ppd_debit" {
			t.Fatalf("ACH registry kept ppd_debit after the import rolled back: %+v", info)
		}
	}
	if names := tableNames(t, db); len(names) != 0 {
		t.Errorf("tables after a rolled-back ACH import = %v, want none", names)
	}

	// The registry becomes valid only once an import actually commits.
	if err := adapter.LoadFiles(t.Context(), achFile); err != nil {
		t.Fatalf("LoadFiles(ach) = %v, want success", err)
	}
	found := false
	for _, info := range libfilesql.GetACHTableInfos() {
		if info.BaseName == "ppd_debit" {
			found = true
		}
	}
	if !found {
		t.Error("ACH registry is empty after a committed import")
	}
}

// tableNames returns the set of user tables in the database.
func tableNames(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.QueryContext(t.Context(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
