package repository

import "os"

// FileRepository is a repository that handles file creation for export.
type FileRepository interface {
	// Create open file or create file.
	Create(filePath string) (*os.File, error)
}
