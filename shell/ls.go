package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nao1215/sqly/config"
)

// lsCommand list files and directories.
// If there is no argument, list the files and directories in the current directory.
// If there is one argument, list the files and directories in the specified directory.
// If there are multiple arguments, return an error.
//
// Listing is done in-process rather than shelling out to ls/dir. Why: external
// binaries differ per platform (ls -l vs dir /q) and produce inconsistent
// output. Entries are sorted by name and directories carry a trailing "/" so
// the result is deterministic across supported operating systems.
func (c CommandList) lsCommand(_ context.Context, _ *Shell, argv []string) error {
	if len(argv) > 1 {
		return errors.New("too many arguments")
	}
	path := "."
	if len(argv) == 1 {
		expanded, err := expandTilde(argv[0])
		if err != nil {
			return err
		}
		path = expanded
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no such file or directory: %s", path)
		}
		return err
	}

	// A non-directory argument lists just that entry, mirroring `ls FILE`.
	if !info.IsDir() {
		fmt.Fprintln(config.Stdout, filepath.Base(path))
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintln(config.Stdout, name)
	}
	return nil
}
