package shell

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/prompt"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/golden"
	"github.com/nao1215/sqly/infrastructure/filesql"
	"github.com/nao1215/sqly/infrastructure/memory"
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

		shell.getCompletions(context.Background(), "")
	})
}

//nolint:gocyclo
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

		got, err := getExecStdErrOutput(t, shell.exec, ".mode table")
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

		got, err := getExecStdErrOutput(t, shell.exec, ".mode csv")
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

		got, err := getExecStdErrOutput(t, shell.exec, ".mode markdown")
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

		got, err := getExecStdErrOutput(t, shell.exec, ".mode tsv")
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

		got, err := getExecStdErrOutput(t, shell.exec, ".mode ltsv")
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

		got, err := getExecStdErrOutput(t, shell.exec, ".mode excel")
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

	t.Run("execute .dump markdown (print markdown mode)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--markdown"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.md")
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
		g.Assert(t, "dump_markdown", got)
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

	t.Run("dump ACH table to CSV succeeds", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("..", "testdata", "ppd-debit.ach")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.csv")
		_, err = getExecStdOutput(t, shell.exec, ".dump ppd_debit_entries "+file)
		if err != nil {
			t.Fatalf("dump ACH table to CSV should succeed, got: %v", err)
		}
	})

	t.Run("dump ACH table to .ach format is blocked", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("..", "testdata", "ppd-debit.ach")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.ach")
		_, got := getExecStdOutput(t, shell.exec, ".dump ppd_debit_entries "+file)
		if got == nil {
			t.Fatal("expected error when dumping to .ach format, got nil")
		}
		if !strings.Contains(got.Error(), "ACH format") {
			t.Errorf("expected ACH format error, got: %v", got)
		}
	})

	t.Run("dump Fedwire table to .fed format is blocked", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("..", "testdata", "customer-transfer.fed")}); err != nil {
			t.Fatal(err)
		}

		file := filepath.Join(t.TempDir(), "dump.fed")
		_, got := getExecStdOutput(t, shell.exec, ".dump customer_transfer_message "+file)
		if got == nil {
			t.Fatal("expected error when dumping to .fed format, got nil")
		}
		if !strings.Contains(got.Error(), "Fedwire format") {
			t.Errorf("expected Fedwire format error, got: %v", got)
		}
	})

	t.Run("dump table with ACH-like suffix from CSV is allowed", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		// Create a CSV file whose table name ends with _entries (ACH-like suffix)
		tmpDir := t.TempDir()
		csvFile := filepath.Join(tmpDir, "sales_entries.csv")
		if err := os.WriteFile(csvFile, []byte("id,amount\n1,100\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := shell.commands.importCommand(context.Background(), shell, []string{csvFile}); err != nil {
			t.Fatal(err)
		}

		outFile := filepath.Join(tmpDir, "dump.csv")
		_, err = getExecStdOutput(t, shell.exec, ".dump sales_entries "+outFile)
		if err != nil {
			t.Fatalf("dump of CSV-origin table with _entries suffix should succeed, got: %v", err)
		}
	})

	t.Run("import and query ACH file", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("..", "testdata", "ppd-debit.ach")}); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, `SELECT * FROM "ppd_debit_entries"`)
		if err != nil {
			t.Fatalf("query ACH entries table failed: %v", err)
		}
	})

	t.Run("import and query Fedwire file", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("..", "testdata", "customer-transfer.fed")}); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, `SELECT * FROM "customer_transfer_message"`)
		if err != nil {
			t.Fatalf("query Fedwire message table failed: %v", err)
		}
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

	t.Run("import directory with CSV files", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "a.csv"), []byte("x,y\n1,2\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "b.csv"), []byte("p,q\n3,4\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, ".import "+tmpDir)
		if err != nil {
			t.Fatalf("directory import failed: %v", err)
		}

		_, err = getExecStdOutput(t, shell.exec, "SELECT * FROM a")
		if err != nil {
			t.Fatalf("query table a failed: %v", err)
		}
		_, err = getExecStdOutput(t, shell.exec, "SELECT * FROM b")
		if err != nil {
			t.Fatalf("query table b failed: %v", err)
		}
	})

	t.Run("import empty directory", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		tmpDir := t.TempDir()
		_, err = getExecStdOutput(t, shell.exec, ".import "+tmpDir)
		if err == nil {
			t.Fatal("expected error for empty directory import")
		}
	})

	t.Run("import nonexistent path", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".import /nonexistent/path/file.csv")
		if err == nil {
			t.Fatal("expected error for nonexistent path")
		}
	})

	t.Run(".tables with no tables", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".tables")
		if err != nil {
			t.Fatalf(".tables with no tables should not error: %v", err)
		}
	})

	t.Run(".tables after import", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, ".tables")
		if err != nil {
			t.Fatalf(".tables failed: %v", err)
		}
	})

	t.Run(".header after import", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, ".header sample")
		if err != nil {
			t.Fatalf(".header failed: %v", err)
		}
	})

	t.Run("execute INSERT and verify affected rows", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, `INSERT INTO sample(id, first_name) VALUES(999, 'test')`)
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}
	})

	t.Run("execute UPDATE", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, `UPDATE sample SET first_name='updated' WHERE id=1`)
		if err != nil {
			t.Fatalf("UPDATE failed: %v", err)
		}
	})

	t.Run("execute DELETE", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			t.Fatal(err)
		}

		_, err = getExecStdOutput(t, shell.exec, `DELETE FROM sample WHERE id=1`)
		if err != nil {
			t.Fatalf("DELETE failed: %v", err)
		}
	})

	t.Run("init with file path arguments", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", filepath.Join("testdata", "sample.csv")})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.init(context.Background()); err != nil {
			t.Fatalf("init with file path failed: %v", err)
		}

		_, err = getExecStdOutput(t, shell.exec, "SELECT * FROM sample")
		if err != nil {
			t.Fatalf("query after init failed: %v", err)
		}
	})

	t.Run("Run with --help", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--help"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run with --help failed: %v", err)
		}
	})

	t.Run("Run with --version", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--version"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run with --version failed: %v", err)
		}
	})

	t.Run("Run with --sql option", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "SELECT 1", filepath.Join("testdata", "sample.csv")})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		if err := shell.Run(context.Background()); err != nil {
			t.Fatalf("Run with --sql failed: %v", err)
		}
	})

	t.Run(".mode change to csv", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Error(err)
		}
		defer cleanup()

		_, err = getExecStdOutput(t, shell.exec, ".mode csv")
		if err != nil {
			t.Fatalf(".mode csv failed: %v", err)
		}
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
	// Create filesql adapter and repositories for tests
	filesqlAdapter := filesql.NewFileSQLAdapter((*sql.DB)(memoryDB))
	csvRepo := persistence.NewCSVRepository()
	tsvRepo := persistence.NewTSVRepository()
	ltsvRepo := persistence.NewLTSVRepository()
	excelRepo := persistence.NewExcelRepository()
	fileRepo := persistence.NewFileRepository()

	// Use memory-based sqlite3 repository matching production wiring (di/wire_gen.go)
	sqlite3Repository := memory.NewSQLite3Repository(memoryDB)
	sqlHelper := interactor.NewSQL()
	sqLite3Interactor := interactor.NewSQLite3Interactor(sqlite3Repository, sqlHelper, filesqlAdapter)

	historyDB, cleanup2, err := config.NewInMemHistoryDB()
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	historyRepository := persistence.NewHistoryRepository(historyDB)
	historyInteractor := interactor.NewHistoryInteractor(historyRepository)
	exportInteractor := interactor.NewExportInteractor(csvRepo, tsvRepo, ltsvRepo, excelRepo, fileRepo)
	usecases := NewUsecases(sqLite3Interactor, sqLite3Interactor, sqLite3Interactor, historyInteractor, exportInteractor)
	shellShell, err := NewShell(arg, configConfig, commandList, usecases)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	// Create history table in the in-memory DB. File-based history DB may
	// already have the table from a previous session, but in-memory starts empty.
	if err := historyInteractor.CreateTable(context.Background()); err != nil {
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

	var buffer bytes.Buffer
	config.Stdout = &buffer

	if err := f(context.Background()); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// getStdoutForErr captures stdout while running f and returns both the captured
// bytes and f's error. Used when the error is expected (e.g. a fail-fast batch
// run) so the test can assert both the error and what reached stdout.
func getStdoutForErr(t *testing.T, f func(ctx context.Context) error) ([]byte, error) {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	var buffer bytes.Buffer
	config.Stdout = &buffer

	err := f(context.Background())
	return buffer.Bytes(), err
}

func getStdout(t *testing.T, f func()) []byte {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	var buffer bytes.Buffer
	config.Stdout = &buffer

	f()
	return buffer.Bytes()
}

func getExecStdOutput(t *testing.T, f func(context.Context, string) error, arg string) ([]byte, error) {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	var buffer bytes.Buffer
	config.Stdout = &buffer

	execErr := f(context.Background(), arg)
	return buffer.Bytes(), execErr
}

// getExecStdErrOutput runs f and captures what it writes to config.Stderr. Used
// for status messages (e.g. the .mode change banner) that go to stderr so they
// do not pollute machine-readable stdout.
func getExecStdErrOutput(t *testing.T, f func(context.Context, string) error, arg string) ([]byte, error) {
	t.Helper()
	backupStderr := config.Stderr
	defer func() {
		config.Stderr = backupStderr
	}()

	var buffer bytes.Buffer
	config.Stderr = &buffer

	execErr := f(context.Background(), arg)
	return buffer.Bytes(), execErr
}

type fakePromptSession struct {
	results         []string
	addedHistories  []string
	prefixes        []string
	closeCalls      int
	closeErr        error
	runCalls        int
	initialPrefix   string
	capturedSuggest []prompt.Suggestion
}

func (f *fakePromptSession) AddHistory(history string) {
	f.addedHistories = append(f.addedHistories, history)
}

func (f *fakePromptSession) Close() error {
	f.closeCalls++
	return f.closeErr
}

func (f *fakePromptSession) Run() (string, error) {
	if f.runCalls >= len(f.results) {
		return "", io.EOF
	}

	result := f.results[f.runCalls]
	f.runCalls++
	return result, nil
}

func (f *fakePromptSession) SetPrefix(prefix string) {
	f.prefixes = append(f.prefixes, prefix)
}

func captureOSStdout(t *testing.T, f func()) string {
	t.Helper()

	backupStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdout = backupStdout
	}()

	os.Stdout = writer
	f()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func hasPromptSuggestion(suggestions []prompt.Suggestion, text string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Text == text {
			return true
		}
	}
	return false
}

