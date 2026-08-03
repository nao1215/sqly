package shell

import (
	"io"
	"os"
	"path/filepath"
)

// fileOps is every filesystem call the write-back commit path makes, in one
// place so a test can make any single one of them fail.
//
// Write-back's guarantee — no source file is replaced until every table has been
// written — only holds if the failure happens at the right moment, and the
// moments that matter are "the second staging write", "the second rename", "the
// backup", and "the rollback". Reproducing those with real filesystem errors
// means read-only directories and chmod, which do not fail for root, do not mean
// the same thing on Windows, and can fail at a different step than the one under
// test. Naming the calls lets a test say exactly which one fails, on any OS, as
// the same user that runs the rest of the suite.
//
// It is deliberately small: the operations the commit path performs, nothing
// more. Everything else keeps calling the os package directly.
type fileOps struct {
	// CreateTemp reserves a scratch path in dir. Write-back stages beside the
	// destination so the later rename stays inside one filesystem.
	CreateTemp func(dir, pattern string) (*os.File, error)
	// Stat reports on a path, and is how the commit decides whether a
	// destination exists and what permissions to preserve.
	Stat func(name string) (os.FileInfo, error)
	// Chmod carries the destination's permissions onto the staged file.
	Chmod func(name string, mode os.FileMode) error
	// Rename moves a staged file onto its destination.
	Rename func(oldpath, newpath string) error
	// Remove discards a staged file or a backup.
	Remove func(name string) error
	// Copy replaces dest's contents with src's, for the platforms and cases where
	// a rename is refused.
	Copy func(src, dest string) error
}

// fs returns the filesystem calls this Shell should use: the ones a test
// installed, or the real filesystem. A Shell built as a bare struct literal —
// which many tests do — gets the real filesystem rather than a nil panic.
func (s *Shell) fs() fileOps {
	if s.files.Rename == nil {
		return defaultFileOps()
	}
	return s.files
}

// defaultFileOps is the real filesystem. Every Shell starts with it.
func defaultFileOps() fileOps {
	return fileOps{
		CreateTemp: os.CreateTemp,
		Stat:       os.Stat,
		Chmod:      os.Chmod,
		Rename:     os.Rename,
		Remove:     os.Remove,
		Copy:       copyFileContents,
	}
}

// copyFileContents replaces dest's contents with src's, truncating whatever was
// there. It is the fallback for a rename the platform refuses, and the way a
// backup is taken.
func copyFileContents(src, dest string) error {
	in, err := os.Open(src) //nolint:gosec // src is a file sqly created or was given as the output
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // dest is the destination the caller asked to save to
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// stagingPath reserves a scratch path beside dest and returns it. The scratch
// file lives in the destination's own directory so the later rename stays within
// one filesystem, where it is atomic; a dot prefix keeps it out of the way of a
// directory listing if the process dies between the write and the move.
//
// The destination's extension is kept on the end, because a writer may read it:
// the Excel and Parquet writers pick their format from the path they are handed,
// and a scratch name ending in ".sqly-out-1234" is a workbook format none of
// them recognize.
func (f fileOps) stagingPath(dest, suffix string) (string, error) {
	file, err := f.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+suffix+filepath.Ext(dest))
	if err != nil {
		return "", err
	}
	name := file.Name()
	// The writers below open the path themselves; the handle is only how the name
	// was reserved.
	if err := file.Close(); err != nil {
		_ = f.Remove(name)
		return "", err
	}
	return name, nil
}
