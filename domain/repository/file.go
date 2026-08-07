package repository

import "os"

// FileRepository is a repository that handles file creation for export.
type FileRepository interface {
	// Create creates or opens a file at the given path for writing.
	Create(filePath string) (*os.File, error)
}