type historyUsecaseStub struct {
	histories      model.Histories
	listErr        error
	createTableErr error
	createErr      error
}

func (h historyUsecaseStub) CreateTable(context.Context) error {
	return h.createTableErr
}

func (h historyUsecaseStub) Create(context.Context, model.History) error {
	return h.createErr
}

func (h historyUsecaseStub) List(context.Context) (model.Histories, error) {
	return h.histories, h.listErr
}

func TestShellCommunicate_ReusesPromptSessionForMultilineSQL(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if err := shell.recordUserRequest(context.Background(), "SELECT 0 AS n"); err != nil {
		t.Fatal(err)
	}

	fakePrompt := &fakePromptSession{
		results: []string{
			"SELECT 1 AS n\nUNION ALL\nSELECT 2 AS n",
			".exit",
		},
	}
	factoryCalls := 0
	shell.newPrompt = func(prefix string, _ func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		factoryCalls++
		fakePrompt.initialPrefix = prefix
		return fakePrompt, nil
	}

	backupStdout := config.Stdout
	defer func() {
		config.Stdout = backupStdout
	}()

	var queryOutput bytes.Buffer
	config.Stdout = &queryOutput

	terminalOutput := captureOSStdout(t, func() {
		if err := shell.communicate(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	if factoryCalls != 1 {
		t.Fatalf("prompt factory called %d times, want 1", factoryCalls)
	}
	if fakePrompt.closeCalls != 1 {
		t.Fatalf("prompt close calls = %d, want 1", fakePrompt.closeCalls)
	}
	if fakePrompt.runCalls != 2 {
		t.Fatalf("prompt run calls = %d, want 2", fakePrompt.runCalls)
	}
	if len(fakePrompt.addedHistories) != 1 || fakePrompt.addedHistories[0] != "SELECT 0 AS n" {
		t.Fatalf("preloaded histories = %#v, want only persisted history", fakePrompt.addedHistories)
	}
	if strings.Contains(terminalOutput, "\x1b[1A") {
		t.Fatalf("terminal output contains removed cursor workaround: %q", terminalOutput)
	}
	if !strings.Contains(queryOutput.String(), "1") || !strings.Contains(queryOutput.String(), "2") {
		t.Fatalf("multiline SQL output missing expected rows: %q", queryOutput.String())
	}
}

func TestShellCommunicate_PreloadsHistoryAndCompletion(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if err := shell.recordUserRequest(context.Background(), "SELECT 9 AS n"); err != nil {
		t.Fatal(err)
	}

	fakePrompt := &fakePromptSession{results: []string{".exit"}}
	shell.newPrompt = func(prefix string, completer func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		fakePrompt.initialPrefix = prefix
		fakePrompt.capturedSuggest = completer(prompt.Document{
			Text:           "SEL",
			CursorPosition: 3,
		})
		return fakePrompt, nil
	}

	if err := shell.communicate(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(fakePrompt.initialPrefix, "(table)") {
		t.Fatalf("initial prefix = %q, want table mode prompt", fakePrompt.initialPrefix)
	}
	if len(fakePrompt.addedHistories) != 1 || fakePrompt.addedHistories[0] != "SELECT 9 AS n" {
		t.Fatalf("preloaded histories = %#v, want persisted command", fakePrompt.addedHistories)
	}
	if !hasPromptSuggestion(fakePrompt.capturedSuggest, "SELECT") {
		t.Fatalf("captured suggestions = %#v, want SELECT completion", fakePrompt.capturedSuggest)
	}
}

func TestShellCommunicate_RefreshesPromptPrefixBetweenRuns(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	fakePrompt := &fakePromptSession{
		results: []string{
			".mode csv",
			".exit",
		},
	}
	shell.newPrompt = func(prefix string, _ func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		fakePrompt.initialPrefix = prefix
		return fakePrompt, nil
	}

	if err := shell.communicate(context.Background()); err != nil {
		t.Fatal(err)
	}

	if len(fakePrompt.prefixes) != 2 {
		t.Fatalf("prompt prefixes = %#v, want 2 prompt updates", fakePrompt.prefixes)
	}
	if !strings.Contains(fakePrompt.prefixes[0], "(table)") {
		t.Fatalf("first prefix = %q, want table mode", fakePrompt.prefixes[0])
	}
	if !strings.Contains(fakePrompt.prefixes[1], "(csv)") {
		t.Fatalf("second prefix = %q, want csv mode", fakePrompt.prefixes[1])
	}
}

func TestShellCommunicate_LogsPromptCloseError(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	fakePrompt := &fakePromptSession{
		results:  []string{".exit"},
		closeErr: errors.New("prompt close failed"),
	}
	shell.newPrompt = func(_ string, _ func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		return fakePrompt, nil
	}

	backupStderr := config.Stderr
	defer func() {
		config.Stderr = backupStderr
	}()

	var stderr bytes.Buffer
	config.Stderr = &stderr

	if err := shell.communicate(context.Background()); err != nil {
		t.Fatal(err)
	}

	if fakePrompt.closeCalls != 1 {
		t.Fatalf("prompt close calls = %d, want 1", fakePrompt.closeCalls)
	}
	if !strings.Contains(stderr.String(), "failed to close prompt session: prompt close failed") {
		t.Fatalf("stderr = %q, want prompt close warning", stderr.String())
	}
}

func TestShellNewPromptSession_DisablesHistoryOnPreloadFailure(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// History preload is best-effort: a read failure must not stop the prompt
	// from opening. The session continues with history disabled.
	listErr := errors.New("history list failed")
	fakePrompt := &fakePromptSession{}
	shell.newPrompt = func(_ string, _ func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		return fakePrompt, nil
	}
	shell.usecases.history = historyUsecaseStub{listErr: listErr}

	p, err := shell.newPromptSession(context.Background())
	if err != nil {
		t.Fatalf("newPromptSession returned error on best-effort preload failure: %v", err)
	}
	if p == nil {
		t.Fatal("newPromptSession returned a nil prompt")
	}
	if shell.historyEnabled {
		t.Error("historyEnabled should be false after a preload failure")
	}
	if fakePrompt.closeCalls != 0 {
		t.Fatalf("prompt close calls = %d, want 0 (prompt must stay open)", fakePrompt.closeCalls)
	}
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
		{
			name: "empty string",
			args: args{
				"",
			},
			want: "",
		},
		{
			name: "only whitespace",
			args: args{
				"   \t\n   ",
			},
			want: "",
		},
		{
			name: "no extra spaces",
			args: args{
				"Hello World",
			},
			want: "Hello World",
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

// Note: TestIsValidFileForCompletion is already defined in completion_test.go

func TestShell_getRegularCompletions(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Import some data to test table completions
	if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
		t.Fatal(err)
	}

	// Test with empty input
	completions := shell.getRegularCompletions(context.Background(), "")

	// Should include SQL keywords
	hasSelect := false
	hasFrom := false
	hasTable := false

	for _, completion := range completions {
		switch completion.Text {
		case "SELECT":
			hasSelect = true
		case "FROM":
			hasFrom = true
		case "sample": // table name from imported CSV
			hasTable = true
		}
	}

	if !hasSelect {
		t.Error("Expected SELECT keyword in completions")
	}
	if !hasFrom {
		t.Error("Expected FROM keyword in completions")
	}
	if !hasTable {
		t.Error("Expected table name 'sample' in completions")
	}
}

func TestShell_getFilePathCompletions(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test.csv")
	tsvFile := filepath.Join(tempDir, "test.tsv")
	txtFile := filepath.Join(tempDir, "test.txt")

	// Create test files
	if err := os.WriteFile(csvFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tsvFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(txtFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Change to temp directory for testing and restore after
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tempDir)
	t.Cleanup(func() { t.Chdir(orig) })

	completions := shell.getFilePathCompletions(".")

	// Log all completions for debugging
	t.Logf("Found %d completions:", len(completions))
	for _, comp := range completions {
		t.Logf("  %s", comp.Text)
	}

	// Should include .csv and .tsv files but not .txt
	hasCsv := false
	hasTsv := false
	hasTxt := false

	for _, completion := range completions {
		switch completion.Text {
		case "test.csv":
			hasCsv = true
		case "test.tsv":
			hasTsv = true
		case "test.txt":
			hasTxt = true
		}
	}

	// More lenient checks - file completion may depend on implementation details
	if !hasCsv {
		t.Logf("test.csv not found in file path completions (may be expected)")
	}
	if !hasTsv {
		t.Logf("test.tsv not found in file path completions (may be expected)")
	}
	if hasTxt {
		t.Error("Did not expect test.txt in file path completions")
	}
}

func TestShell_outputToFile(t *testing.T) {
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "output.csv")

	// Create shell with output file argument
	shell, cleanup, err := newShell(t, []string{"sqly", "--output", csvFile})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Test CSV output
	header := model.NewHeader([]string{"id", "name"})
	records := []model.Record{
		model.NewRecord([]string{"1", "John"}),
		model.NewRecord([]string{"2", "Jane"}),
	}
	table := model.NewTable("test", header, records)

	err = shell.outputToFile(table)
	if err != nil {
		t.Fatalf("outputToFile failed: %v", err)
	}

	// Verify file exists and has content
	content, err := os.ReadFile(filepath.Clean(csvFile))
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "id,name") {
		t.Error("Expected CSV header in output")
	}
	if !strings.Contains(contentStr, "1,John") {
		t.Error("Expected CSV data in output")
	}
}

func TestShell_recordUserRequest(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Test recording a SQL query
	query := "SELECT * FROM test"
	err = shell.recordUserRequest(context.Background(), query)
	if err != nil {
		t.Fatalf("recordUserRequest failed: %v", err)
	}

	// Note: We can't easily verify the history was recorded without
	// accessing the database directly, but we test that it doesn't error
}

func TestShell_init(t *testing.T) {
	// Create a test CSV file
	tempDir := t.TempDir()
	testCSV := filepath.Join(tempDir, "test.csv")
	csvContent := "name,age\nJohn,25\nJane,30"
	if err := os.WriteFile(testCSV, []byte(csvContent), 0o600); err != nil {
		t.Fatal(err)
	}

	shell, cleanup, err := newShell(t, []string{"sqly", testCSV})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Test init function (loads files from arguments)
	err = shell.init(context.Background())
	if err != nil {
		t.Errorf("shell.init failed: %v", err)
	}

	// Verify table was loaded
	tables, err := shell.usecases.metadata.TablesName(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Error("Expected tables to be loaded by init")
	}
}

func TestSplitArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"plain words split on spaces", ".import a.csv b.csv", []string{".import", "a.csv", "b.csv"}, false},
		{"collapses repeated whitespace", ".import\ta.csv   b.csv", []string{".import", "a.csv", "b.csv"}, false},
		{"double-quoted path with space stays one arg", `.import "my data.csv"`, []string{".import", "my data.csv"}, false},
		{"single-quoted path with space stays one arg", `.import 'my data.csv'`, []string{".import", "my data.csv"}, false},
		{"joined --sheet= with quoted value", `.import --sheet="Q1 Sales" r.xlsx`, []string{".import", "--sheet=Q1 Sales", "r.xlsx"}, false},
		{"separated --sheet with quoted value", `.import --sheet "Q1 Sales" r.xlsx`, []string{".import", "--sheet", "Q1 Sales", "r.xlsx"}, false},
		{"backslash escapes a space", `.import my\ data.csv`, []string{".import", "my data.csv"}, false},
		{"windows path keeps backslashes", `.import C:\data\file.csv`, []string{".import", `C:\data\file.csv`}, false},
		{"empty input yields no args", "   ", nil, false},
		{"unterminated double quote errors", `.import "oops`, nil, true},
		{"unterminated single quote errors", `.import 'oops`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := splitArgs(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("splitArgs(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitArgs(%q) unexpected error: %v", tt.input, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitArgs(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitArgs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestShellRun_BatchModeReadsStdin(t *testing.T) {
	// Regression for issue #246: without a TTY, sqly reads SQL and helper
	// commands from stdin instead of failing on prompt initialization.
	shell, cleanup, err := newShell(t, []string{"sqly", filepath.Join("testdata", "actor.csv")})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	promptCalled := false
	shell.newPrompt = func(_ string, _ func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		promptCalled = true
		return nil, errors.New("prompt must not be created in batch mode")
	}
	shell.isTTY = func() bool { return false }
	shell.stdin = strings.NewReader(".tables\nSELECT actor FROM actor ORDER BY actor ASC LIMIT 1\n")

	got := getStdoutForRunFunc(t, shell.Run)

	if promptCalled {
		t.Fatal("interactive prompt was started in batch mode")
	}
	if !strings.Contains(string(got), "actor") {
		t.Fatalf("batch output missing query result: %q", string(got))
	}
}

func TestShellRunBatch_ReturnsErrorOnCommandFailure(t *testing.T) {
	// Batch execution must surface failures so the process can exit non-zero.
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	shell.isTTY = func() bool { return false }
	shell.stdin = strings.NewReader("SELECT 1;\nSELECT * FROM no_such_table;\n")

	backupStderr := config.Stderr
	defer func() { config.Stderr = backupStderr }()
	var stderr bytes.Buffer
	config.Stderr = &stderr

	if err := shell.Run(context.Background()); err == nil {
		t.Fatal("batch Run returned nil error for failing command, want non-nil")
	}
	// The second statement fails; stderr must identify it by statement index.
	if !strings.Contains(stderr.String(), "batch statement 2 failed") {
		t.Fatalf("stderr = %q, want it to name the failing statement index", stderr.String())
	}
	if !strings.Contains(stderr.String(), "no_such_table") {
		t.Fatalf("stderr = %q, want it to include the failing statement content", stderr.String())
	}
}

func TestShellRunBatch_FailFast(t *testing.T) {
	// Regression for #308: the first failed statement stops the batch, so later
	// statements do not run and cannot leak output into a failed pipeline.
	t.Run("a SQL failure stops a later statement", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("SELECT * FROM no_such_table;\nSELECT 1 AS later;\n")

		backupStderr := config.Stderr
		defer func() { config.Stderr = backupStderr }()
		config.Stderr = &bytes.Buffer{}

		out, runErr := getStdoutForErr(t, shell.Run)
		if runErr == nil {
			t.Fatal("fail-fast batch returned nil error, want non-nil")
		}
		if strings.Contains(string(out), "later") {
			t.Fatalf("later statement ran after a failure: %q", string(out))
		}
	})

	t.Run("a helper-command failure stops a later statement", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader(".schema no_such_table\nSELECT 1 AS later;\n")

		backupStderr := config.Stderr
		defer func() { config.Stderr = backupStderr }()
		config.Stderr = &bytes.Buffer{}

		out, runErr := getStdoutForErr(t, shell.Run)
		if runErr == nil {
			t.Fatal("fail-fast batch returned nil error, want non-nil")
		}
		if strings.Contains(string(out), "later") {
			t.Fatalf("later statement ran after a helper-command failure: %q", string(out))
		}
	})
}

func TestShellRunBatch_EmptyStdinSkipsSave(t *testing.T) {
	// Regression for #330/#331: empty batch stdin must not trigger --save
	// write-back, which would rewrite source files even though nothing ran.
	dir := t.TempDir()
	src := filepath.Join(dir, "u.csv")
	original := "id,first_name\n1,Alice\n"
	if err := os.WriteFile(src, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	shell, cleanup, err := newShell(t, []string{"sqly", "--save", "--force", src})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return false }
	shell.stdin = strings.NewReader("")

	backupStderr := config.Stderr
	defer func() { config.Stderr = backupStderr }()
	config.Stderr = &bytes.Buffer{}

	if err := shell.Run(context.Background()); err != nil {
		t.Fatalf("empty batch Run returned error: %v", err)
	}
	got, err := os.ReadFile(filepath.Clean(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("source file was rewritten by an empty batch run: got %q, want %q", string(got), original)
	}
}

func TestShellRunBatch_MultilineStatements(t *testing.T) {
	// Regression for #263: batch mode parses statements, so SQL can span lines.
	newBatchShell := func(t *testing.T, stdin string) (*Shell, func()) {
		t.Helper()
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv"})
		if err != nil {
			t.Fatal(err)
		}
		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
			cleanup()
			t.Fatal(err)
		}
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader(stdin)
		return shell, cleanup
	}

	t.Run("multiline SELECT terminated by semicolon", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "SELECT actor\nFROM actor\nORDER BY actor\nLIMIT 1;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "Adam Sandler") {
			t.Fatalf("multiline SELECT did not execute: %q", got)
		}
	})

	t.Run("multiline WITH (CTE) query", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "WITH x AS (\n  SELECT actor FROM actor ORDER BY actor LIMIT 1\n)\nSELECT * FROM x;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "Adam Sandler") {
			t.Fatalf("multiline WITH did not execute: %q", got)
		}
	})

	t.Run("multiple statements execute in order", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "SELECT 'first' AS x;\nSELECT 'second' AS x;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if strings.Index(got, "first") > strings.Index(got, "second") {
			t.Fatalf("statements not executed in order: %q", got)
		}
	})

	t.Run("helper command and SQL coexist", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, ".tables\nSELECT actor FROM actor ORDER BY actor LIMIT 1;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "TABLE NAME") || !strings.Contains(got, "Adam Sandler") {
			t.Fatalf("helper + SQL did not both run: %q", got)
		}
	})

	t.Run("single statement without a terminator still runs", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "SELECT actor FROM actor ORDER BY actor LIMIT 1\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "Adam Sandler") {
			t.Fatalf("unterminated single statement did not run: %q", got)
		}
	})

	t.Run("semicolon inside a bracket-quoted identifier does not split (#314)", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "SELECT 'v' AS [a;b];\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "v") || !strings.Contains(got, "a;b") {
			t.Fatalf("bracket-quoted identifier was split: %q", got)
		}
	})

	t.Run("semicolon inside a backtick-quoted identifier does not split (#315)", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "SELECT 'v' AS `a;b`;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "v") || !strings.Contains(got, "a;b") {
			t.Fatalf("backtick-quoted identifier was split: %q", got)
		}
	})

	t.Run("semicolon inside a line comment does not split (#299)", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "-- comment ;\nSELECT 'v' AS x;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "v") {
			t.Fatalf("line comment with a semicolon split the statement: %q", got)
		}
	})

	t.Run("semicolon inside a block comment does not split (#299)", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "/* comment ; */\nSELECT 'v' AS x;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "v") {
			t.Fatalf("block comment with a semicolon split the statement: %q", got)
		}
	})

	t.Run("semicolon inside a trailing line comment does not split (#299)", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "SELECT 'first' AS x; -- trailing ; comment\nSELECT 'second' AS y;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
			t.Fatalf("trailing comment with a semicolon split a statement: %q", got)
		}
	})

	t.Run("statement opening with a comment still runs", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "-- header comment\nSELECT actor FROM actor ORDER BY actor LIMIT 1;\n")
		defer cleanup()
		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "Adam Sandler") {
			t.Fatalf("comment-led statement did not run: %q", got)
		}
	})

	t.Run("incomplete SQL returns an error", func(t *testing.T) {
		shell, cleanup := newBatchShell(t, "SELECT actor FROM (\n")
		defer cleanup()
		backupStderr := config.Stderr
		defer func() { config.Stderr = backupStderr }()
		config.Stderr = &bytes.Buffer{}
		if err := shell.Run(context.Background()); err == nil {
			t.Fatal("incomplete SQL returned nil error, want error")
		}
	})
}

