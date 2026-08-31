package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// settingsFileName is what the settings file is called. It sits beside the
// history file, which is the other thing a session leaves behind, so a run told
// where to keep its history keeps its settings there too.
const settingsFileName = "settings.json"

// defaultFilePerm keeps the settings file readable by its owner alone, like the
// history beside it: neither holds a secret, but both are a record of what
// someone was doing.
const defaultFilePerm = 0o600

// Settings are the choices a session makes that outlive it.
//
// Only what a user would be annoyed to set twice belongs here. The output mode
// and the dialect are deliberately absent: those are answers to "what am I
// doing right now", and a shell that opened in last week's mode would surprise
// more than it saved.
type Settings struct {
	// Theme names the colors the interactive shell draws SQL in. An empty one
	// means the built-in default, so a file written by a later version that
	// names a theme this one does not have falls back rather than fails.
	Theme string `json:"theme"`
}

// settingsFilePath returns where the settings file is kept, resolving the
// default when SQLY_SETTINGS_PATH named none.
//
// The default is beside the history, not in the config directory regardless: a
// run told where to keep its history has said where its per-session state
// lives, and creating the config directory to write a theme into would ignore
// that. It matters most to a test, which isolates itself by routing history to
// a temp directory and would otherwise read and write the developer's real
// settings.
func (c *Config) settingsFilePath() string {
	if c.SettingsPath != "" {
		return c.SettingsPath
	}
	if c.HistoryPath != "" {
		return filepath.Join(filepath.Dir(c.HistoryPath), settingsFileName)
	}
	return filepath.Join(c.Dir(), settingsFileName)
}

// LoadSettings reads what an earlier session saved.
//
// A missing file is not a failure: it is what a first run looks like, and the
// defaults are the answer. A file that cannot be read or parsed is reported,
// because that is a real fault the caller may want to mention -- but the
// defaults come back with it, so a corrupt file costs a warning rather than
// the shell.
func (c *Config) LoadSettings() (Settings, error) {
	data, err := os.ReadFile(c.settingsFilePath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("failed to read %s: %w", c.settingsFilePath(), err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("failed to read %s: %w", c.settingsFilePath(), err)
	}
	return settings, nil
}

// SaveSettings writes settings for the next session to read. The config
// directory is created if it is not there, which is what a first run needs.
func (c *Config) SaveSettings(settings Settings) error {
	path := c.settingsFilePath()
	if err := os.MkdirAll(filepath.Dir(path), defaultDirPerm); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}

	// Indented, because this is a file someone may open and edit by hand.
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode settings: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), defaultFilePerm); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
