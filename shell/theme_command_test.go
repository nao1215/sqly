package shell

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
)

// newThemeShell builds a shell whose settings file is in a temp directory, so a
// test never reads or writes the developer's real one.
func newThemeShell(t *testing.T) *Shell {
	t.Helper()

	s := newBoundaryTestShell(t, Usecases{})
	s.config = &config.Config{SettingsPath: filepath.Join(t.TempDir(), "settings.json")}
	s.theme = themeFromSettings(config.Settings{})
	return s
}

// TestThemeCommandSetsAndSaves covers what the command is for: the session
// changes, and the next one opens the same way.
func TestThemeCommandSetsAndSaves(t *testing.T) {
	// Serial: replaces config.Stderr, which is a package-level writer.
	s := newThemeShell(t)

	backup := config.Stderr
	defer func() { config.Stderr = backup }()
	var stderr bytes.Buffer
	config.Stderr = &stderr

	if err := s.commands.themeCommand(context.Background(), s, []string{"dracula"}); err != nil {
		t.Fatalf(".theme dracula: %v", err)
	}
	if s.themeName() != "dracula" {
		t.Errorf("the session theme is %q, want dracula", s.themeName())
	}
	if !strings.Contains(stderr.String(), "theme set to dracula") {
		t.Errorf("stderr = %q, want it to say the theme changed", stderr.String())
	}

	saved, err := s.config.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if saved.Theme != "dracula" {
		t.Errorf("the saved theme is %q, want dracula", saved.Theme)
	}
}

// TestThemeCommandWithoutArgumentsReports covers asking rather than telling.
func TestThemeCommandWithoutArgumentsReports(t *testing.T) {
	// Serial: replaces config.Stderr.
	s := newThemeShell(t)

	backup := config.Stderr
	defer func() { config.Stderr = backup }()
	var stderr bytes.Buffer
	config.Stderr = &stderr

	if err := s.commands.themeCommand(context.Background(), s, nil); err != nil {
		t.Fatalf(".theme: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "current theme: "+defaultTheme) {
		t.Errorf("stderr = %q, want it to name the theme in effect", out)
	}
	for _, name := range []string{"dracula", noHighlightTheme} {
		if !strings.Contains(out, name) {
			t.Errorf("stderr = %q, want it to list %q among the available themes", out, name)
		}
	}
}

// TestThemeCommandRejectsWhatItCannotApply covers the two ways the command can
// be written wrong. Neither changes the session.
func TestThemeCommandRejectsWhatItCannotApply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{name: "a name no theme has", argv: []string{"no-such-theme"}, want: "unknown theme"},
		{name: "more than one name", argv: []string{"nord", "dracula"}, want: "single theme name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newThemeShell(t)
			before := s.themeName()

			err := s.commands.themeCommand(context.Background(), s, tt.argv)
			if err == nil {
				t.Fatalf(".theme %v succeeded, want a refusal", tt.argv)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
			if s.themeName() != before {
				t.Errorf("a refused .theme changed the session to %q", s.themeName())
			}
		})
	}
}

// TestThemeFromSettings covers what a session opens in, including the two ways
// a saved name can be unusable.
func TestThemeFromSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		saved string
		want  string
	}{
		{name: "a saved theme is what the session opens in", saved: "nord", want: "nord"},
		{name: "no saved theme means the default", saved: "", want: defaultTheme},
		{name: "a name this version does not have means the default", saved: "removed-theme", want: defaultTheme},
		{name: "the no-highlight choice is remembered like any other", saved: noHighlightTheme, want: noHighlightTheme},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := themeFromSettings(config.Settings{Theme: tt.saved}).name; got != tt.want {
				t.Errorf("themeFromSettings(%q) = %q, want %q", tt.saved, got, tt.want)
			}
		})
	}
}

// TestThemeSurvivesAnUnwritableSettingsFile covers a config directory that
// cannot be written. The theme still applies to this session, because losing
// the shell over a color would be a poor trade.
func TestThemeSurvivesAnUnwritableSettingsFile(t *testing.T) {
	// Serial: replaces config.Stderr.
	s := newThemeShell(t)
	// A path whose parent is a file, so creating the directory fails.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("preparing the unwritable path: %v", err)
	}
	s.config = &config.Config{SettingsPath: filepath.Join(blocker, "settings.json")}

	backup := config.Stderr
	defer func() { config.Stderr = backup }()
	var stderr bytes.Buffer
	config.Stderr = &stderr

	if err := s.commands.themeCommand(context.Background(), s, []string{"nord"}); err != nil {
		t.Fatalf(".theme nord returned %v, want the session to carry on", err)
	}
	if s.themeName() != "nord" {
		t.Errorf("the session theme is %q, want nord", s.themeName())
	}
	if !strings.Contains(stderr.String(), "could not be saved") {
		t.Errorf("stderr = %q, want a warning that the choice was not saved", stderr.String())
	}
}