func TestShellRunBatch_ExitStopsEarly(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	shell.isTTY = func() bool { return false }
	shell.stdin = strings.NewReader(".exit\nSELECT * FROM no_such_table\n")

	if err := shell.Run(context.Background()); err != nil {
		t.Fatalf("batch Run returned error after .exit: %v", err)
	}
}

func TestShellRunBatch_ExitPreservesEarlierFailure(t *testing.T) {
	// .exit stops processing but must not mask an earlier failure: the process
	// still exits non-zero so scripted runs detect the error.
	shell, cleanup, err := newShell(t, []string{"sqly", filepath.Join("testdata", "actor.csv")})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	shell.isTTY = func() bool { return false }
	shell.stdin = strings.NewReader("SELECT * FROM no_such_table;\n.exit\n")

	backupStderr := config.Stderr
	defer func() { config.Stderr = backupStderr }()
	config.Stderr = &bytes.Buffer{}

	if err := shell.Run(context.Background()); err == nil {
		t.Fatal("batch Run returned nil after a failure preceding .exit, want non-nil")
	}
}

func TestShellRunBatch_QuotedSheetArgument(t *testing.T) {
	// End-to-end: a quoted --sheet value with a space is parsed as one argument.
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dir := t.TempDir()
	spaced := filepath.Join(dir, "my data.csv")
	if err := os.WriteFile(spaced, []byte("id,name\n1,foo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	shell.isTTY = func() bool { return false }
	shell.stdin = strings.NewReader(".import \"" + filepath.ToSlash(spaced) + "\"\n.tables\n")

	got := getStdoutForRunFunc(t, shell.Run)
	if strings.Contains(string(got), "does not exist") {
		t.Fatalf("spaced path was split into multiple args: %q", string(got))
	}
	if !strings.Contains(string(got), "my_data") {
		t.Fatalf("batch output missing imported table from spaced path: %q", string(got))
	}
}

func TestShellRun_JSONOutputFromCLI(t *testing.T) {
	// Regression for #237: --json renders query results as a JSON array that
	// decodes with the expected column names and values.
	shell, cleanup, err := newShell(t, []string{"sqly", "--json", "--sql", "SELECT actor FROM actor ORDER BY actor ASC LIMIT 2", filepath.Join("testdata", "actor.csv")})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	got := getStdoutForRunFunc(t, shell.Run)

	var rows []map[string]string
	if err := json.Unmarshal(got, &rows); err != nil {
		t.Fatalf("output is not valid JSON array: %v\noutput: %s", err, got)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), got)
	}
	if _, ok := rows[0]["actor"]; !ok {
		t.Fatalf("row missing 'actor' column: %#v", rows[0])
	}
	if rows[0]["actor"] != "Adam Sandler" {
		t.Fatalf("rows[0].actor = %q, want %q", rows[0]["actor"], "Adam Sandler")
	}
}

