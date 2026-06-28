package shell

import (
	"os"
	"path/filepath"
	"strings"
)

// expandTilde expands a leading "~" or "~/..." in path to the current user's
// home directory. A bare "~" maps to home; "~/sub" (and the Windows "~\sub")
// maps to home joined with "sub". Why resolve here: helper commands pass the
// argument straight to os.Chdir/os.Stat, which do not understand "~", so the
// shell prompt advertises "~" while the commands rejected it.
//
// Forms other than "~" and "~<separator>..." (for example "~user" or a "~" that
// appears later in the path) are returned unchanged, so they fail as a literal
// path rather than silently resolving to the wrong location.
func expandTilde(path string) (string, error) {
	if !hasTildePrefix(path) {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return expandTildeWithHome(path, home), nil
}

// hasTildePrefix reports whether path is a bare "~" or starts with "~" followed
// by a path separator ("/" on every OS, plus the native separator on Windows).
func hasTildePrefix(path string) bool {
	return path == "~" ||
		strings.HasPrefix(path, "~/") ||
		strings.HasPrefix(path, "~"+string(os.PathSeparator))
}

// expandTildeWithHome performs the expansion against an explicit home directory
// so the rule is unit-testable without depending on the host account.
func expandTildeWithHome(path, home string) string {
	if !hasTildePrefix(path) {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}
