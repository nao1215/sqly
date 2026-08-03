package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestJoinKeepsBothErrors is the whole point of the package: the rule it
// replaces — assign the cleanup error only when the primary error is nil —
// dropped the cleanup failure whenever the operation also failed, which is the
// case that matters most, because that is when a file is left on disk or a
// database is left attached.
func TestJoinKeepsBothErrors(t *testing.T) {
	t.Parallel()

	primary := errors.New("export failed")
	cleanupErr := errors.New("permission denied")

	tests := []struct {
		name        string
		primary     error
		cleanupErr  error
		wantNil     bool
		wantErrs    []error
		wantNotErrs []error
	}{
		{
			name:    "both succeeded",
			wantNil: true,
		},
		{
			name:     "primary failed, cleanup succeeded",
			primary:  primary,
			wantErrs: []error{primary},
			// No cleanup happened, so nothing should claim one failed.
			wantNotErrs: []error{ErrCleanup},
		},
		{
			name:       "primary succeeded, cleanup failed",
			cleanupErr: cleanupErr,
			wantErrs:   []error{cleanupErr, ErrCleanup},
		},
		{
			// The case the old rule silently discarded.
			name:       "both failed",
			primary:    primary,
			cleanupErr: cleanupErr,
			wantErrs:   []error{primary, cleanupErr, ErrCleanup},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Join(tt.primary, tt.cleanupErr, "remove staging file")
			if tt.wantNil {
				if got != nil {
					t.Fatalf("Join = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("Join = nil, want an error reaching %v", tt.wantErrs)
			}
			for _, want := range tt.wantErrs {
				if !errors.Is(got, want) {
					t.Errorf("errors.Is(err, %v) = false; err = %v", want, got)
				}
			}
			for _, unwanted := range tt.wantNotErrs {
				if errors.Is(got, unwanted) {
					t.Errorf("errors.Is(err, %v) = true, want false; err = %v", unwanted, got)
				}
			}
			if tt.cleanupErr != nil && !strings.Contains(got.Error(), "remove staging file") {
				t.Errorf("err = %q, want it to name the cleanup step", got)
			}
		})
	}
}

// TestJoinPreservesTypedErrors checks errors.As, not only errors.Is: a caller
// that inspects a typed cause must still find it after a cleanup failure has
// been joined on. Formatting the two into one string would break this.
func TestJoinPreservesTypedErrors(t *testing.T) {
	t.Parallel()

	got := Join(&pathError{path: "/tmp/x"}, errors.New("cleanup failed"), "close")

	var typed *pathError
	if !errors.As(got, &typed) {
		t.Fatalf("errors.As did not reach the typed primary error; err = %v", got)
	}
	if typed.path != "/tmp/x" {
		t.Errorf("path = %q, want /tmp/x", typed.path)
	}
	if !errors.Is(got, ErrCleanup) {
		t.Errorf("cleanup marker lost; err = %v", got)
	}
}

type pathError struct{ path string }

func (e *pathError) Error() string { return "path error: " + e.path }

// TestContextStaysLiveWhenNotDone checks that cleanup inherits the caller's
// deadline while the operation's context is still usable, so a caller's timeout
// keeps applying.
func TestContextStaysLiveWhenNotDone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()

	got, release := Context(ctx)
	defer release()

	if got != ctx {
		t.Error("Context returned a different context while the original was live")
	}
	if got.Err() != nil {
		t.Errorf("cleanup context is already done: %v", got.Err())
	}
}

// TestContextSurvivesCancellation is the reason the helper exists. Reusing a
// cancelled context for cleanup makes every statement fail immediately, so the
// resource stays held — a cancelled query would leave a database attached and
// break the next run. Cleanup gets a detached context with a grace period.
func TestContextSurvivesCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if ctx.Err() == nil {
		t.Fatal("precondition: the context should be cancelled")
	}

	got, release := Context(ctx)
	defer release()

	if got.Err() != nil {
		t.Errorf("cleanup context is unusable after cancellation: %v", got.Err())
	}
	deadline, ok := got.Deadline()
	if !ok {
		t.Fatal("cleanup context has no deadline; it must not run unbounded")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > cleanupGrace {
		t.Errorf("cleanup deadline in %v, want (0, %v]", remaining, cleanupGrace)
	}
}

// TestContextReleaseIsAlwaysSafe checks that the returned cancel func can be
// called on both paths, so callers can defer it unconditionally.
func TestContextReleaseIsAlwaysSafe(t *testing.T) {
	t.Parallel()

	live, release := Context(t.Context())
	release()
	release() // idempotent
	if live == nil {
		t.Error("Context returned a nil context")
	}

	cancelled, stop := context.WithCancel(t.Context())
	stop()
	detached, release2 := Context(cancelled)
	release2()
	if detached.Err() == nil {
		t.Error("releasing the cleanup context did not cancel it")
	}
}