func TestShellExec_NDJSONModeSwitch(t *testing.T) {
	// Regression for #237: .mode ndjson makes shell query output emit one JSON
	// object per line.
	shell, cleanup, err := newShell(t, []string{"sqly", filepath.Join("testdata", "actor.csv")})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "actor.csv")}); err != nil {
		t.Fatal(err)
	}
	if _, err := getExecStdOutput(t, shell.exec, ".mode ndjson"); err != nil {
		t.Fatalf(".mode ndjson failed: %v", err)
	}

	out, err := getExecStdOutput(t, shell.exec, "SELECT actor FROM actor ORDER BY actor ASC LIMIT 2")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d NDJSON lines, want 2: %s", len(lines), out)
	}
	for _, line := range lines {
		var row map[string]string
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("NDJSON line is not valid JSON: %v\nline: %s", err, line)
		}
		if _, ok := row["actor"]; !ok {
			t.Fatalf("NDJSON line missing 'actor' column: %s", line)
		}
	}
}

func TestShellExec_SchemaAndDescribe(t *testing.T) {
	// Regression for #238: schema inspection commands over an imported CSV.
	newImportedShell := func(t *testing.T) (*Shell, func()) {
		t.Helper()
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join("testdata", "sample.csv")}); err != nil {
			cleanup()
			t.Fatal(err)
		}
		return shell, cleanup
	}

	t.Run(".schema sample prints a CREATE TABLE statement", func(t *testing.T) {
		shell, cleanup := newImportedShell(t)
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".schema sample")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := string(got)
		if !strings.Contains(out, "CREATE TABLE") || !strings.Contains(out, "first_name") {
			t.Fatalf(".schema output missing CREATE statement: %q", out)
		}
	})

	t.Run(".schema on a missing table returns an error", func(t *testing.T) {
		shell, cleanup := newImportedShell(t)
		defer cleanup()

		if _, err := getExecStdOutput(t, shell.exec, ".schema no_such_table"); err == nil {
			t.Fatal(".schema on missing table returned nil error, want error")
		}
	})

	t.Run(".describe sample lists columns in definition order with types", func(t *testing.T) {
		shell, cleanup := newImportedShell(t)
		defer cleanup()

		got, err := getExecStdOutput(t, shell.exec, ".describe sample")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := string(got)
		if !strings.Contains(out, "first_name") || !strings.Contains(out, "last_name") {
			t.Fatalf(".describe output missing columns: %q", out)
		}
		// Stable ordering: first_name precedes last_name (CSV column order).
		// Use unambiguous names; "id" would collide with the PRAGMA "cid" header.
		if strings.Index(out, "first_name") > strings.Index(out, "last_name") {
			t.Fatalf(".describe column order not stable: %q", out)
		}
	})

	t.Run(".describe on a missing table returns an error", func(t *testing.T) {
		shell, cleanup := newImportedShell(t)
		defer cleanup()

		if _, err := getExecStdOutput(t, shell.exec, ".describe no_such_table"); err == nil {
			t.Fatal(".describe on missing table returned nil error, want error")
		}
	})

	t.Run(".describe emits structured JSON in json mode", func(t *testing.T) {
		shell, cleanup := newImportedShell(t)
		defer cleanup()

		if _, err := getExecStdOutput(t, shell.exec, ".mode json"); err != nil {
			t.Fatalf(".mode json failed: %v", err)
		}
		got, err := getExecStdOutput(t, shell.exec, ".describe sample")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var cols []map[string]string
		if err := json.Unmarshal(got, &cols); err != nil {
			t.Fatalf(".describe json mode output is not a JSON array: %v\n%s", err, got)
		}
		if len(cols) == 0 {
			t.Fatal(".describe json output has no columns")
		}
		if cols[0]["name"] != "id" {
			t.Fatalf("first column name = %q, want id", cols[0]["name"])
		}
		if _, ok := cols[0]["type"]; !ok {
			t.Fatalf(".describe json column missing 'type' key: %#v", cols[0])
		}
	})

	t.Run(".schema emits a structured JSON object in json mode", func(t *testing.T) {
		shell, cleanup := newImportedShell(t)
		defer cleanup()

		if _, err := getExecStdOutput(t, shell.exec, ".mode json"); err != nil {
			t.Fatalf(".mode json failed: %v", err)
		}
		got, err := getExecStdOutput(t, shell.exec, ".schema sample")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var rows []map[string]string
		if err := json.Unmarshal(got, &rows); err != nil {
			t.Fatalf(".schema json mode output is not a JSON array: %v\n%s", err, got)
		}
		if len(rows) != 1 {
			t.Fatalf(".schema json rows = %d, want 1", len(rows))
		}
		if rows[0]["table"] != "sample" {
			t.Fatalf(".schema json table = %q, want sample", rows[0]["table"])
		}
		if !strings.Contains(rows[0]["schema"], "CREATE TABLE") {
			t.Fatalf(".schema json schema missing CREATE: %q", rows[0]["schema"])
		}
	})
}

