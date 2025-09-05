package shell

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/c-bata/go-prompt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/golden"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/interactor"
)

func TestShellRun(t *testing.T) {
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

		got := getStdoutForRunFunc(t, shell.Run)

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

		got := getStdoutForRunFunc(t, shell.Run)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "help", got)
	})

	t.Run("SELECT * FROM actor ORDER BY actor ASC LIMIT 5", func(t *testing.T) {
		config.Version = "(devel)"
		defer func() {
			config.Version = ""
		}()
		shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "SELECT actor, printf('%.2f', total_gross) as total_gross, number_of_movies, printf('%.2f', average_per_movie) as average_per_movie, best_movie, printf('%.2f', gross) as gross FROM actor ORDER BY actor ASC LIMIT 5", filepath.Join("testdata", "actor.csv")})
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
		config.Version = "(devel)"
		defer func() {
			config.Version = ""
		}()

		file := filepath.Join(t.TempDir(), "dump.csv")
		shell, cleanup, err := newShell(t, []string{"sqly", "--output", file, "--sql", "SELECT * FROM sample", filepath.Join("testdata", "sample.csv")})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.Run(context.Background()); err != nil {
			t.Fatal(err)
		}

		// TODO:
		got, err := os.ReadFile(filepath.Clean(file))
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			t.Fatal(err)
		}

		shell.completer(context.Background(), *prompt.NewDocument())
	})
}

func TestShellExec(t *testing.T) {
	t.Run("execute .tables", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
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

		if shell.state.mode.PrintMode != model.PrintModeTable {
			t.Errorf("mismatch got=%s, want=%s", shell.state.mode.String(), model.PrintModeTable.String())
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

		if shell.state.mode.PrintMode != model.PrintModeCSV {
			t.Errorf("mismatch got=%s, want=%s", shell.state.mode.String(), model.PrintModeCSV.String())
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

		if shell.state.mode.PrintMode != model.PrintModeMarkdownTable {
			t.Errorf("mismatch got=%s, want=%s", shell.state.mode.String(), model.PrintModeMarkdownTable.String())
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

		if shell.state.mode.PrintMode != model.PrintModeTSV {
			t.Errorf("mismatch got=%s, want=%s", shell.state.mode.String(), model.PrintModeTSV.String())
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

		if shell.state.mode.PrintMode != model.PrintModeLTSV {
			t.Errorf("mismatch got=%s, want=%s", shell.state.mode.String(), model.PrintModeLTSV.String())
		}
	})

	t.Run("execute .mode: table to excel", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".mode excel")
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "mode_table_to_excel", got)

		if shell.state.mode.PrintMode != model.PrintModeExcel {
			t.Errorf("mismatch got=%s, want=%s", shell.state.mode.String(), model.PrintModeExcel.String())
		}
	})

	t.Run("execute .mode: table to same mode", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".mode table")
		if !strings.Contains(err.Error(), "already table mode") {
			t.Fatal(err)
		}
	})

	t.Run("execute .mode: table to invalid mode", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".mode not_exist_mode")
		if !strings.Contains(err.Error(), "invalid output mode: not_exist_mode") {
			t.Fatal(err)
		}
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

		got, err := getExecStdOutput(t, shell.exec, "SELECT id, first_name, last_name, phone_number, email, url, age, birth_day, password FROM sample")
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.csv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(filepath.Clean(file))
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.csv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			t.Fatal(err)
		}

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "dump_csv", got)
	})

	t.Run("execute .dump tsv (print tsv mode)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--tsv"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.tsv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(filepath.Clean(file))
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.ltsv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sample "+file)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(filepath.Clean(file))
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
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

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			t.Fatal(err)
		}
		got, err := getExecStdOutput(t, shell.exec, "SELECT actor, printf('%.2f', total_gross) as total_gross, number_of_movies, printf('%.2f', average_per_movie) as average_per_movie, best_movie, printf('%.2f', gross) as gross FROM actor ORDER BY actor ASC LIMIT 5")
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
	memoryDB, cleanup, err := config.NewInMemDB()
	if err != nil {
		return nil, nil, err
	}
	// Create filesql adapter for tests
	filesqlAdapter := filesql.NewFileSQLAdapter((*sql.DB)(memoryDB))
	csvInteractor := interactor.NewCSVInteractor(filesqlAdapter)
	tsvInteractor := interactor.NewTSVInteractor(filesqlAdapter)
	ltsvInteractor := interactor.NewLTSVInteractor(filesqlAdapter)
	excelInteractor := interactor.NewExcelInteractor(filesqlAdapter)

	// Use filesql-based sqlite3 repository and interactor for consistency
	sqlite3Repository := filesql.NewSQLite3Repository(filesqlAdapter)
	sql := interactor.NewSQL()
	sqLite3Interactor := interactor.NewSQLite3Interactor(sqlite3Repository, sql)

	historyDB, cleanup2, err := config.NewHistoryDB(configConfig)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	historyRepository := persistence.NewHistoryRepository(historyDB)
	historyInteractor := interactor.NewHistoryInteractor(historyRepository)
	usecases := NewUsecases(csvInteractor, tsvInteractor, ltsvInteractor, sqLite3Interactor, historyInteractor, excelInteractor)
	shellShell, err := NewShell(arg, configConfig, commandList, usecases)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return shellShell, func() {
		cleanup2()
		cleanup()
	}, nil
}

func getStdoutForRunFunc(t *testing.T, f func(ctx context.Context) error) []byte {
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

	if err := f(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.Close() //nolint:gosec // Test cleanup, error not critical for test execution

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
	w.Close() //nolint:gosec // Test cleanup, error not critical for test execution

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes()
}

func getExecStdOutput(t *testing.T, f func(context.Context, string) error, arg string) ([]byte, error) {
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

	execErr := f(context.Background(), arg)
	w.Close() //nolint:gosec // Test cleanup, error not critical for test execution

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes(), execErr
}

func TestTrimGaps(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Before:' Hello,    World  ! ', After:'Hello, World !'",
			args: args{
				" Hello,    World  ! ",
			},
			want: "Hello, World !",
		},
		{
			name: "Before:' Hello,    World  ! ', After:'Hello, World !'",
			args: args{
				" Hello,    World  ! ",
			},
			want: "Hello, World !",
		},
		{
			name: "Before:' \t\n\t Hello, \n\t World \n ! \n\t ', After:'Hello, World !'",
			args: args{
				" \t\n\t Hello, \n\t World \n ! \n\t ",
			},
			want: "Hello, World !",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimGaps(tt.args.s); got != tt.want {
				t.Errorf("TrimGaps() = %v, want %v", got, tt.want)
			}
		})
	}
}
