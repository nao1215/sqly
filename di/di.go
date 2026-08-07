// Package di holds sqly's composition root: the one place that knows how the
// application's parts are wired together.
//
// It is written by hand rather than generated. sqly's graph is a single path of
// about a dozen constructors called in dependency order, and the two resources
// that need releasing are released in the reverse of that order. A generator
// bought nothing for a graph that size, and it cost a build-time dependency on a
// project that is no longer maintained; this file is what it used to emit, only
// readable and diffable.
package di

import (
	"database/sql"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/memory"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/interactor"
	"github.com/nao1215/sqly/shell"
)

// The two constructors that can fail once something is already open are reached
// through variables so a test can make them fail and watch what gets released.
// sqly's own failures here are unreachable from the outside, and a cleanup that
// is never exercised is a cleanup that silently stops working.
//
// This is the seam main.go already uses for os.Exit, not a place to register
// dependencies: nothing but a test ever assigns to them, and the graph below is
// still written out in full.
var (
	newInMemDB   = config.NewInMemDB
	newSqlyShell = shell.NewShell
)

// NewShell builds the sqly application from args and returns it together with
// the function that releases everything it opened.
//
// The returned cleanup is the caller's to run exactly once, and only when the
// error is nil: a failure releases whatever it had already opened before
// returning, so there is never a resource to clean up alongside an error. The
// session database is the one thing with a lifetime.
func NewShell(args []string) (*shell.Shell, func(), error) {
	arg, err := config.NewArg(args)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, nil, err
	}
	commands := shell.NewCommands()

	// The in-memory database is the session: every imported table lives in it.
	// It is the first resource with a lifetime, so it is the last to be closed.
	memoryDB, closeMemoryDB, err := newInMemDB()
	if err != nil {
		return nil, nil, err
	}

	// The repository and the filesql adapter are two views of that same
	// database, and they have to stay that way. filesql loads files straight
	// into the connection the repository then queries; hand either of them a
	// database of its own and an import would land somewhere no query looks.
	sqlite3Repository := memory.NewSQLite3Repository(memoryDB)
	fileSQLAdapter := newFileSQLAdapter(memoryDB)
	sqlite3Interactor := interactor.NewSQLite3Interactor(sqlite3Repository, interactor.NewSQL(), fileSQLAdapter)

	queryUsecase := interactor.NewQueryUsecase(sqlite3Interactor)
	importUsecase := interactor.NewImportUsecase(sqlite3Interactor)
	metadataUsecase := interactor.NewMetadataUsecase(sqlite3Interactor)
	persistenceUsecase := interactor.NewPersistenceUsecase(sqlite3Interactor)

	// History is a file under the user's config directory, separate from the
	// session: it outlives the tables it was typed against. Nothing is held open
	// for it — each entry is one appending write — so there is no second resource
	// to release.
	historyUsecase := interactor.NewHistoryInteractor(persistence.NewHistoryRepository(cfg.HistoryPath))

	exportUsecase := interactor.NewExportInteractor()

	usecases := shell.NewUsecases(
		queryUsecase,
		importUsecase,
		metadataUsecase,
		historyUsecase,
		exportUsecase,
		persistenceUsecase,
	)

	sqlyShell, err := newSqlyShell(arg, cfg, commands, usecases)
	if err != nil {
		closeMemoryDB()
		return nil, nil, err
	}

	return sqlyShell, closeMemoryDB, nil
}

// newFileSQLAdapter points filesql at the session database. The conversion is
// the whole job: config.MemoryDB is a named *sql.DB that exists so a database
// cannot be passed where the other one was meant.
func newFileSQLAdapter(db config.MemoryDB) *filesql.FileSQLAdapter {
	return filesql.NewFileSQLAdapter((*sql.DB)(db))
}