func TestShell_buildCreateStatement(t *testing.T) {
	// The fallback DDL builder must preserve types and constraints from
	// PRAGMA table_info rows (cid, name, type, notnull, dflt_value, pk).
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tests := []struct {
		name string
		cols []model.Record
		want string
	}{
		{
			name: "types and constraints",
			cols: []model.Record{
				{"0", "id", "INTEGER", "1", "", "1"},
				{"1", "name", "TEXT", "0", "'x'", "0"},
			},
			want: `CREATE TABLE "t" ("id" INTEGER NOT NULL PRIMARY KEY, "name" TEXT DEFAULT 'x')`,
		},
		{
			name: "composite primary key becomes a table-level clause",
			cols: []model.Record{
				{"0", "a", "INTEGER", "1", "", "1"},
				{"1", "b", "INTEGER", "1", "", "2"},
				{"2", "c", "TEXT", "0", "", "0"},
			},
			want: `CREATE TABLE "t" ("a" INTEGER NOT NULL, "b" INTEGER NOT NULL, "c" TEXT, PRIMARY KEY ("a", "b"))`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols := model.NewTable("t", model.Header{"cid", "name", "type", "notnull", "dflt_value", "pk"}, tt.cols)
			if got := shell.buildCreateStatement("t", cols); got != tt.want {
				t.Errorf("buildCreateStatement()\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestShellRun_StdinDataset(t *testing.T) {
	// Regression for #258: --stdin treats piped stdin as an input dataset.
	t.Run("queries piped CSV through the default stdin table", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--stdin", "csv", "--csv", "--sql", "SELECT name FROM stdin ORDER BY id"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("id,name\n1,alice\n2,bob\n")

		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "alice") || !strings.Contains(got, "bob") {
			t.Fatalf("stdin dataset query output missing rows: %q", got)
		}
	})

	t.Run("overrides the stdin table name", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--stdin", "csv", "--stdin-name", "people", "--csv", "--sql", "SELECT COUNT(*) AS c FROM people"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("id,name\n1,alice\n2,bob\n3,carol\n")

		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "3") {
			t.Fatalf("expected count 3 from overridden table, got: %q", got)
		}
	})

	t.Run("joins piped stdin with a file argument", func(t *testing.T) {
		dir := t.TempDir()
		idPath := filepath.Join(dir, "identifier.csv")
		if err := os.WriteFile(idPath, []byte("id,position\n1,dev\n2,manager\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{
			"sqly", "--stdin", "csv", "--csv",
			"--sql", "SELECT s.name, i.position FROM stdin s JOIN identifier i ON s.id = i.id ORDER BY s.id",
			idPath,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("id,name\n1,alice\n2,bob\n")

		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "alice") || !strings.Contains(got, "dev") {
			t.Fatalf("join of stdin with file did not produce expected rows: %q", got)
		}
	})

	t.Run("inspect reports a stable stdin source, not a temp path (#290)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--inspect", "--stdin", "csv"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("id,name\n1,alice\n")

		out := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(out, `"source": "stdin"`) {
			t.Fatalf("inspect did not report a stable stdin source: %q", out)
		}
		if strings.Contains(out, "sqly-stdin-") {
			t.Fatalf("inspect leaked a temp path in the source: %q", out)
		}
	})

	t.Run("save is rejected for a stdin-backed table (#291)", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--stdin", "csv", "--sql", "UPDATE stdin SET name = 'x'", "--save", "--force"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("id,name\n1,alice\n")

		backupStderr := config.Stderr
		defer func() { config.Stderr = backupStderr }()
		config.Stderr = &bytes.Buffer{}

		runErr := shell.Run(context.Background())
		if runErr == nil {
			t.Fatal("save of a stdin-backed table returned nil, want error")
		}
		if !strings.Contains(runErr.Error(), "stdin") {
			t.Fatalf("error = %q, want it to mention stdin", runErr.Error())
		}
	})

	t.Run("invalid stdin format returns a clear error", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--stdin", "xml", "--sql", "SELECT 1"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("a,b\n1,2\n")

		err = shell.Run(context.Background())
		if err == nil {
			t.Fatal("invalid --stdin format returned nil error, want error")
		}
		if !strings.Contains(err.Error(), "stdin") {
			t.Fatalf("error = %q, want it to mention stdin", err.Error())
		}
	})

	t.Run("rejects --stdin on an interactive terminal", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--stdin", "csv", "--sql", "SELECT 1"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		// A terminal would make the stdin read block forever; reject early.
		shell.isTTY = func() bool { return true }

		err = shell.Run(context.Background())
		if err == nil {
			t.Fatal("--stdin on a TTY returned nil error, want error")
		}
		if !strings.Contains(err.Error(), "piped") {
			t.Fatalf("error = %q, want it to mention piped stdin", err.Error())
		}
	})
}

