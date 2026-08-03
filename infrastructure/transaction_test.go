package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// stubTx records how the transaction was ended and fails either ending on cue.
//
// It reproduces the semantics of *sql.Tx rather than a convenient
// simplification: once Commit or Rollback has been called the transaction is
// terminal, and every later call answers sql.ErrTxDone regardless of what the
// first one returned. A fake that returned a configured error forever hid a
// real defect — code that rolled back after a failed commit looked correct
// against the fake and manufactured a spurious ErrTxDone against a real
// database.
type stubTx struct {
	commitErr   error
	rollbackErr error
	ended       []string
	done        bool
}

func (tx *stubTx) Commit() error {
	tx.ended = append(tx.ended, "commit")
	if tx.done {
		return sql.ErrTxDone
	}
	tx.done = true
	return tx.commitErr
}

func (tx *stubTx) Rollback() error {
	tx.ended = append(tx.ended, "rollback")
	if tx.done {
		return sql.ErrTxDone
	}
	tx.done = true
	return tx.rollbackErr
}

type stubBeginner struct {
	tx       *stubTx
	beginErr error
}

func (b stubBeginner) BeginTx(_ context.Context, _ *sql.TxOptions) (*stubTx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	return b.tx, nil
}

// TestWithTransactionOutcomes is the table that fixes the deferred named-return
// rewrite. Each case asserts three things at once: the errors the caller can
// still reach, whether the work counted as committed, and how the transaction
// was ended — because a cleanup rule that gets any one of those wrong is a rule
// that silently loses either the cause or the cleanup failure.
func TestWithTransactionOutcomes(t *testing.T) {
	t.Parallel()

	errBegin := errors.New("begin failed")
	errWork := errors.New("work failed")
	errCommit := errors.New("commit failed")
	errRollback := errors.New("rollback failed")

	tests := []struct {
		name          string
		beginErr      error
		workErr       error
		commitErr     error
		rollbackErr   error
		wantCommitted bool
		wantErrs      []error
		// wantNotErrs are errors the result must NOT carry, which is how a case
		// pins the absence of a manufactured cleanup error.
		wantNotErrs []error
		wantEnded   []string
	}{
		{
			name:          "success commits once and never rolls back",
			wantCommitted: true,
			wantEnded:     []string{"commit"},
		},
		{
			name:      "begin failure never reaches the work",
			beginErr:  errBegin,
			wantErrs:  []error{errBegin},
			wantEnded: nil,
		},
		{
			name:      "work failure rolls back",
			workErr:   errWork,
			wantErrs:  []error{errWork},
			wantEnded: []string{"rollback"},
		},
		{
			// Commit ends the transaction, so rolling back afterwards could only
			// answer sql.ErrTxDone. Reporting that as a rollback failure would
			// describe a second problem the caller does not have.
			name:        "commit failure is not followed by a rollback",
			commitErr:   errCommit,
			wantErrs:    []error{errCommit},
			wantNotErrs: []error{ErrRollback, sql.ErrTxDone},
			wantEnded:   []string{"commit"},
		},
		{
			name:        "work failure keeps the rollback failure beside it",
			workErr:     errWork,
			rollbackErr: errRollback,
			wantErrs:    []error{errWork, errRollback, ErrRollback},
			wantEnded:   []string{"rollback"},
		},
		{
			// The rollback error is configured but unreachable: no rollback is
			// attempted after Commit, so the commit failure stands alone.
			name:        "a configured rollback failure cannot reach a commit failure",
			commitErr:   errCommit,
			rollbackErr: errRollback,
			wantErrs:    []error{errCommit},
			wantNotErrs: []error{errRollback, ErrRollback, sql.ErrTxDone},
			wantEnded:   []string{"commit"},
		},
		{
			name:          "a failing rollback is irrelevant when the commit worked",
			rollbackErr:   errRollback,
			wantCommitted: true,
			wantEnded:     []string{"commit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tx := &stubTx{commitErr: tt.commitErr, rollbackErr: tt.rollbackErr}
			calls := 0
			committed, err := WithTransaction(t.Context(), stubBeginner{tx: tx, beginErr: tt.beginErr},
				func(_ *stubTx) error {
					calls++
					return tt.workErr
				})

			if committed != tt.wantCommitted {
				t.Errorf("committed = %v, want %v (err = %v)", committed, tt.wantCommitted, err)
			}
			if len(tt.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
			} else if err == nil {
				t.Fatalf("err = nil, want an error reaching %v", tt.wantErrs)
			}
			for _, want := range tt.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("errors.Is(err, %v) = false; err = %v", want, err)
				}
			}
			for _, unwanted := range tt.wantNotErrs {
				if errors.Is(err, unwanted) {
					t.Errorf("errors.Is(err, %v) = true, want false; err = %v", unwanted, err)
				}
			}
			if strings.Join(tx.ended, ",") != strings.Join(tt.wantEnded, ",") {
				t.Errorf("transaction endings = %v, want %v", tx.ended, tt.wantEnded)
			}
			wantCalls := 1
			if tt.beginErr != nil {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Errorf("work function called %d times, want %d", calls, wantCalls)
			}
		})
	}
}

