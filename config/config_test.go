// Package config manage sqly configuration
package config

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nao1215/gorky/file"
)

func TestConfig_CreateDir(t *testing.T) {
	t.Run("Create sqly config directory", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		c, err := NewConfig()
		if err != nil {
			t.Fatal(err)
		}

		if err := c.CreateDir(); err != nil {
			t.Fatal(err)
		}

		if runtime.GOOS != "windows" { //nolint
			want := filepath.Join(homeDir, ".config", "sqly")
			if !file.IsDir(want) {
				t.Errorf("failed to create config directory at %s", want)
			}
		}
	})
}
