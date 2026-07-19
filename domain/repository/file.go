package repository

import "os"

// FileRepository is a repository that handles file creation for export.
type FileRepository interface {
	// Create creates or opens a file at the given path for writing.
	Create(filePath string) (*os.File, error)
	// CreateTemp creates a temporary file in the specified directory.
	CreateTemp(dir, pattern string) (*os.File, error)
	// Remove deletes a file.
	Remove(path string) error
}
