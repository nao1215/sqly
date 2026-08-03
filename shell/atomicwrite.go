package shell

import (
	"errors"
	"fmt"
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
// write is given the scratch path and must produce the complete file at it.
func (s *Shell) writeFileAtomically(dest string, write func(path string) error) (err error) {
	staging, err := s.fs().stagingPath(dest, ".sqly-out-*")
	if err != nil {
		return &fileOpError{Op: opStage, Path: dest, Err: err}
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		// The scratch file is only ever left behind on a failure, and then it is a
		// leftover the caller has to hear about, so its removal joins the error
		// rather than replacing it.
		err = cleanup.Join(err, s.fs().Remove(staging), fmt.Sprintf("remove the staged file %q", staging))
	}()

	if err := write(staging); err != nil {
		return errors.New(renamePathInMessage(err.Error(), staging, dest))
	}
	if err := s.commitStagedFile(staging, dest); err != nil {
		return &fileOpError{Op: opCommit, Path: dest, Err: err}
	}
	committed = true
	return nil
}

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
