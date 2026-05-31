package persistence

import (
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/domain/repository"
)

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
	const defaultFilePerm = 0o600
	return os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultFilePerm)
}
