package shell

import (
	"context"
	"errors"
	"io"

	"github.com/nao1215/sqly/config"
)

// A caller that runs sqly from a script has one question when it fails: whose
// fault was it, and what should happen next. "The command line was wrong" needs
// a human; "the input would not import" needs a different file; "the disk was
// full" needs a retry. One non-zero code answered none of them, so a wrapper had
// to grep stderr — matching on wording that was free to change until it did.
//
// The codes below are that answer. They are decided from the error types in
// errors.go rather than from message text, so the classification and the message
// stay independent: rewording an error never moves a run into another class.
const (
	// ExitOK is a run that did what it was asked.
	ExitOK = 0
	// ExitFailure is a statement that ran and failed: a SQL error, a constraint,
	// a missing table. The invocation was well-formed and the data was readable.
	ExitFailure = 1
	// ExitUsage is a command line or a script sqly will not accept: an unknown
	// flag, two flags that contradict, a helper command where only SQL is
	// allowed, a format that cannot carry the results the script produces.
	// Nothing was read and nothing was written.
	ExitUsage = 2
	// ExitInput is an input that could not be read: a missing path, an
	// unsupported format, a download that failed or exceeded its limits, a
	// malformed row under --row-mismatch error.
	ExitInput = 3
	// ExitOutput is a destination that could not be written: a missing parent
	// directory, a source with no writable form, a collision, a failed commit or
	// rollback. Query results may already have been produced.
	ExitOutput = 4
	// ExitInterrupt is a run cut short by SIGINT or SIGTERM. 128+SIGINT is what
	// a shell reports for a process killed by that signal, so a wrapper that
	// already special-cases 130 keeps working.
	ExitInterrupt = 130
)

// ExitCode maps a run's error to the code sqly exits with. A nil error is
// ExitOK.
//
// The stages are tried in the order a run passes through them, so an error that
// carries more than one classification is reported as the earliest one: a bad
// invocation that would also have failed to write is a usage error, because
// fixing the command line is what has to happen first.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}

	// An interrupt is checked first because it is not a stage: it can arrive
	// during any of them, and what it means — the user stopped this — does not
	// depend on where the run had got to.
	//
	// Only context.Canceled. A context.DeadlineExceeded is a timeout, not a user
	// signal, and sqly's only deadlines are on the HTTP client: treating one as an
	// interrupt would report a download that timed out as 130 instead of as the
	// input failure it is.
	if errors.Is(err, context.Canceled) {
		return ExitInterrupt
	}

	var (
		argErr         *config.ArgError
		invocationErr  *invocationError
		scriptErr      *scriptError
		resultCountErr *resultCountError
	)
	if errors.As(err, &argErr) || errors.As(err, &invocationErr) ||
		errors.As(err, &scriptErr) || errors.As(err, &resultCountErr) {
		return ExitUsage
	}

	var (
		partialErr      *partialImportError
		importFailedErr *importFailedError
		scriptSourceErr *scriptSourceError
	)
	if errors.As(err, &partialErr) || errors.As(err, &importFailedErr) ||
		errors.As(err, &scriptSourceErr) {
		return ExitInput
	}

	var (
		outputPathErr *outputPathError
		writeBackErr  *writeBackError
		fileOpErr     *fileOpError
	)
	if errors.As(err, &outputPathErr) || errors.As(err, &writeBackErr) ||
		errors.As(err, &fileOpErr) {
		return ExitOutput
	}

	return ExitFailure
}

// readAllContext is io.ReadAll that gives up when ctx is cancelled.
//
// It exists because trapping SIGINT is only half of handling it. main.go cancels
// the run's context instead of letting the default handler kill the process,
// which is what lets the deferred cleanup remove the temp directories a download
// or a staged stdin dataset created. The cost is that every blocking read now has
// to notice the cancellation itself: an io.ReadAll parked on a pipe whose writer
// never closes — a harness that hands its child a reader, a FIFO with an idle
// writer — used to be killed outright by the signal, and after trapping it would
// simply sit there. "Ctrl-C quits" would have become "Ctrl-C does nothing".
//
// The read continues in its goroutine after a cancellation returns. There is no
// way to interrupt it (the reader may be any io.Reader, with no deadline to set),
// and nothing is waiting on it: the process is on its way out, and the goroutine
// goes with it.
func readAllContext(ctx context.Context, r io.Reader) ([]byte, error) {
	type outcome struct {
		data []byte
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		data, err := io.ReadAll(r)
		done <- outcome{data: data, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-done:
		return res.data, res.err
	}
}
