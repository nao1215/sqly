package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/c-bata/go-prompt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/gorky/golden"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/memory"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/usecase"
)

func TestShell_Run(t *testing.T) {
	t.Run("print version", func(t *testing.T) {
		config.Version = "(devel)" //nolint
		defer func() {
			config.Version = ""
		}()
		shell, cleanup, err := newShell(t, []string{"sqly", "--version"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got := getStdoutForRunFunc(t, shell.Run)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "version", got)
	})

	t.Run("print help", func(t *testing.T) {
		config.Version = "(devel)" //nolint
		defer func() {
			config.Version = ""
		}()
		shell, cleanup, err := newShell(t, []string{"sqly", "--help"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got := getStdoutForRunFunc(t, shell.Run)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "help", got)
	})

	t.Run("SELECT * FROM actor ORDER BY actor ASC LIMIT 5", func(t *testing.T) {
		config.Version = "(devel)" //nolint
		defer func() {
			config.Version = ""
		}()
		shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "SELECT * FROM actor ORDER BY actor ASC LIMIT 5", filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got := getStdoutForRunFunc(t, shell.Run)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_asc_limit5_table", got)
	})

	t.Run("execute sql and output result to file", func(t *testing.T) {
		config.Version = "(devel)" //nolint
		defer func() {
			config.Version = ""
		}()

		file := filepath.Join(t.TempDir(), "dump.csv")
		shell, cleanup, err := newShell(t, []string{"sqly", "--output", file, "--sql", "SELECT * FROM sample", filepath.Join("testdata", "sample.csv")})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.Run(); err != nil {
			t.Fatal(err)
		}

		// TODO:
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_csv", got)
	})
}

func TestShell_printWelcomeMessage(t *testing.T) {
	t.Run("check welcome message", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got := getStdout(t, shell.printWelcomeMessage)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "welcome", got)
	})
}

func TestShell_completer(t *testing.T) {
	t.Run("set completer(this is not test. just for coverage)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			t.Fatal(err)
		}

		shell.completer(*prompt.NewDocument())
	})
}

