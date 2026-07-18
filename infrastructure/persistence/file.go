package persistence

import (
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/repository"
)

// defaultFilePerm is the permission for files sqly writes. It is non-executable
// (0600) so exports are treated as ordinary data files, consistent across CSV,
// TSV, LTSV, Parquet, and Excel outputs.
const defaultFilePerm = 0o600

// _ interface implementation check
var _ repository.FileRepository = (*fileRepository)(nil)

type fileRepository struct{}

// NewFileRepository return FileRepository
func NewFileRepository() repository.FileRepository {
	return &fileRepository{}
}

// Create creates the file, truncating it if it already exists, and returns it
// open for writing. O_TRUNC is required so overwriting an existing file with
// shorter content (for example saving a smaller table over its source, or a
// compressed export) does not leave stale trailing bytes that corrupt the file.
func (fr *fileRepository) Create(path string) (*os.File, error) {
	return os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultFilePerm)
}

// CreateTemp creates a temporary file in the specified directory.
func (fr *fileRepository) CreateTemp(dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

// Rename renames a file from oldPath to newPath.
func (fr *fileRepository) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// Remove deletes a file.
func (fr *fileRepository) Remove(path string) error {
	return os.Remove(path)
}
