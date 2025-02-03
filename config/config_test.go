// Package config manage sqly configuration
package config

import (
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/nao1215/gorky/file"
)

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
		if !file.IsDir(want) {
			t.Errorf("failed to create config directory at %s", want)
		}
	})
}
