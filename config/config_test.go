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