// TestWithTransactionSurfacesTxDone checks that a rollback reporting ErrTxDone
// is not swallowed. The helper only rolls back a transaction it did not commit,
// so ErrTxDone there means something else already ended it — a defect that an
// unconditional `if errors.Is(err, sql.ErrTxDone) { return nil }` would hide.
func TestWithTransactionSurfacesTxDone(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	tx := &stubTx{rollbackErr: sql.ErrTxDone}
	committed, err := WithTransaction(t.Context(), stubBeginner{tx: tx}, func(_ *stubTx) error {
		return cause
	})
	if committed {
		t.Error("committed = true, want false")
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(err, cause) = false; err = %v", err)
	}
	if !errors.Is(err, sql.ErrTxDone) {
		t.Errorf("errors.Is(err, sql.ErrTxDone) = false; err = %v", err)
	}
}

// TestWithTransactionErrorAsReachesBothSides checks errors.As, not just
// errors.Is: a caller that inspects a typed cause must still find it after a
// rollback failure has been joined onto it.
func TestWithTransactionErrorAsReachesBothSides(t *testing.T) {
	t.Parallel()

	cause := &typedError{code: 42}
	tx := &stubTx{rollbackErr: errors.New("rollback failed")}
	_, err := WithTransaction(t.Context(), stubBeginner{tx: tx}, func(_ *stubTx) error {
		return cause
	})

	var got *typedError
	if !errors.As(err, &got) {
		t.Fatalf("errors.As did not find the typed cause; err = %v", err)
	}
	if got.code != 42 {
		t.Errorf("code = %d, want 42", got.code)
	}
	if !errors.Is(err, ErrRollback) {
		t.Errorf("rollback failure lost; err = %v", err)
	}
}

type typedError struct{ code int }

func (e *typedError) Error() string { return "typed error" }

// TestWithTransactionCommitFailureOnRealTx is the case the stubs could not catch
// on their own, and the defect that survived the first version of this helper.
//
// A real *sql.Tx is terminal the moment Commit is called, so a rollback
// afterwards answers sql.ErrTxDone. Code that rolled back on the commit-failure
// path therefore joined an ErrTxDone describing nothing the caller can act on —
// a second, invented problem on top of the real one. A deferred foreign key
// gives a genuine commit failure on a genuine transaction: the INSERT succeeds
// and COMMIT is what fails, so this exercises the exact path.
func TestWithTransactionCommitFailureOnRealTx(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// One connection, so the PRAGMA and the transaction share a session.
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE parent (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE child (id INTEGER PRIMARY KEY,
		     pid INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED)`,
	} {
		if _, err := db.ExecContext(t.Context(), stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	committed, err := WithTransaction(t.Context(), SQLTxBeginner{DB: db}, func(tx *sql.Tx) error {
		// Valid until COMMIT checks the deferred constraint.
		_, execErr := tx.ExecContext(t.Context(), `INSERT INTO child (id, pid) VALUES (1, 999)`)
		return execErr
	})

	if committed {
		t.Error("committed = true for a transaction whose COMMIT failed")
	}
	if err == nil {
		t.Fatal("err = nil, want the commit failure")
	}
	if !strings.Contains(err.Error(), "commit transaction") {
		t.Errorf("err = %v, want it to name the commit", err)
	}
	if errors.Is(err, ErrRollback) {
		t.Errorf("a rollback was attempted after Commit; err = %v", err)
	}
	if errors.Is(err, sql.ErrTxDone) {
		t.Errorf("err carries a manufactured sql.ErrTxDone: %v", err)
	}

	// The failed commit left nothing behind, and the session still works.
	var n int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM child`).Scan(&n); err != nil {
		t.Fatalf("database unusable after the failed commit: %v", err)
	}
	if n != 0 {
		t.Errorf("child rows = %d after a failed commit, want 0", n)
	}
}

