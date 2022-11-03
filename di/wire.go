//go:build wireinject
// +build wireinject

// Pacakge di Inject dependence by wire command.
package di

import (
	"github.com/google/wire"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/infrastructure/persistence/csv"
	"github.com/nao1215/sqly/infrastructure/persistence/sqlite3"
	"github.com/nao1215/sqly/shell"
	"github.com/nao1215/sqly/usecase"
)

//go:generate wire

// NewShell initailize main class of sqly application.
// The return function is the function to close the DB.
func NewShell() (*shell.Shell, func(), error) {
	wire.Build(
		config.NewDB,
		config.NewArg,
		shell.NewShell,
		shell.NewCommands,
		shell.NewInteractive,
		shell.NewHistory,
		usecase.NewCSVInteractor,
		csv.NewCSVRepository,
		usecase.NewSQLite3Interactor,
		sqlite3.NewSQLite3Repository,
	)
	return nil, nil, nil
}
