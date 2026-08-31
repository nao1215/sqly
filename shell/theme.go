package shell

import (
	"sort"
	"strings"

	"github.com/nao1215/prompt"
)

// syntaxTheme is the colors one theme draws SQL in, plus the prompt's own
// scheme so a light theme is light all the way through rather than colored
// tokens on a dark prompt.
//
// A role left at the zero color is drawn in the prompt's input color, which is
// what an identifier gets: naming everything would make the line a rainbow and
// tell the reader less than naming the few things that carry meaning.
type syntaxTheme struct {
	// name is what .theme takes and what the settings file records.
	name string
	// prompt is the scheme the prompt draws itself with: the prefix, the input,
	// and the completion menu.
	prompt *prompt.ColorScheme
	// keyword is a SQL keyword, quoted is a name in quotes, and the rest are
	// what they say.
	keyword prompt.Color
	str     prompt.Color
	comment prompt.Color
	number  prompt.Color
	quoted  prompt.Color
	// command is a helper command's name, the ".mode" of a ".mode csv".
	command prompt.Color
}

// noHighlightTheme is the way out for a terminal or a reader that wants none.
// It names no colors, so nothing is highlighted and the prompt draws the input
// the way it did before any of this.
const noHighlightTheme = "none"

// defaultTheme is what a session with no saved choice starts in. It is night-owl
// because that is the scheme the prompt has always drawn sqly with.
const defaultTheme = "night-owl"