// TestWithTransactionContextCancel checks that cancelling the context is not
// reported as a broken transaction lifecycle. database/sql rolls the
// transaction back itself when the context is done, so the rollback here loses
// the race and gets sql.ErrTxDone. That is the documented outcome of
// cancellation, and the cause the caller needs is already the context error.
func TestWithTransactionContextCancel(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithCancel(t.Context())
	committed, err := WithTransaction(ctx, SQLTxBeginner{DB: db}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE t (a)`); err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	})

	if committed {
		t.Error("committed = true for a cancelled transaction")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
	if errors.Is(err, ErrRollback) {
		t.Errorf("cancellation was reported as a rollback failure; err = %v", err)
	}

	// The session must still be usable afterwards: a cancelled transaction is
	// not a poisoned database.
	if _, err := db.ExecContext(t.Context(), `CREATE TABLE after_cancel (a)`); err != nil {
		t.Errorf("database unusable after a cancelled transaction: %v", err)
	}
	var n int
	if err := db.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='t'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("the cancelled transaction's CREATE TABLE survived")
	}
}

// TestWithTransactionRetryAfterRollbackFailure checks that a failed unit of work
// leaves the caller able to try again. A helper that left a transaction open, or
// a connection checked out, would fail here on the second attempt rather than on
// the first.
func TestWithTransactionRetryAfterRollbackFailure(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cause := errors.New("staging failed")
	for i := range 3 {
		committed, err := WithTransaction(t.Context(), SQLTxBeginner{DB: db}, func(tx *sql.Tx) error {
			if _, execErr := tx.ExecContext(t.Context(), `CREATE TABLE discarded (a)`); execErr != nil {
				return execErr
			}
			return cause
		})
		if committed || !errors.Is(err, cause) {
			t.Fatalf("attempt %d: (%v, %v), want (false, staging failed)", i, committed, err)
		}
	}

	committed, err := WithTransaction(t.Context(), SQLTxBeginner{DB: db}, func(tx *sql.Tx) error {
		_, execErr := tx.ExecContext(t.Context(), `CREATE TABLE kept (a)`)
		return execErr
	})
	if !committed || err != nil {
		t.Fatalf("after three failed attempts: (%v, %v), want (true, nil)", committed, err)
	}
}

// TestWithTransactionRealDatabase runs the helper against a real *sql.DB so the
// generic instantiation used in production is exercised, not only the stubs.
func TestWithTransactionRealDatabase(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	committed, err := WithTransaction(t.Context(), SQLTxBeginner{DB: db}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), `CREATE TABLE kept (v TEXT)`)
		return err
	})
	if err != nil || !committed {
		t.Fatalf("WithTransaction = (%v, %v), want (true, nil)", committed, err)
	}

	failure := errors.New("abandon")
	committed, err = WithTransaction(t.Context(), SQLTxBeginner{DB: db}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(t.Context(), `CREATE TABLE discarded (v TEXT)`); err != nil {
			return err
		}
		return failure
	})
	if committed {
		t.Error("committed = true for a failing unit of work")
	}
	if !errors.Is(err, failure) {
		t.Errorf("errors.Is(err, failure) = false; err = %v", err)
	}

	var n int
	if err := db.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='discarded'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("the rolled-back CREATE TABLE survived")
	}
	if err := db.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='kept'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("the committed CREATE TABLE did not survive")
	}
}
