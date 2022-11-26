package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/golden"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/infrastructure/memory"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/usecase"
)

func TestShell_Run(t *testing.T) {
	t.Run("print version", func(t *testing.T) {
		config.Version = "(devel)"
		defer func() {
			config.Version = ""
		}()
		shell, cleanup, err := newShell(t, []string{"sqly", "--version"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getStdoutForRunFunc(t, shell.Run)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "version", got)
	})

	t.Run("print help", func(t *testing.T) {
		config.Version = "(devel)"
		defer func() {
			config.Version = ""
		}()
		shell, cleanup, err := newShell(t, []string{"sqly", "--help"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getStdoutForRunFunc(t, shell.Run)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "help", got)
	})

	t.Run("SELECT * FROM actor ORDER BY actor ASC LIMIT 5", func(t *testing.T) {
		config.Version = "(devel)"
		defer func() {
			config.Version = ""
		}()
		shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "SELECT * FROM actor ORDER BY actor ASC LIMIT 5", "testdata/actor.csv"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getStdoutForRunFunc(t, shell.Run)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_asc_limit5_table", got)
	})
}

func newShell(t *testing.T, args []string) (*Shell, func(), error) {
	t.Helper()
	arg, err := config.NewArg(args)
	if err != nil {
		return nil, nil, err
	}
	configConfig, err := config.NewConfig()
	if err != nil {
		return nil, nil, err
	}
	commandList := NewCommands()
	csvRepository := persistence.NewCSVRepository()
	csvInteractor := usecase.NewCSVInteractor(csvRepository)
	tsvRepository := persistence.NewTSVRepository()
	tsvInteractor := usecase.NewTSVInteractor(tsvRepository)
	ltsvRepository := persistence.NewLTSVRepository()
	ltsvInteractor := usecase.NewLTSVInteractor(ltsvRepository)
	jsonRepository := persistence.NewJSONRepository()
	jsonInteractor := usecase.NewJSONInteractor(jsonRepository)
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		return nil, nil, err
	}
	sqLite3Repository := memory.NewSQLite3Repository(memoryDB)
	sql := usecase.NewSQL()
	sqLite3Interactor := usecase.NewSQLite3Interactor(sqLite3Repository, sql)
	historyDB, cleanup2, err := config.NewHistoryDB(configConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	historyRepository := persistence.NewHistoryRepository(historyDB)
	historyInteractor := usecase.NewHistoryInteractor(historyRepository)
	shellShell := NewShell(arg, configConfig, commandList, csvInteractor, tsvInteractor, ltsvInteractor, jsonInteractor, sqLite3Interactor, historyInteractor)
	return shellShell, func() {
		cleanup2()
		cleanup()
	}, nil
}

func getStdoutForRunFunc(t *testing.T, f func() error) ([]byte, error) {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	r, w, _ := os.Pipe()
	config.Stdout = w

	if err := f(); err != nil {
		return nil, err
	}
	w.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes(), nil
}
