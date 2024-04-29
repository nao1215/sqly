package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/gorky/golden"
	"github.com/nao1215/sqly/config"
)

func Test_main(t *testing.T) {
	t.Run("show version message", func(t *testing.T) {
		osExit = func(code int) {}
		os.Args = []string{"sqly", "-v"}
		defer func() {
			osExit = os.Exit
			os.Args = []string{}
		}()

		got := getStdout(t, main)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "version", got)
	})
}

func Test_run(t *testing.T) {
	t.Run("show version message", func(t *testing.T) {
		args := []string{"sqly", "--version"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "version", got)
	})

	t.Run("SELECT * FROM actor ORDER BY actor ASC LIMIT 5, print ascii table", func(t *testing.T) {
		args := []string{"sqly", "--sql", "SELECT * FROM actor ORDER BY actor ASC LIMIT 5", "testdata/actor.csv"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_asc_limit5_table", got)
	})

	t.Run("sqly --sql 'SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id' testdata/user.csv testdata/identifier.csv", func(t *testing.T) {
		args := []string{"sqly", "--csv", "--sql", "SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id", "testdata/user.csv", "testdata/identifier.csv"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_inner_join_csv", got)
	})

	t.Run("sqly --json --sql 'SELECT * FROM user' testdata/user.csv", func(t *testing.T) {
		args := []string{"sqly", "--json", "--sql", "SELECT * FROM user", "testdata/user.csv"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_json", got)
	})

	t.Run("sqly --tsv --sql 'SELECT * FROM user' testdata/user.csv", func(t *testing.T) {
		args := []string{"sqly", "--tsv", "--sql", "SELECT * FROM user", "testdata/user.csv"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_tsv", got)
	})

	t.Run("sqly --ltsv --sql 'SELECT * FROM user' testdata/user.csv", func(t *testing.T) {
		args := []string{"sqly", "--ltsv", "--sql", "SELECT * FROM user", "testdata/user.csv"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "select_ltsv", got)
	})

	t.Run("import excel, output csv", func(t *testing.T) {
		args := []string{"sqly", "--sql", "SELECT * FROM test_sheet", "-S", "test_sheet", "--csv", "testdata/sample.xlsx"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "excel_to_csv", got)
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
		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "numeric", got)
	})

	t.Run("Fix Issue 42: Panic when json field is null", func(t *testing.T) {
		args := []string{"sqly", "--sql", "select * from bug_issue42 limit 1", "--csv", "testdata/bug_issue42.json"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "fix_bug_issue42_csv", got)
	})

	t.Run("Fix Issue 43: Panic when importing json table with numeric field", func(t *testing.T) {
		args := []string{"sqly", "--sql", "select * from bug_issue43 limit 1", "--csv", "testdata/bug_issue43.json"}
		got := getStdoutForRunFunc(t, run, args)

		g := golden.New(t,
			golden.WithFixtureDir(filepath.Join("testdata", "golden")))
		g.Assert(t, "fix_bug_issue43_csv", got)
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
}

func getStdoutForRunFunc(t *testing.T, f func([]string) int, list []string) []byte {
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

	f(list)
	w.Close()

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
	w.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes()
}
