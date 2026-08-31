// Package config manage sqly configuration
package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/caarlos0/env/v11"
)

const (
	// Windows represents the Windows operating system identifier used for OS-specific logic.
	Windows = "windows"
)

const (
	// defaultDirPerm is the default permission for directory creation.
	defaultDirPerm = 0o750
)

// Config is the configuration for sqly.
type Config struct {
	// HistoryPath is where the shell's command history is kept. It is a text
	// file, one entry per line, so SQLY_HISTORY_PATH is what names it; the
	// SQLY_HISTORY_DB_PATH that preceded it named a SQLite database.
	HistoryPath string `env:"SQLY_HISTORY_PATH"`
	// SettingsPath is where the session settings that outlive a session are
	// kept, named by SQLY_SETTINGS_PATH the way SQLY_HISTORY_PATH names the
	// history. Empty means the default beside it; settingsFilePath resolves
	// that, so nothing has to check.
	SettingsPath string `env:"SQLY_SETTINGS_PATH"`
}

// NewConfig return *Config.
func NewConfig() (*Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	// Only create the default config directory when history falls back to the
	// default location. When SQLY_HISTORY_PATH is set the caller routed history
	// elsewhere, so creating the XDG directory would be a useless side effect.
	if cfg.HistoryPath == "" {
		if err := cfg.CreateDir(); err != nil {
			return nil, err
		}
		cfg.HistoryPath = filepath.Join(cfg.Dir(), "history")
	}
	return &cfg, nil
}

// Dir return configuration directory path.
func (c *Config) Dir() string {
	return filepath.Join(xdg.ConfigHome, "sqly")
}

// CreateDir make configuration directory.
func (c *Config) CreateDir() error {
	return os.MkdirAll(c.Dir(), defaultDirPerm)
}
