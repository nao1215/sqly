package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nao1215/golden"
	"github.com/nao1215/sqly/config"
)

func Test_main(t *testing.T) {
	t.Run("show version message", func(t *testing.T) {
		osExit = func(code int) { return }
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

}

func getStdoutForRunFunc(t *testing.T, f func([]string) int, list []string) []byte {
	t.Helper()
	backupColorStdout := config.Stdout
	defer func() {
		config.Stdout = backupColorStdout
	}()

	r, w, _ := os.Pipe()
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

	r, w, _ := os.Pipe()
	config.Stdout = w

	f()
	w.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(r); err != nil {
		t.Fatalf("failed to read buffer: %v", err)
	}
	return buffer.Bytes()
}
