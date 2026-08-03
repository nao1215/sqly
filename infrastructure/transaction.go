package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Tx is the part of *sql.Tx a transactional unit of work needs to finish: the
// two calls that end it. Statement execution is not part of this interface —
// callers receive the concrete transaction and use it directly — so production
// code gains no indirection it does not need, while tests can supply a
// two-method fake that fails commit, fails rollback, or fails both.
type Tx interface {
	Commit() error
	Rollback() error
}

// TxBeginner starts transactions. *sql.DB satisfies it, and a test double can
// satisfy it to make BeginTx itself fail.
type TxBeginner[T Tx] interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (T, error)
}

// ErrRollback marks the cleanup half of a failed transaction: the rollback that
// was supposed to undo the work also failed, so the database is in a state
// neither the caller's intent nor the pre-transaction state describes. It is
// joined onto the error that caused the rollback rather than replacing it, so
// errors.Is finds both the reason the work failed and the fact that undoing it
// did not work either.
var ErrRollback = errors.New("rollback transaction")

// txPhase is how far a transaction got, which is what decides whether a
// rollback is still meaningful.
//
// The distinction that matters is not "did it succeed" but "was Commit called".
// database/sql ends a transaction the moment Commit is invoked, whatever Commit
// then returns, so a rollback afterwards can only ever answer sql.ErrTxDone.
// Treating a failed commit as "not finished, so roll back" therefore does not
// undo anything — it manufactures a second error describing a transaction that
// was already over.
type txPhase int

const (
	// phaseStaging: the unit of work is running and Commit has not been called,
	// so a failure still needs a rollback.
	phaseStaging txPhase = iota
	// phaseCommitCalled: Commit was invoked. The transaction is terminal
	// whether it succeeded or failed, and must not be rolled back.
	phaseCommitCalled
)

// WithTransaction runs fn inside a transaction begun on db and returns the
// outcome, with one rule for cleanup errors that every caller shares.
//
// The transaction ends exactly once. A failure while staging is rolled back. A
// commit is never followed by a rollback, whether the commit succeeded or
// failed: database/sql ends the transaction when Commit is called, so a
// rollback afterwards can only report sql.ErrTxDone, and joining that onto a
// real commit failure would tell the caller about a second problem that does
// not exist. A commit failure is returned as a commit failure and nothing else.
//
// A rollback failure never displaces the error that caused it. The two are
// joined, so errors.Is and errors.As reach the original cause (a parse failure,
// a constraint violation) and the cleanup failure alike. The previous shape of
// this code — assigning the rollback error only when the primary error was nil —
// dropped the rollback failure in precisely the case that produces one, because
// a rollback happens when something already went wrong.
//
// One rollback error is not a defect: when the context is done, database/sql's
// own watcher has already rolled the transaction back, so the rollback here
// races with it and loses. That yields sql.ErrTxDone with a cancelled context,
// which is the documented outcome of cancellation rather than a broken
// lifecycle, so it is not reported as a rollback failure. The cause of the
// cancellation is already in err, which fn returned. An ErrTxDone with a live
// context is a different matter — something outside this function ended the
// transaction — and is still reported.
//
// The returned bool reports whether the transaction committed. Callers use it to
// gate work that must not become visible before the data is durable, such as
// publishing an in-memory registry; err != nil already implies false, and the
// explicit flag keeps that decision from being re-derived at each call site.
func WithTransaction[T Tx](
	ctx context.Context,
	db TxBeginner[T],
	fn func(tx T) error,
) (committed bool, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}

	phase := phaseStaging
	defer func() {
		if phase == phaseCommitCalled {
			return
		}
		rollbackErr := tx.Rollback()
		if rollbackErr == nil {
			return
		}
		if errors.Is(rollbackErr, sql.ErrTxDone) && ctx.Err() != nil {
			return
		}
		err = errors.Join(err, fmt.Errorf("%w: %w", ErrRollback, rollbackErr))
	}()

	if err := fn(tx); err != nil {
		return false, err
	}

	// Enter the terminal phase before calling Commit, not after it succeeds:
	// the transaction is over either way, and the deferred rollback must not
	// run on the failure path.
	phase = phaseCommitCalled
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}
	return true, nil
}

// SQLTxBeginner adapts *sql.DB to TxBeginner. The method set of *sql.DB already
// matches, but its BeginTx returns *sql.Tx, so the adapter exists to name the
// type argument at the call site rather than to add behavior.
type SQLTxBeginner struct {
	DB *sql.DB
}

// BeginTx starts a transaction on the wrapped database.
func (b SQLTxBeginner) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return b.DB.BeginTx(ctx, opts)
}

// SQLConnTxBeginner adapts a single *sql.Conn to TxBeginner.
//
// It exists because some SQLite state is per-connection rather than
// per-database — an ATTACHed schema, above all. Work that depends on such state
// must run on the connection that established it, so the transaction has to be
// begun from the connection rather than from the pool, which would hand out an
// arbitrary one.
type SQLConnTxBeginner struct {
	Conn *sql.Conn
}

// BeginTx starts a transaction on the wrapped connection.
func (b SQLConnTxBeginner) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return b.Conn.BeginTx(ctx, opts)
}
