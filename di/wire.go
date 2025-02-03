//go:build wireinject
// +build wireinject

// Package di Inject dependence by wire command.
package di

import (
	"github.com/google/wire"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/infrastructure/memory"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/interactor"
	"github.com/nao1215/sqly/shell"
)

//go:generate wire

// NewShell initailize main class of sqly application.
// The return function is the function to close the DB.
func NewShell(args []string) (*shell.Shell, func(), error) {
	wire.Build(
		config.Set,
		shell.Set,
		interactor.Set,
		persistence.Set,
		memory.Set,
	)
	return nil, nil, nil
}
