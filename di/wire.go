//go:build wireinject

// Package di Inject dependence by wire command.
package di

import (
	"database/sql"

	"github.com/google/wire"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/memory"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/interactor"
	"github.com/nao1215/sqly/shell"
)

// provideFileSQLAdapter creates a FileSQLAdapter with the shared database
func provideFileSQLAdapter(db config.MemoryDB) *filesql.FileSQLAdapter {
	return filesql.NewFileSQLAdapter((*sql.DB)(db))
}

//go:generate wire

// NewShell initailize main class of sqly application.
// The return function is the function to close the DB.
func NewShell(args []string) (*shell.Shell, func(), error) {
	wire.Build(
		config.Set,
		shell.Set,
		interactor.Set,
		memory.Set,
		persistence.Set, // Full persistence set for repositories
		// Add filesql adapter for filesql-only approach
		provideFileSQLAdapter,
	)
	return nil, nil, nil
}