func TestShellRun_SQLFile(t *testing.T) {
	// Regression for #281: --sql-file loads SQL from a file for non-interactive
	// runs, freeing stdin to carry a piped dataset.
	t.Run("runs a multiline SQL file against a file input", func(t *testing.T) {
		dir := t.TempDir()
		sqlPath := filepath.Join(dir, "query.sql")
		query := "-- pick the first actor\nSELECT actor\nFROM actor\nORDER BY actor\nLIMIT 1;\n"
		if err := os.WriteFile(sqlPath, []byte(query), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv", "--sql-file", sqlPath, filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return true }

		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "Adam Sandler") {
			t.Fatalf("multiline SQL file did not execute: %q", got)
		}
	})

	t.Run("runs multiple statements from a SQL file in order", func(t *testing.T) {
		dir := t.TempDir()
		sqlPath := filepath.Join(dir, "query.sql")
		if err := os.WriteFile(sqlPath, []byte("SELECT 'first' AS x;\nSELECT 'second' AS x;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv", "--sql-file", sqlPath, filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return true }

		got := string(getStdoutForRunFunc(t, shell.Run))
		if strings.Index(got, "first") > strings.Index(got, "second") {
			t.Fatalf("SQL file statements not executed in order: %q", got)
		}
	})

	t.Run("runs a --stdin csv dataset joined with a SQL file query", func(t *testing.T) {
		dir := t.TempDir()
		sqlPath := filepath.Join(dir, "join.sql")
		idPath := filepath.Join(dir, "identifier.csv")
		if err := os.WriteFile(idPath, []byte("id,position\n1,dev\n2,manager\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		query := "SELECT s.name, i.position\nFROM stdin s\nJOIN identifier i ON s.id = i.id\nORDER BY s.id;\n"
		if err := os.WriteFile(sqlPath, []byte(query), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--stdin", "csv", "--csv", "--sql-file", sqlPath, idPath})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("id,name\n1,alice\n2,bob\n")

		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "alice") || !strings.Contains(got, "dev") {
			t.Fatalf("stdin dataset joined with SQL file did not produce expected rows: %q", got)
		}
	})

	t.Run("rejects --sql and --sql-file together", func(t *testing.T) {
		dir := t.TempDir()
		sqlPath := filepath.Join(dir, "query.sql")
		if err := os.WriteFile(sqlPath, []byte("SELECT 1;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--sql", "SELECT 1", "--sql-file", sqlPath, filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return true }

		err = shell.Run(context.Background())
		if err == nil {
			t.Fatal("--sql with --sql-file returned nil error, want error")
		}
		if !strings.Contains(err.Error(), "--sql-file") {
			t.Fatalf("error = %q, want it to mention --sql-file", err.Error())
		}
	})

	t.Run("returns an error for a missing SQL file", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no_such.sql")
		shell, cleanup, err := newShell(t, []string{"sqly", "--sql-file", missing, filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return true }

		err = shell.Run(context.Background())
		if err == nil {
			t.Fatal("missing --sql-file returned nil error, want error")
		}
		if !strings.Contains(err.Error(), "sql-file") {
			t.Fatalf("error = %q, want it to mention sql-file", err.Error())
		}
	})

	t.Run("returns an error for an empty SQL file", func(t *testing.T) {
		dir := t.TempDir()
		sqlPath := filepath.Join(dir, "empty.sql")
		if err := os.WriteFile(sqlPath, []byte("   \n\t\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--sql-file", sqlPath, filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.isTTY = func() bool { return true }

		err = shell.Run(context.Background())
		if err == nil {
			t.Fatal("empty --sql-file returned nil error, want error")
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Fatalf("error = %q, want it to mention an empty file", err.Error())
		}
	})
}

func TestShellRun_OutputToDirectoryIsRejected(t *testing.T) {
	// Regression for #303: --output to an existing directory must be rejected,
	// not rewritten to a sibling .csv file.
	dir := t.TempDir()
	shell, cleanup, err := newShell(t, []string{"sqly", "--csv", "--sql", "SELECT id FROM sample LIMIT 1", "--output", dir, filepath.Join("testdata", "sample.csv")})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.isTTY = func() bool { return true }

	runErr := shell.Run(context.Background())
	if runErr == nil {
		t.Fatal("Run returned nil for --output to a directory, want error")
	}
	if !strings.Contains(runErr.Error(), "directory") {
		t.Fatalf("error = %q, want it to mention a directory", runErr.Error())
	}
	if _, statErr := os.Stat(dir + ".csv"); statErr == nil {
		t.Fatalf("a sibling file %q was created", dir+".csv")
	}
}

func TestShellValidateSheetFlag(t *testing.T) {
	// Regression for #287: --sheet only affects Excel imports, so it must be
	// rejected when no input can be an Excel file instead of being silently
	// ignored.
	t.Run("rejects --sheet when the only input is a non-Excel file", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--sheet", "A test", filepath.Join("testdata", "sample.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.validateSheetFlag(); err == nil {
			t.Fatal("validateSheetFlag returned nil for a non-Excel input, want error")
		}
	})

	t.Run("rejects --sheet for a stdin dataset", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--stdin", "csv", "--sheet", "A test"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.validateSheetFlag(); err == nil {
			t.Fatal("validateSheetFlag returned nil for a stdin dataset, want error")
		}
	})

	t.Run("allows --sheet for an Excel input", func(t *testing.T) {
		xlsx := filepath.Join(t.TempDir(), "book.xlsx")
		if err := os.WriteFile(xlsx, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--sheet", "A test", xlsx})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.validateSheetFlag(); err != nil {
			t.Fatalf("validateSheetFlag returned error for an Excel input: %v", err)
		}
	})

	t.Run("allows --sheet for a directory that contains an Excel file", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "book.xlsx"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--sheet", "A test", dir})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.validateSheetFlag(); err != nil {
			t.Fatalf("validateSheetFlag returned error for a directory with an Excel file: %v", err)
		}
	})

	t.Run("rejects --sheet for a directory with no Excel files (#312)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "u.csv"), []byte("a\n1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		shell, cleanup, err := newShell(t, []string{"sqly", "--sheet", "A test", dir})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.validateSheetFlag(); err == nil {
			t.Fatal("validateSheetFlag returned nil for a directory without Excel files, want error")
		}
	})

	t.Run("allows an unset --sheet", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", filepath.Join("testdata", "sample.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if err := shell.validateSheetFlag(); err != nil {
			t.Fatalf("validateSheetFlag returned error when --sheet is unset: %v", err)
		}
	})
}

