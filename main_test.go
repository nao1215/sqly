package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/testutil"
)

func TestMain(m *testing.M) {
	config.InitSQLite3()
	os.Exit(m.Run())
}

func Test_run(t *testing.T) {
	t.Run("show version message", func(t *testing.T) {
		args := []string{"sqly", "--version"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "version.golden", got)
	})

	t.Run("SELECT * FROM actor ORDER BY actor ASC LIMIT 5, print ascii table", func(t *testing.T) {
		args := []string{"sqly", "--sql", "SELECT actor, printf('%.2f', total_gross) as total_gross, number_of_movies, printf('%.2f', average_per_movie) as average_per_movie, best_movie, printf('%.2f', gross) as gross FROM actor ORDER BY actor ASC LIMIT 5", "testdata/actor.csv"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "select_asc_limit5_table.golden", got)
	})

	t.Run("sqly --sql 'SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id' testdata/user.csv testdata/identifier.csv", func(t *testing.T) {
		args := []string{"sqly", "--csv", "--sql", "SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id", "testdata/user.csv", "testdata/identifier.csv"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "select_inner_join_csv.golden", got)
	})

	t.Run("sqly --tsv --sql 'SELECT * FROM user' testdata/user.csv", func(t *testing.T) {
		args := []string{"sqly", "--tsv", "--sql", "SELECT * FROM user", "testdata/user.csv"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "select_tsv.golden", got)
	})

	t.Run("sqly --ltsv --sql 'SELECT * FROM user' testdata/user.csv", func(t *testing.T) {
		args := []string{"sqly", "--ltsv", "--sql", "SELECT * FROM user", "testdata/user.csv"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "select_ltsv.golden", got)
	})

	t.Run("import excel, output csv", func(t *testing.T) {
		args := []string{"sqly", "--sql", "SELECT * FROM sample_test_sheet", "-S", "test_sheet", "--csv", "testdata/sample.xlsx"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "excel_to_csv.golden", got)
	})

	t.Run("--sql-file runs a multiline query loaded from a file", func(t *testing.T) {
		sqlPath := filepath.Join(t.TempDir(), "query.sql")
		query := "-- top actor by name\nSELECT actor\nFROM actor\nORDER BY actor ASC\nLIMIT 1;\n"
		if err := os.WriteFile(sqlPath, []byte(query), 0o600); err != nil {
			t.Fatal(err)
		}
		args := []string{"sqly", "--csv", "--sql-file", sqlPath, "testdata/actor.csv"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "sql_file_multiline.golden", got)
	})

	t.Run("Treat numbers as numeric types; support numerical sorting", func(t *testing.T) {
		// SELECT * FROM numeric ORDER BY id
		// [Previously Result]
		// id,name
		// 1,John
		// 11,Ringo
		// 12,Billy
		// 2,Paul
		// 3,George
		//
		// [Current Result]
		// id,name
		// 1,John
		// 2,Paul
		// 3,George
		// 11,Ringo
		// 12,Billy

		args := []string{"sqly", "--sql", "SELECT * FROM numeric ORDER BY id", "--csv", "testdata/numeric.csv"}
		got := getStdoutForRunFunc(t, run, args)
		assertMainFixture(t, "numeric.golden", got)
	})
}

func Test_runErrPatern(t *testing.T) {
	t.Run("empty argument", func(t *testing.T) {
		got := run([]string{})
		if got != 1 {
			t.Errorf("mismatch got=%d, want=%d", got, 1)
		}
	})

	t.Run("specify ocsv file that do not exist", func(t *testing.T) {
		got := run([]string{"sqly", "not_exist.csv"})
		if got != 1 {
			t.Errorf("mismatch got=%d, want=%d", got, 1)
		}
	})

	t.Run("invalid flag is reported as a CLI error, not a shell initialization failure", func(t *testing.T) {
		stderr := getStderrForRunFunc(t, run, []string{"sqly", "--no-such-flag"})

		if bytes.Contains(stderr, []byte("failed to initialize sqly shell")) {
			t.Errorf("an invalid flag must not be labeled a shell init failure: %s", stderr)
		}
		if !bytes.Contains(stderr, []byte("unknown flag")) {
			t.Errorf("stderr should explain the invalid flag: %s", stderr)
		}
	})
}

func Test_startupErrorMessage(t *testing.T) {
	t.Parallel()

	t.Run("a CLI argument error is printed verbatim, without the shell-start prefix", func(t *testing.T) {
		t.Parallel()
		_, argErr := config.NewArg([]string{"sqly", "--no-such-flag"})
		if argErr == nil {
			t.Fatal("expected NewArg to reject --no-such-flag")
		}

		got := startupErrorMessage(argErr)
		if got != argErr.Error() {
			t.Errorf("CLI error should be printed verbatim: got=%q, want=%q", got, argErr.Error())
		}
	})

	t.Run("a genuine shell-start failure keeps the shell initialization prefix", func(t *testing.T) {
		t.Parallel()
		got := startupErrorMessage(errors.New("history db open failed"))
		want := "failed to initialize sqly shell: history db open failed"
		if got != want {
			t.Errorf("mismatch got=%q, want=%q", got, want)
		}
	})
}

func BenchmarkImport100000Records(b *testing.B) {
	b.ResetTimer()

	for range b.N {
		run([]string{
			"sqly",
			"--sql",
			"SELECT * FROM `customers100000` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000",
			"testdata/benchmark/customers100000.csv",
		})
	}
}

func getStdoutForRunFunc(t *testing.T, f func([]string) int, list []string) []byte {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	var buffer bytes.Buffer
	config.Stdout = &buffer

	f(list)
	return buffer.Bytes()
}

func getStderrForRunFunc(t *testing.T, f func([]string) int, list []string) []byte {
	t.Helper()
	backupColorStderr := config.Stderr
	defer func() {
		config.Stderr = backupColorStderr
	}()

	var buffer bytes.Buffer
	config.Stderr = &buffer

	f(list)
	return buffer.Bytes()
}

func assertMainFixture(t *testing.T, name string, got []byte) {
	t.Helper()
	testutil.AssertFileEquals(t, filepath.Join("testdata", "golden", name), got)
}