// syntaxThemes are the themes .theme can select, by name.
//
// The names follow the palettes they come from, which is what someone who has
// chosen one in another program will look for. sqluv, sqly's sibling, offers
// the same choice over its own interface.
func syntaxThemes() map[string]syntaxTheme {
	return map[string]syntaxTheme{
		noHighlightTheme: {
			name:   noHighlightTheme,
			prompt: prompt.ThemeNightOwl,
		},
		defaultTheme: {
			name:    defaultTheme,
			prompt:  prompt.ThemeNightOwl,
			keyword: prompt.Color{R: 0xc7, G: 0x92, B: 0xea, Bold: true},
			str:     prompt.Color{R: 0xec, G: 0xc4, B: 0x8d},
			comment: prompt.Color{R: 0x63, G: 0x77, B: 0x7d},
			number:  prompt.Color{R: 0xf7, G: 0x8c, B: 0x6c},
			quoted:  prompt.Color{R: 0x7f, G: 0xdb, B: 0xca},
			command: prompt.Color{R: 0x82, G: 0xaa, B: 0xff, Bold: true},
		},
		"dracula": {
			name:    "dracula",
			prompt:  prompt.ThemeDracula,
			keyword: prompt.Color{R: 0xff, G: 0x79, B: 0xc6, Bold: true},
			str:     prompt.Color{R: 0xf1, G: 0xfa, B: 0x8c},
			comment: prompt.Color{R: 0x62, G: 0x72, B: 0xa4},
			number:  prompt.Color{R: 0xbd, G: 0x93, B: 0xf9},
			quoted:  prompt.Color{R: 0x8b, G: 0xe9, B: 0xfd},
			command: prompt.Color{R: 0x50, G: 0xfa, B: 0x7b, Bold: true},
		},
		"monokai": {
			name:    "monokai",
			prompt:  prompt.ThemeMonokai,
			keyword: prompt.Color{R: 0xf9, G: 0x26, B: 0x72, Bold: true},
			str:     prompt.Color{R: 0xe6, G: 0xdb, B: 0x74},
			comment: prompt.Color{R: 0x75, G: 0x71, B: 0x5e},
			number:  prompt.Color{R: 0xae, G: 0x81, B: 0xff},
			quoted:  prompt.Color{R: 0x66, G: 0xd9, B: 0xef},
			command: prompt.Color{R: 0xa6, G: 0xe2, B: 0x2e, Bold: true},
		},
		"nord": {
			name:    "nord",
			prompt:  prompt.ThemeDark,
			keyword: prompt.Color{R: 0x81, G: 0xa1, B: 0xc1, Bold: true},
			str:     prompt.Color{R: 0xa3, G: 0xbe, B: 0x8c},
			comment: prompt.Color{R: 0x61, G: 0x6e, B: 0x88},
			number:  prompt.Color{R: 0xb4, G: 0x8e, B: 0xad},
			quoted:  prompt.Color{R: 0x8f, G: 0xbc, B: 0xbb},
			command: prompt.Color{R: 0x88, G: 0xc0, B: 0xd0, Bold: true},
		},
		"solarized": {
			name:    "solarized",
			prompt:  prompt.ThemeSolarizedDark,
			keyword: prompt.Color{R: 0x85, G: 0x99, B: 0x00, Bold: true},
			str:     prompt.Color{R: 0x2a, G: 0xa1, B: 0x98},
			comment: prompt.Color{R: 0x58, G: 0x6e, B: 0x75},
			number:  prompt.Color{R: 0xd3, G: 0x36, B: 0x82},
			quoted:  prompt.Color{R: 0x26, G: 0x8b, B: 0xd2},
			command: prompt.Color{R: 0xb5, G: 0x89, B: 0x00, Bold: true},
		},
		"gruvbox": {
			name:    "gruvbox",
			prompt:  prompt.ThemeDark,
			keyword: prompt.Color{R: 0xfb, G: 0x49, B: 0x34, Bold: true},
			str:     prompt.Color{R: 0xb8, G: 0xbb, B: 0x26},
			comment: prompt.Color{R: 0x92, G: 0x83, B: 0x74},
			number:  prompt.Color{R: 0xd3, G: 0x86, B: 0x9b},
			quoted:  prompt.Color{R: 0x83, G: 0xa5, B: 0x98},
			command: prompt.Color{R: 0xfa, G: 0xbd, B: 0x2f, Bold: true},
		},
		"tokyo-night": {
			name:    "tokyo-night",
			prompt:  prompt.ThemeNightOwl,
			keyword: prompt.Color{R: 0xbb, G: 0x9a, B: 0xf7, Bold: true},
			str:     prompt.Color{R: 0x9e, G: 0xce, B: 0x6a},
			comment: prompt.Color{R: 0x56, G: 0x5f, B: 0x89},
			number:  prompt.Color{R: 0xff, G: 0x9e, B: 0x64},
			quoted:  prompt.Color{R: 0x7d, G: 0xcf, B: 0xff},
			command: prompt.Color{R: 0x7a, G: 0xa2, B: 0xf7, Bold: true},
		},
		"catppuccin": {
			name:    "catppuccin",
			prompt:  prompt.ThemeDark,
			keyword: prompt.Color{R: 0xcb, G: 0xa6, B: 0xf7, Bold: true},
			str:     prompt.Color{R: 0xa6, G: 0xe3, B: 0xa1},
			comment: prompt.Color{R: 0x6c, G: 0x70, B: 0x86},
			number:  prompt.Color{R: 0xfa, G: 0xb3, B: 0x87},
			quoted:  prompt.Color{R: 0x89, G: 0xdc, B: 0xeb},
			command: prompt.Color{R: 0x89, G: 0xb4, B: 0xfa, Bold: true},
		},
		"vscode": {
			name:    "vscode",
			prompt:  prompt.ThemeVSCode,
			keyword: prompt.Color{R: 0x56, G: 0x9c, B: 0xd6, Bold: true},
			str:     prompt.Color{R: 0xce, G: 0x91, B: 0x78},
			comment: prompt.Color{R: 0x6a, G: 0x99, B: 0x55},
			number:  prompt.Color{R: 0xb5, G: 0xce, B: 0xa8},
			quoted:  prompt.Color{R: 0x4e, G: 0xc9, B: 0xb0},
			command: prompt.Color{R: 0xdc, G: 0xdc, B: 0xaa, Bold: true},
		},
		"github-light": {
			name:    "github-light",
			prompt:  prompt.ThemeLight,
			keyword: prompt.Color{R: 0xcf, G: 0x22, B: 0x2e, Bold: true},
			str:     prompt.Color{R: 0x0a, G: 0x30, B: 0x69},
			comment: prompt.Color{R: 0x6e, G: 0x77, B: 0x81},
			number:  prompt.Color{R: 0x05, G: 0x50, B: 0xae},
			quoted:  prompt.Color{R: 0x95, G: 0x38, B: 0x00},
			command: prompt.Color{R: 0x82, G: 0x50, B: 0xdf, Bold: true},
		},
		"accessible": {
			name:    "accessible",
			prompt:  prompt.ThemeAccessible,
			keyword: prompt.Color{R: 0xff, G: 0xff, B: 0x00, Bold: true},
			str:     prompt.Color{R: 0x00, G: 0xff, B: 0xff},
			comment: prompt.Color{R: 0xc0, G: 0xc0, B: 0xc0},
			number:  prompt.Color{R: 0xff, G: 0xff, B: 0xff, Bold: true},
			quoted:  prompt.Color{R: 0x00, G: 0xff, B: 0x00},
			command: prompt.Color{R: 0xff, G: 0xff, B: 0xff, Bold: true},
		},
	}
}

// lookupTheme returns the theme by name, or reports that there is none.
// The match is case-insensitive because a theme name is typed, not code.
func lookupTheme(name string) (syntaxTheme, bool) {
	themes := syntaxThemes()
	theme, ok := themes[strings.ToLower(strings.TrimSpace(name))]
	return theme, ok
}

// themeNames lists the themes in name order, which is how .theme offers them
// and how completion does.
func themeNames() []string {
	themes := syntaxThemes()
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// highlights reports whether this theme colors anything. Only "none" does not.
func (t syntaxTheme) highlights() bool {
	return t.name != noHighlightTheme
}