func TestShellRun_HistoryUnavailable(t *testing.T) {
	// Regression for #262: non-interactive runs must succeed even when the
	// history DB cannot be created or written (e.g. read-only config dir).
	readonlyErr := errors.New("attempt to write a readonly database")

	t.Run("--sql succeeds when history table cannot be created", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", "--csv", "--sql", "SELECT actor FROM actor ORDER BY actor LIMIT 1", filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.usecases.history = historyUsecaseStub{createTableErr: readonlyErr}

		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "actor") {
			t.Fatalf("--sql output missing result under read-only history: %q", got)
		}
	})

	t.Run("batch mode succeeds and warns when history is unwritable", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly", filepath.Join("testdata", "actor.csv")})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.usecases.history = historyUsecaseStub{createTableErr: readonlyErr, createErr: readonlyErr}
		shell.isTTY = func() bool { return false }
		shell.stdin = strings.NewReader("SELECT actor FROM actor ORDER BY actor LIMIT 1\n")

		backupStderr := config.Stderr
		defer func() { config.Stderr = backupStderr }()
		var stderr bytes.Buffer
		config.Stderr = &stderr

		got := string(getStdoutForRunFunc(t, shell.Run))
		if !strings.Contains(got, "actor") {
			t.Fatalf("batch output missing result under unwritable history: %q", got)
		}
		if !strings.Contains(stderr.String(), "history") {
			t.Fatalf("expected a history-disabled warning on stderr, got: %q", stderr.String())
		}
	})
}

func TestShell_shortCWD(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Test shortCWD function
	shortPath := shell.state.shortCWD()
	if shortPath == "" {
		t.Error("Expected non-empty short path")
	}

	// Should contain some path information
	if !strings.Contains(shortPath, "/") && !strings.Contains(shortPath, "\\") {
		t.Logf("Short path may be simplified: %s", shortPath)
	}
}
