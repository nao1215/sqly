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

// Create open file or create file.
func (fr *fileRepository) Create(path string) (*os.File, error) {
	return os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE, 0600)
}
