// Package config manage sqly configuration
package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/caarlos0/env/v6"
)

const (
	// Windows represents the Windows operating system identifier used for OS-specific logic.
	Windows = "windows"
)

// Config is sqly configuration.
type Config struct {
	HistoryDBPath string `env:"SQLY_HISTORY_DB_PATH"`
}

// NewConfig return *Config.
func NewConfig() (*Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.CreateDir(); err != nil {
		return nil, err
	}

	if cfg.HistoryDBPath == "" {
		cfg.HistoryDBPath = filepath.Join(cfg.Dir(), "history.db")
	}
	return &cfg, nil
}

// Dir return configuration directory path.
func (c *Config) Dir() string {
	return filepath.Join(xdg.ConfigHome, "sqly")
}

// CreateDir make configuration directory.
func (c *Config) CreateDir() error {
	return os.MkdirAll(c.Dir(), 0750)
}
