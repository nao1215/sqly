//go:build wireinject
// +build wireinject

// Pacakge di Inject dependence by wire command.
package di

import (
	"github.com/google/wire"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/infrastructure/memory"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/shell"
	"github.com/nao1215/sqly/usecase"
)

//go:generate wire

// NewShell initailize main class of sqly application.
// The return function is the function to close the DB.
func NewShell(args []string) (*shell.Shell, func(), error) {
	wire.Build(
		config.NewConfig,
		config.NewInMemDB,
		config.NewHistoryDB,
		config.NewArg,
		shell.NewShell,
		shell.NewCommands,
		usecase.NewCSVInteractor,
		usecase.NewTSVInteractor,
		usecase.NewLTSVInteractor,
		usecase.NewJSONInteractor,
		usecase.NewHistoryInteractor,
		usecase.NewSQLite3Interactor,
		usecase.NewSQL,
		persistence.NewCSVRepository,
		persistence.NewTSVRepository,
		persistence.NewLTSVRepository,
		persistence.NewJSONRepository,
		persistence.NewHistoryRepository,
		memory.NewSQLite3Repository,
	)
	return nil, nil, nil
}
