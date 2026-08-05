package shell

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nao1215/sqly/domain/cleanup"
)

// Writing a file is the same problem twice in sqly: `--output` writes one query
// result, and `.save` writes the tables a session changed. Both have to leave an
// existing file exactly as it was when anything goes wrong, and both have to
// keep its permissions when it does not.
//
// The rules live here so there is one answer rather than two. What stays apart
// is what genuinely differs: `--output` replaces a single file and can commit as
// soon as it is written, while `.save` must hold every staged file until all of
// them exist, and can then have to undo commits that already landed. That
// difference is the reason `.save` drives the phases itself instead of calling
// the helper below.

// writeFileAtomically serializes into a scratch file beside dest and moves it
// into place. The destination is not touched until write has finished
// successfully, so a serializer that fails on the third row — a value the format
// cannot hold, a full disk — leaves the previous file whole.
//
// A rename is what makes that true, and a rename is not always available:
// commitStagedFile falls back to copying over the destination where the platform
// refuses one, and a copy truncates before it writes. So an existing destination
// is copied aside first and put back if the commit reports it was touched. That
// backup is the difference between "an existing file is either the old one or
// the new one" being a guarantee and being a hope about which syscall was used.
//
// write is given the scratch path and must produce the complete file at it.
func (s *Shell) writeFileAtomically(dest string, write func(path string) error) (err error) {
	staging, err := s.fs().stagingPath(dest, ".sqly-out-*")
	if err != nil {
		return &fileOpError{Op: opStage, Path: dest, Err: err}
	}
	defer func() {
		// The scratch file is removed whatever happened. A successful rename already
		// consumed it, but the fallback copy does not: it leaves the scratch file
		// sitting beside the destination, which is how a Windows run would litter a
		// dot-file next to every export. Remove is therefore unconditional, and a
		// "no such file" from the rename case is not a failure.
		if removeErr := s.fs().Remove(staging); !isNotExist(removeErr) {
			err = cleanup.Join(err, removeErr, fmt.Sprintf("remove the staged file %q", staging))
		}
	}()

	if err := write(staging); err != nil {
		return &stagedWriteError{message: renamePathInMessage(err.Error(), staging, dest), Err: err}
	}

	// The backup is taken only if it is about to be needed. A rename replaces the
	// destination without ever reading it, so copying the old file aside first
	// would double the cost of every export for a case that a successful rename
	// never reaches. commitStagedFile calls this the moment it decides to fall
	// back to a copy, which is the only path that can damage what is there.
	backup := ""
	defer func() {
		if backup == "" {
			return
		}
		err = cleanup.Join(err, s.fs().Remove(backup), fmt.Sprintf("remove the backup file %q", backup))
	}()
	takeBackup := func() error {
		saved, backupErr := s.backupExisting(dest)
		if backupErr != nil {
			return &fileOpError{Op: opBackup, Path: dest, Err: backupErr}
		}
		backup = saved
		return nil
	}

	touched, commitErr := s.commitStagedFile(staging, dest, takeBackup)
	if commitErr == nil {
		return nil
	}
	failure := error(&fileOpError{Op: opCommit, Path: dest, Err: commitErr})
	if !touched {
		// A rename that failed did not create or alter the destination, so there is
		// nothing to put back.
		return failure
	}
	if restoreErr := s.restoreFromBackup(dest, backup); restoreErr != nil {
		// The destination is now neither the old file nor the new one, which is the
		// one outcome the user has to be told about. It is reported alongside the
		// failure that caused it, never instead of it.
		return errors.Join(failure, restoreErr)
	}
	return failure
}

// restoreFromBackup puts dest back to what backup holds, or removes dest when
// there was no file there before this write created one. It is what
// rollbackCommitted does for one target, shared so `--output` and `.save` undo a
// touched destination the same way.
func (s *Shell) restoreFromBackup(dest, backup string) error {
	if backup == "" {
		if err := s.fs().Remove(dest); err != nil {
			return &fileOpError{Op: opRollback, Path: dest,
				Err: fmt.Errorf("could not remove the file this write created: %w", err)}
		}
		return nil
	}
	if err := s.fs().Copy(backup, dest); err != nil {
		return &fileOpError{Op: opRollback, Path: dest,
			Err: fmt.Errorf("could not restore it from its backup; it now holds the new content: %w", err)}
	}
	return nil
}

// stagedWriteError is a failure to serialize into the scratch file, reported
// with the scratch path rewritten to the destination.
//
// It is a type rather than a fresh errors.New for one reason: rewriting a
// message must not cost the error underneath it. A caller asking errors.Is
// whether the export hit a format it could not represent is asking about the
// failure, not about which path was in its text.
type stagedWriteError struct {
	message string
	Err     error
}

func (e *stagedWriteError) Error() string { return e.message }
func (e *stagedWriteError) Unwrap() error { return e.Err }

// renamePathInMessage rewrites every mention of the scratch path as the
// destination. The serializer names the path it was writing, which is the
// scratch file; the user asked for the destination and has no reason to learn
// the other name, so the message reads as if the write had gone straight there.
//
// Both spellings are replaced. A message that formatted the path with %q holds
// it escaped, and on Windows escaping a path changes it: `.\\out.csv` in the
// message is `.\out.csv` on disk. Replacing only the raw form left the scratch
// name visible there and nowhere else, which is the kind of difference only the
// platform it breaks on ever reports.
func renamePathInMessage(message, staging, dest string) string {
	message = strings.ReplaceAll(message, strconv.Quote(staging), strconv.Quote(dest))
	return strings.ReplaceAll(message, staging, dest)
}

// isNotExist reports whether err is nil or a "no such file" — the two answers
// that mean there is nothing left to clean up.
func isNotExist(err error) bool {
	return err == nil || errors.Is(err, os.ErrNotExist)
}
