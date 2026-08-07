package testutil

import (
	"os"
	"path/filepath"
)

// NewTempHistoryPath returns a history file path inside a fresh temp directory,
// and the function that removes it.
//
// It lives here rather than beside the real constructor in config because
// nothing sqly ships ever wants one: the shell's history is a file under the
// user's config directory so that it survives the session. Only a test wants
// history that disappears with the run, and it wants it somewhere the
// developer's real history is not.
//
// The file itself is not created; the repository's Init does that, which is the
// step a test of an unwritable location wants to watch fail.
func NewTempHistoryPath() (string, func(), error) {
	dir, err := os.MkdirTemp("", "sqly-history-*")
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(dir, "history"), func() { _ = os.RemoveAll(dir) }, nil
}
