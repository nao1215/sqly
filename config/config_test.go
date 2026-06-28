// Package config manage sqly configuration
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

func TestMain(m *testing.M) {
	InitSQLite3()
	os.Exit(m.Run())
}

func TestConfigCreateDir(t *testing.T) {
	t.Run("Create sqly config directory", func(t *testing.T) {
		homeDir := t.TempDir()
		orgConfigHome := xdg.ConfigHome
		xdg.ConfigHome = homeDir
		t.Cleanup(func() {
			xdg.ConfigHome = orgConfigHome
		})

		c, err := NewConfig()
		if err != nil {
			t.Fatal(err)
		}

		if err := c.CreateDir(); err != nil {
			t.Fatal(err)
		}

		want := filepath.Join(homeDir, "sqly")

		if !isDir(t, want) {
			t.Errorf("failed to create config directory at %s", want)
		}
	})
}

func isDir(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func TestInitSQLite3Idempotent(t *testing.T) {
	// Registering the sqlite3 driver more than once must not panic with
	// "sql: Register called twice". The driver registry is process-global, so
	// this cannot run in parallel with code that registers drivers.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("InitSQLite3 panicked when called repeatedly: %v", r)
		}
	}()
	InitSQLite3()
	InitSQLite3()
}

func TestIsInputFromTTY(t *testing.T) {
	// Under `go test` stdin is not a terminal, so this must report false and,
	// most importantly, never panic. This guards the non-TTY batch-mode switch.
	if IsInputFromTTY() {
		t.Skip("stdin is a terminal in this environment; skipping non-TTY assertion")
	}
}
