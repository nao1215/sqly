package usecase

import "context"

//go:generate mockgen -typed -source=$GOFILE -destination=../interactor/mock/$GOFILE -package mock

// PersistenceUsecase reconstructs native financial files from a table set. These
// operations round-trip session tables back out to on-disk files, so they are
// grouped apart from plain file import.
type PersistenceUsecase interface {
	// DumpACHFile reconstructs a complete ACH file at outputPath from the table set
	// registered under baseName, reflecting any UPDATEs applied in the session.
	DumpACHFile(ctx context.Context, baseName, outputPath string) error
	// DumpFedWireFile reconstructs a complete Fedwire file at outputPath from the
	// message table registered under baseName, reflecting any UPDATEs in the session.
	DumpFedWireFile(ctx context.Context, baseName, outputPath string) error
}
