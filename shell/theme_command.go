package shell

import (
	"context"
	"fmt"
	"strings"

	"github.com/nao1215/sqly/config"
)

// themeCommand shows or sets the colors the shell draws SQL in.
//
// The choice outlives the session: it is written to the settings file, so a
// shell opened tomorrow looks the way this one was left. That is the whole
// reason it is a command rather than a flag -- a color is chosen once and then
// wanted forever, which a flag would ask for on every run.
func (c CommandList) themeCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		printSessionSetting(settingTheme, s.themeName(), strings.Join(themeNames(), ", "))
		return nil
	}
	if len(argv) > 1 {
		return &invocationError{Err: fmt.Errorf(".theme accepts a single theme name, got %d arguments", len(argv))}
	}

	theme, ok := lookupTheme(argv[0])
	if !ok {
		return &invocationError{Err: fmt.Errorf("unknown theme %q (available: %s)", argv[0], strings.Join(themeNames(), ", "))}
	}
	s.setTheme(theme)
	fmt.Fprintf(config.Stderr, "theme set to %s\n", theme.name)

	// Saving is best effort. A theme that cannot be remembered is still a theme
	// for this session, and losing the shell over a read-only config directory
	// would be a poor trade for a color.
	if err := s.saveTheme(theme.name); err != nil {
		fmt.Fprintf(config.Stderr, "warning: the theme applies to this session but could not be saved (%v)\n", err)
	}
	return nil
}

// themeName is the name of the theme in effect, for the message .theme prints
// when it is asked rather than told.
func (s *Shell) themeName() string {
	return s.theme.name
}

// setTheme applies a theme to this session: the colors SQL is drawn in, and the
// scheme the prompt draws itself with, so a light theme is light all the way
// through rather than colored tokens on a dark prompt.
func (s *Shell) setTheme(theme syntaxTheme) {
	s.theme = theme
	if s.promptSession != nil {
		s.promptSession.SetTheme(theme.prompt)
	}
}

// saveTheme records the choice for the next session.
func (s *Shell) saveTheme(name string) error {
	if s.config == nil {
		return nil // a partially built shell in a test has nowhere to save
	}
	settings, err := s.config.LoadSettings()
	if err != nil {
		// The file is unreadable, so the rest of it is not ours to preserve.
		settings = config.Settings{}
	}
	settings.Theme = name
	return s.config.SaveSettings(settings)
}

// themeFromSettings is the theme a session opens in: the one the last session
// saved, or the default when there is none or the name is not one this version
// has. A theme that has been renamed or removed costs the default rather than
// the shell.
func themeFromSettings(settings config.Settings) syntaxTheme {
	if theme, ok := lookupTheme(settings.Theme); ok {
		return theme
	}
	theme, _ := lookupTheme(defaultTheme)
	return theme
}