func TestShell_exec(t *testing.T) {
	t.Run("execute .tables", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			t.Fatal(err)
		}
		got, err := getExecStdOutput(t, shell.exec, ".tables")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "tables", got)
	})

	t.Run("execute .tables: if no table exist", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".tables")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "no_table_exist", got)
	})

	t.Run("execute .header", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			t.Fatal(err)
		}
		got, err := getExecStdOutput(t, shell.exec, ".header actor")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "header", got)
	})

	t.Run("execute .header: if not specify table name", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			t.Fatal(err)
		}
		got, err := getExecStdOutput(t, shell.exec, ".header")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "header_not_specify_table", got)
	})

	t.Run("execute .mode: csv to table", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode table")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_csv_to_table", got)

		if shell.argument.Output.Mode != model.PrintModeTable {
			t.Errorf("mismatch got=%s, want=%s", shell.argument.Output.Mode.String(), model.PrintModeTable.String())
		}
	})

	t.Run("execute .mode: table to csv", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode csv")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_csv", got)

		if shell.argument.Output.Mode != model.PrintModeCSV {
			t.Errorf("mismatch got=%s, want=%s", shell.argument.Output.Mode.String(), model.PrintModeCSV.String())
		}
	})

	t.Run("execute .mode: table to markdown", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode markdown")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_markdown", got)

		if shell.argument.Output.Mode != model.PrintModeMarkdownTable {
			t.Errorf("mismatch got=%s, want=%s", shell.argument.Output.Mode.String(), model.PrintModeMarkdownTable.String())
		}
	})

	t.Run("execute .mode: table to tsv", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode tsv")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_tsv", got)

		if shell.argument.Output.Mode != model.PrintModeTSV {
			t.Errorf("mismatch got=%s, want=%s", shell.argument.Output.Mode.String(), model.PrintModeTSV.String())
		}
	})

	t.Run("execute .mode: table to ltsv", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode ltsv")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_ltsv", got)

		if shell.argument.Output.Mode != model.PrintModeLTSV {
			t.Errorf("mismatch got=%s, want=%s", shell.argument.Output.Mode.String(), model.PrintModeLTSV.String())
		}
	})

	t.Run("execute .mode: table to json", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode json")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_json", got)

		if shell.argument.Output.Mode != model.PrintModeJSON {
			t.Errorf("mismatch got=%s, want=%s", shell.argument.Output.Mode.String(), model.PrintModeJSON.String())
		}
	})

	t.Run("execute .mode: table to same mode", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode table")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_same_mode", got)
	})

	t.Run("execute .mode: table to invalid mode", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode not_exist_mode")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_not_exist_mode", got)
	})

	t.Run("execute .mode: if not specify mode name", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_without_arg", got)
	})

	t.Run("execute .help", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".help")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "help_command", got)
	})

	t.Run("execute .import csv", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".import "+filepath.Join("testdata", "sample.csv"))
		if err != nil {
			t.Fatal(err)
		}

		got, err := getExecStdOutput(t, shell.exec, "SELECT * FROM sample")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "import_csv", got)
	})

	t.Run("execute .import json", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".import "+filepath.Join("testdata", "sample.json"))
		if err != nil {
			t.Fatal(err)
		}

		got, err := getExecStdOutput(t, shell.exec, "SELECT * FROM sample")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "import_json", got)
	})

	t.Run("execute .import tsv", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".import "+filepath.Join("testdata", "sample.tsv"))
		if err != nil {
			t.Fatal(err)
		}

		got, err := getExecStdOutput(t, shell.exec, "SELECT * FROM sample")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "import_tsv", got)
	})

	t.Run("execute .import ltsv", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".import "+filepath.Join("testdata", "sample.ltsv"))
		if err != nil {
			t.Fatal(err)
		}

		got, err := getExecStdOutput(t, shell.exec, "SELECT * FROM sample")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "import_ltsv", got)
	})

	t.Run("execute .import without argument", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".import")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "import_without_arg", got)
	})

	t.Run("execute .import not support file", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".import "+filepath.Join("testdata", "sample.not_support"))
		if err == nil {
			t.Fatal("expect cause error, however import command return nil")
		}
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "import_not_support_file_format", []byte(err.Error()))
	})

	t.Run("execute .dump csv (print table mode)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.csv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_csv", got)
	})

	t.Run("execute .dump csv (print csv mode)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.csv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_csv", got)
	})

	t.Run("execute .dump json (print json mode)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--json"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.json")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_json", got)
	})

	t.Run("execute .dump tsv (print tsv mode)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--tsv"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.tsv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_tsv", got)
	})

	t.Run("execute .dump ltsv (print ltsv mode)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--ltsv"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.ltsv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_ltsv", got)
	})

	t.Run("execute .dump with few argument", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		got, err := getExecStdOutput(t, shell.exec, ".dump sample")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_with_few_arg", got)
	})

	t.Run("execute .dump not exist table", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, got := getExecStdOutput(t, shell.exec, ".dump not_exist_table dummy.csv")
		if got == nil {
			t.Fatal("execute .dump with bad argument(=not exist table name), however return nil")
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_not_exist_table", []byte(got.Error()))
	})

	t.Run("execute sql", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			t.Fatal(err)
		}
		got, err := getExecStdOutput(t, shell.exec, "SELECT * FROM actor ORDER BY actor ASC LIMIT 5")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_asc_limit5_table", got)
	})

	t.Run("bad argument", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, "bad argument")
		if err == nil {
			t.Errorf("expect error, however execute sql result is nil")
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "bad_arg", []byte(err.Error()))
	})

	t.Run("bad argument with dot prefix", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".bad argument")
		if err == nil {
			t.Errorf("expect error, however execute sql result is nil")
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "bad_arg_with_dot_prefix", []byte(err.Error()))
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
	excelRepository := persistence.NewExcelRepository()
	excelInteractor := usecase.NewExcelInteractor(excelRepository)
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
	shellShell := NewShell(arg, configConfig, commandList, csvInteractor, tsvInteractor, ltsvInteractor, jsonInteractor, sqLite3Interactor, historyInteractor, excelInteractor)
	return shellShell, func() {
		cleanup2()
		cleanup()
	}, nil
}

func getStdoutForRunFunc(t *testing.T, f func() error) []byte {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	config.Stdout = w

	if err := f(); err != nil {
		t.Fatal(err)
	}
	w.Close() //nolint

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes()
}

func getStdout(t *testing.T, f func()) []byte {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	config.Stdout = w

	f()
	w.Close() //nolint

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes()
}

func getExecStdOutput(t *testing.T, f func(string) error, arg string) ([]byte, error) {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	config.Stdout = w

	execErr := f(arg)
	w.Close() //nolint

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes(), execErr
}
