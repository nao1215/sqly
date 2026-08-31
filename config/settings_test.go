package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
)

// newSettingsConfig builds a config whose settings file is in a temp directory,
// so a test never reads or writes the developer's real one.
func newSettingsConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{SettingsPath: filepath.Join(t.TempDir(), "settings.json")}
}

// TestSettingsRoundTrip covers what the feature is for: what one session saved
// is what the next one reads.
func TestSettingsRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := newSettingsConfig(t)
	want := config.Settings{Theme: "dracula"}
	if err := cfg.SaveSettings(want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	got, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got != want {
		t.Errorf("settings read back = %+v, want %+v", got, want)
	}
}

// TestLoadSettingsWithoutAFile covers a first run. There is nothing to read and
// nothing wrong with that, so the defaults come back without an error.
func TestLoadSettingsWithoutAFile(t *testing.T) {
	t.Parallel()

	got, err := newSettingsConfig(t).LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings on a first run: %v", err)
	}
	if (got != config.Settings{}) {
		t.Errorf("settings = %+v, want the defaults", got)
	}
}

// TestLoadSettingsFromAnUnreadableFile covers a file that is there and wrong.
// The fault is reported, because it is one, and the defaults come back with it
// so a corrupt file costs a warning rather than the shell.
func TestLoadSettingsFromAnUnreadableFile(t *testing.T) {
	t.Parallel()

	cfg := newSettingsConfig(t)
	if err := os.WriteFile(cfg.SettingsPath, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("writing the broken file: %v", err)
	}

	got, err := cfg.LoadSettings()
	if err == nil {
		t.Fatal("LoadSettings accepted a file that is not JSON, want an error")
	}
	if !strings.Contains(err.Error(), cfg.SettingsPath) {
		t.Errorf("error = %v, want it to name the file that is wrong", err)
	}
	if (got != config.Settings{}) {
		t.Errorf("settings = %+v, want the defaults alongside the error", got)
	}
}

// TestSaveSettingsCreatesTheDirectory covers a first run again, from the other
// side: there is nowhere to write yet.
func TestSaveSettingsCreatesTheDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")
	cfg := &config.Config{SettingsPath: path}
	if err := cfg.SaveSettings(config.Settings{Theme: "nord"}); err != nil {
		t.Fatalf("SaveSettings into a directory that does not exist: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the settings file was not written: %v", err)
	}
}

// TestSettingsFileIsReadableByHand checks the shape of what is written. It is a
// file someone may open, so it is indented and ends in a newline.
func TestSettingsFileIsReadableByHand(t *testing.T) {
	t.Parallel()

	cfg := newSettingsConfig(t)
	if err := cfg.SaveSettings(config.Settings{Theme: "monokai"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	data, err := os.ReadFile(cfg.SettingsPath)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}

	want := "{\n  \"theme\": \"monokai\"\n}\n"
	if string(data) != want {
		t.Errorf("settings file = %q, want %q", data, want)
	}
}

// TestSaveSettingsOverwrites covers a second change in one session: the file
// holds the latest choice, not both.
func TestSaveSettingsOverwrites(t *testing.T) {
	t.Parallel()

	cfg := newSettingsConfig(t)
	for _, theme := range []string{"nord", "dracula", "none"} {
		if err := cfg.SaveSettings(config.Settings{Theme: theme}); err != nil {
			t.Fatalf("SaveSettings(%q): %v", theme, err)
		}
	}
	got, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.Theme != "none" {
		t.Errorf("theme = %q, want the last one saved", got.Theme)
	}
}

// TestSettingsDefaultToBesideTheHistory covers where the file goes when
// SQLY_SETTINGS_PATH named nowhere. A run told where to keep its history has
// said where its per-session state lives, and a test that routes history to a
// temp directory is isolated by that alone.
func TestSettingsDefaultToBesideTheHistory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{HistoryPath: filepath.Join(dir, "history")}
	if err := cfg.SaveSettings(config.Settings{Theme: "nord"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	want := filepath.Join(dir, "settings.json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the settings file is not beside the history at %s: %v", want, err)
	}

	got, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.Theme != "nord" {
		t.Errorf("theme read back = %q, want nord", got.Theme)
	}
}

// TestSettingsPathWinsOverTheHistory covers the two being named at once: the
// one that names the settings file is the one that decides.
func TestSettingsPathWinsOverTheHistory(t *testing.T) {
	t.Parallel()

	historyDir, settingsPath := t.TempDir(), filepath.Join(t.TempDir(), "chosen.json")
	cfg := &config.Config{
		HistoryPath:  filepath.Join(historyDir, "history"),
		SettingsPath: settingsPath,
	}
	if err := cfg.SaveSettings(config.Settings{Theme: "nord"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("the settings file was not written where SQLY_SETTINGS_PATH named: %v", err)
	}
	if _, err := os.Stat(filepath.Join(historyDir, "settings.json")); err == nil {
		t.Error("a settings file was also written beside the history, which nothing asked for")
	}
}
