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

// WithTransaction runs fn inside a transaction begun on db and returns the
// outcome, with one rule for cleanup errors that every caller shares.
//
// The transaction ends exactly once. On success it is committed; on any failure
// — fn's, or the commit's — it is rolled back, and only then. A commit that
// succeeded is never followed by a rollback, so sql.ErrTxDone is not something
// this function has to swallow: if a rollback still reports ErrTxDone, the
// transaction was ended by something other than this function, which is a real
// defect and is reported rather than hidden.
//
// A rollback failure never displaces the error that caused it. The two are
// joined, so errors.Is and errors.As reach the original cause (a parse failure,
// a constraint violation) and the cleanup failure alike. The previous shape of
// this code — assigning the rollback error only when the primary error was nil —
// dropped the rollback failure in precisely the case that produces one, because
// a rollback happens when something already went wrong.
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

	defer func() {
		if committed {
			return
		}
		rollbackErr := tx.Rollback()
		if rollbackErr == nil {
			return
		}
		err = errors.Join(err, fmt.Errorf("%w: %w", ErrRollback, rollbackErr))
	}()

	if err := fn(tx); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
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
