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
type stubTx struct {
	commitErr   error
	rollbackErr error
	ended       []string
}

func (tx *stubTx) Commit() error {
	tx.ended = append(tx.ended, "commit")
	return tx.commitErr
}

func (tx *stubTx) Rollback() error {
	tx.ended = append(tx.ended, "rollback")
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
		wantEnded     []string
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
			name:      "commit failure rolls back",
			commitErr: errCommit,
			wantErrs:  []error{errCommit},
			wantEnded: []string{"commit", "rollback"},
		},
		{
			name:        "work failure keeps the rollback failure beside it",
			workErr:     errWork,
			rollbackErr: errRollback,
			wantErrs:    []error{errWork, errRollback, ErrRollback},
			wantEnded:   []string{"rollback"},
		},
		{
			name:        "commit failure keeps the rollback failure beside it",
			commitErr:   errCommit,
			rollbackErr: errRollback,
			wantErrs:    []error{errCommit, errRollback, ErrRollback},
			wantEnded:   []string{"commit", "rollback"},
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
