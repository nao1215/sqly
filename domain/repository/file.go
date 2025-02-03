package repository

import "os"

//go:generate mockgen -typed -source=$GOFILE -destination=../../infrastructure/mock/$GOFILE -package mock

// FileRepository is a repository that handles file.
type FileRepository interface {
	// Open open file.
	Open(filePath string) (*os.File, error)
	// Create open file or create file.
	Create(filePath string) (*os.File, error)
}
