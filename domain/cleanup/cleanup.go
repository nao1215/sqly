// Package cleanup carries the one rule sqly uses for errors produced while
// releasing or undoing something: detaching a database, closing a writer,
// removing a staging file.
//
// It is a leaf package with no dependencies inside sqly, because every layer
// has cleanup to do and the rule must be the same everywhere. Before it
// existed, each site wrote its own variant of "assign the cleanup error only
// when the primary error is nil", which discards the cleanup failure whenever
// the operation also failed — the case where cleanup is most likely to fail and
// most important to report, because that is when a temporary file is left on
// disk or a database is left attached.
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrCleanup marks a failure in work done to undo or release something after
// the operation itself finished. It is joined onto the primary error rather
// than replacing it, because the two answer different questions — "did the
// operation do what I asked" and "is anything left over".
//
// The transaction rollback case has its own sentinel in the infrastructure
// package, so a caller can tell "the work could not be undone" from "something
// was left over".
var ErrCleanup = errors.New("cleanup")

// Join attaches a cleanup failure to the primary error of an operation.
//
// It exists so no call site has to decide the rule for itself. The rule that
// kept appearing by hand — assign the cleanup error only when the primary error
// is nil — loses the cleanup failure whenever the operation also failed, which
// is when cleanup is most likely to fail and most important to hear about.
// Joining keeps both reachable through errors.Is and errors.As.
//
// what names the cleanup step for the message ("close the output file"). A nil
// cleanupErr returns primary unchanged, so callers can call it unconditionally.
func Join(primary, cleanupErr error, what string) error {
	if cleanupErr == nil {
		return primary
	}
	return errors.Join(primary, fmt.Errorf("%w: %s: %w", ErrCleanup, what, cleanupErr))
}

// cleanupGrace is how long cleanup may take after the operation's own context is
// already done. It is short: the point is to release a resource, not to finish
// the work that was cancelled.
const cleanupGrace = 5 * time.Second

// Context returns the context to run cleanup under.
//
// While the operation's context is still live, cleanup uses it, so a caller's
// deadline continues to apply. Once it is done — cancelled, or past its
// deadline — cleanup cannot use it: every statement would fail immediately, and
// the resource would stay held. Cancelling a query is a reason to release what
// it attached, not a reason to leak it. In that case cleanup gets a detached
// context with a short grace period instead.
//
// The returned cancel func must always be called.
func Context(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return ctx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupGrace)
}
