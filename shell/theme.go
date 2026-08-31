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
	// keyword is a SQL keyword and the rest are what they say.
	keyword prompt.Color
	str     prompt.Color
	comment prompt.Color
	number  prompt.Color
	// table and column are names the session actually has, which is what sqly
	// knows and an editor does not. A name it does not recognize -- an alias,
	// a function, a table not imported yet -- keeps the input color, so a
	// misspelling is visible as the one word on the line with no color.
	table  prompt.Color
	column prompt.Color
	// command is a helper command's name, the ".mode" of a ".mode csv".
	command prompt.Color
}

// The themes .theme takes, named once because each appears as a map key, as the
// theme's own name, and as the name of the scheme it draws the prompt with.
const (
	themeDracula     = "dracula"
	themeMonokai     = "monokai"
	themeNord        = "nord"
	themeSolarized   = "solarized"
	themeGruvbox     = "gruvbox"
	themeTokyoNight  = "tokyo-night"
	themeCatppuccin  = "catppuccin"
	themeVscode      = "vscode"
	themeGithubLight = "github-light"
	themeAccessible  = "accessible"
)

// noHighlightTheme is the way out for a terminal or a reader that wants none.
// It names no colors, so nothing is highlighted and the prompt draws the input
// the way it did before any of this.
const noHighlightTheme = "none"

// defaultTheme is what a session with no saved choice starts in. It is night-owl
// because that is the scheme the prompt has always drawn sqly with.
const defaultTheme = "night-owl"

// schemeNightOwl draws the prompt in night-owl's own colors.
var schemeNightOwl = &prompt.ColorScheme{
	Name:   defaultTheme,
	Prefix: prompt.Color{R: 0x82, G: 0xaa, B: 0xff, Bold: true},
	Input:  prompt.Color{R: 0xd6, G: 0xde, B: 0xeb},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x7f, G: 0xdb, B: 0xca},
		Description: prompt.Color{R: 0x63, G: 0x77, B: 0x77},
	},
	Selected: prompt.Color{R: 0xc7, G: 0x92, B: 0xea, Bold: true},
}

// schemeDracula draws the prompt in dracula's own colors.
var schemeDracula = &prompt.ColorScheme{
	Name:   themeDracula,
	Prefix: prompt.Color{R: 0xff, G: 0x79, B: 0xc6, Bold: true},
	Input:  prompt.Color{R: 0xf8, G: 0xf8, B: 0xf2},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x8b, G: 0xe9, B: 0xfd},
		Description: prompt.Color{R: 0x62, G: 0x72, B: 0xa4},
	},
	Selected: prompt.Color{R: 0x50, G: 0xfa, B: 0x7b, Bold: true},
}

// schemeMonokai draws the prompt in monokai's own colors.
var schemeMonokai = &prompt.ColorScheme{
	Name:   themeMonokai,
	Prefix: prompt.Color{R: 0xf9, G: 0x26, B: 0x72, Bold: true},
	Input:  prompt.Color{R: 0xf8, G: 0xf8, B: 0xf2},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x66, G: 0xd9, B: 0xef},
		Description: prompt.Color{R: 0x75, G: 0x71, B: 0x5e},
	},
	Selected: prompt.Color{R: 0xa6, G: 0xe2, B: 0x2e, Bold: true},
}

// schemeNord draws the prompt in nord's own colors.
var schemeNord = &prompt.ColorScheme{
	Name:   themeNord,
	Prefix: prompt.Color{R: 0x88, G: 0xc0, B: 0xd0, Bold: true},
	Input:  prompt.Color{R: 0xd8, G: 0xde, B: 0xe9},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x8f, G: 0xbc, B: 0xbb},
		Description: prompt.Color{R: 0x61, G: 0x6e, B: 0x88},
	},
	Selected: prompt.Color{R: 0xa3, G: 0xbe, B: 0x8c, Bold: true},
}

// schemeSolarized draws the prompt in solarized's own colors.
var schemeSolarized = &prompt.ColorScheme{
	Name:   themeSolarized,
	Prefix: prompt.Color{R: 0xb5, G: 0x89, B: 0x00, Bold: true},
	Input:  prompt.Color{R: 0x93, G: 0xa1, B: 0xa1},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x26, G: 0x8b, B: 0xd2},
		Description: prompt.Color{R: 0x58, G: 0x6e, B: 0x75},
	},
	Selected: prompt.Color{R: 0x85, G: 0x99, B: 0x00, Bold: true},
}

// schemeGruvbox draws the prompt in gruvbox's own colors.
var schemeGruvbox = &prompt.ColorScheme{
	Name:   themeGruvbox,
	Prefix: prompt.Color{R: 0xfa, G: 0xbd, B: 0x2f, Bold: true},
	Input:  prompt.Color{R: 0xeb, G: 0xdb, B: 0xb2},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x83, G: 0xa5, B: 0x98},
		Description: prompt.Color{R: 0x92, G: 0x83, B: 0x74},
	},
	Selected: prompt.Color{R: 0x8e, G: 0xc0, B: 0x7c, Bold: true},
}

// schemeTokyoNight draws the prompt in tokyo-night's own colors.
var schemeTokyoNight = &prompt.ColorScheme{
	Name:   themeTokyoNight,
	Prefix: prompt.Color{R: 0x7a, G: 0xa2, B: 0xf7, Bold: true},
	Input:  prompt.Color{R: 0xc0, G: 0xca, B: 0xf5},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x7d, G: 0xcf, B: 0xff},
		Description: prompt.Color{R: 0x56, G: 0x5f, B: 0x89},
	},
	Selected: prompt.Color{R: 0xbb, G: 0x9a, B: 0xf7, Bold: true},
}

// schemeCatppuccin draws the prompt in catppuccin's own colors.
var schemeCatppuccin = &prompt.ColorScheme{
	Name:   themeCatppuccin,
	Prefix: prompt.Color{R: 0x89, G: 0xb4, B: 0xfa, Bold: true},
	Input:  prompt.Color{R: 0xcd, G: 0xd6, B: 0xf4},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x89, G: 0xdc, B: 0xeb},
		Description: prompt.Color{R: 0x6c, G: 0x70, B: 0x86},
	},
	Selected: prompt.Color{R: 0xcb, G: 0xa6, B: 0xf7, Bold: true},
}

// schemeVscode draws the prompt in vscode's own colors.
var schemeVscode = &prompt.ColorScheme{
	Name:   themeVscode,
	Prefix: prompt.Color{R: 0x56, G: 0x9c, B: 0xd6, Bold: true},
	Input:  prompt.Color{R: 0xd4, G: 0xd4, B: 0xd4},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x4e, G: 0xc9, B: 0xb0},
		Description: prompt.Color{R: 0x6a, G: 0x99, B: 0x55},
	},
	Selected: prompt.Color{R: 0xdc, G: 0xdc, B: 0xaa, Bold: true},
}

// schemeGithubLight draws the prompt in github-light's own colors.
var schemeGithubLight = &prompt.ColorScheme{
	Name:   themeGithubLight,
	Prefix: prompt.Color{R: 0x09, G: 0x69, B: 0xda, Bold: true},
	Input:  prompt.Color{R: 0x24, G: 0x29, B: 0x2f},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x95, G: 0x38, B: 0x00},
		Description: prompt.Color{R: 0x6e, G: 0x77, B: 0x81},
	},
	Selected: prompt.Color{R: 0x82, G: 0x50, B: 0xdf, Bold: true},
}

// schemeAccessible draws the prompt in accessible's own colors.
var schemeAccessible = &prompt.ColorScheme{
	Name:   themeAccessible,
	Prefix: prompt.Color{R: 0x00, G: 0xff, B: 0xff, Bold: true},
	Input:  prompt.Color{R: 0xff, G: 0xff, B: 0xff},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x00, G: 0xff, B: 0x00},
		Description: prompt.Color{R: 0xc0, G: 0xc0, B: 0xc0},
	},
	Selected: prompt.Color{R: 0xff, G: 0x00, B: 0xff, Bold: true},
}

// schemeNone draws the prompt in none's own colors.
var schemeNone = &prompt.ColorScheme{
	Name:   noHighlightTheme,
	Prefix: prompt.Color{R: 0x82, G: 0xaa, B: 0xff, Bold: true},
	Input:  prompt.Color{R: 0xd6, G: 0xde, B: 0xeb},
	Suggestion: prompt.SuggestionColors{
		Text:        prompt.Color{R: 0x7f, G: 0xdb, B: 0xca},
		Description: prompt.Color{R: 0x63, G: 0x77, B: 0x77},
	},
	Selected: prompt.Color{R: 0xc7, G: 0x92, B: 0xea, Bold: true},
}

// syntaxThemes are the themes .theme can select, by name.
//
// The names follow the palettes they come from, which is what someone who has
// chosen one in another program will look for. sqluv, sqly's sibling, offers
// the same choice over its own interface.
func syntaxThemes() map[string]syntaxTheme {
	return map[string]syntaxTheme{
		noHighlightTheme: {
			name:   noHighlightTheme,
			prompt: schemeNone,
		},
		defaultTheme: {
			name:    defaultTheme,
			prompt:  schemeNightOwl,
			keyword: prompt.Color{R: 0xc7, G: 0x92, B: 0xea, Bold: true},
			str:     prompt.Color{R: 0xec, G: 0xc4, B: 0x8d},
			comment: prompt.Color{R: 0x63, G: 0x77, B: 0x7d},
			number:  prompt.Color{R: 0xf7, G: 0x8c, B: 0x6c},
			table:   prompt.Color{R: 0x7f, G: 0xdb, B: 0xca},
			column:  prompt.Color{R: 0xad, G: 0xdb, B: 0x67},
			command: prompt.Color{R: 0x82, G: 0xaa, B: 0xff, Bold: true},
		},
		themeDracula: {
			name:    themeDracula,
			prompt:  schemeDracula,
			keyword: prompt.Color{R: 0xff, G: 0x79, B: 0xc6, Bold: true},
			str:     prompt.Color{R: 0xf1, G: 0xfa, B: 0x8c},
			comment: prompt.Color{R: 0x62, G: 0x72, B: 0xa4},
			number:  prompt.Color{R: 0xbd, G: 0x93, B: 0xf9},
			table:   prompt.Color{R: 0x8b, G: 0xe9, B: 0xfd},
			column:  prompt.Color{R: 0x50, G: 0xfa, B: 0x7b},
			command: prompt.Color{R: 0x50, G: 0xfa, B: 0x7b, Bold: true},
		},
		themeMonokai: {
			name:    themeMonokai,
			prompt:  schemeMonokai,
			keyword: prompt.Color{R: 0xf9, G: 0x26, B: 0x72, Bold: true},
			str:     prompt.Color{R: 0xe6, G: 0xdb, B: 0x74},
			comment: prompt.Color{R: 0x75, G: 0x71, B: 0x5e},
			number:  prompt.Color{R: 0xae, G: 0x81, B: 0xff},
			table:   prompt.Color{R: 0x66, G: 0xd9, B: 0xef},
			column:  prompt.Color{R: 0xa6, G: 0xe2, B: 0x2e},
			command: prompt.Color{R: 0xa6, G: 0xe2, B: 0x2e, Bold: true},
		},
		themeNord: {
			name:    themeNord,
			prompt:  schemeNord,
			keyword: prompt.Color{R: 0x81, G: 0xa1, B: 0xc1, Bold: true},
			str:     prompt.Color{R: 0xa3, G: 0xbe, B: 0x8c},
			comment: prompt.Color{R: 0x61, G: 0x6e, B: 0x88},
			number:  prompt.Color{R: 0xb4, G: 0x8e, B: 0xad},
			table:   prompt.Color{R: 0x8f, G: 0xbc, B: 0xbb},
			column:  prompt.Color{R: 0xa3, G: 0xbe, B: 0x8c},
			command: prompt.Color{R: 0x88, G: 0xc0, B: 0xd0, Bold: true},
		},
		themeSolarized: {
			name:    themeSolarized,
			prompt:  schemeSolarized,
			keyword: prompt.Color{R: 0x85, G: 0x99, B: 0x00, Bold: true},
			str:     prompt.Color{R: 0x2a, G: 0xa1, B: 0x98},
			comment: prompt.Color{R: 0x58, G: 0x6e, B: 0x75},
			number:  prompt.Color{R: 0xd3, G: 0x36, B: 0x82},
			table:   prompt.Color{R: 0x26, G: 0x8b, B: 0xd2},
			column:  prompt.Color{R: 0x2a, G: 0xa1, B: 0x98},
			command: prompt.Color{R: 0xb5, G: 0x89, B: 0x00, Bold: true},
		},
		themeGruvbox: {
			name:    themeGruvbox,
			prompt:  schemeGruvbox,
			keyword: prompt.Color{R: 0xfb, G: 0x49, B: 0x34, Bold: true},
			str:     prompt.Color{R: 0xb8, G: 0xbb, B: 0x26},
			comment: prompt.Color{R: 0x92, G: 0x83, B: 0x74},
			number:  prompt.Color{R: 0xd3, G: 0x86, B: 0x9b},
			table:   prompt.Color{R: 0x83, G: 0xa5, B: 0x98},
			column:  prompt.Color{R: 0x8e, G: 0xc0, B: 0x7c},
			command: prompt.Color{R: 0xfa, G: 0xbd, B: 0x2f, Bold: true},
		},
		themeTokyoNight: {
			name:    themeTokyoNight,
			prompt:  schemeTokyoNight,
			keyword: prompt.Color{R: 0xbb, G: 0x9a, B: 0xf7, Bold: true},
			str:     prompt.Color{R: 0x9e, G: 0xce, B: 0x6a},
			comment: prompt.Color{R: 0x56, G: 0x5f, B: 0x89},
			number:  prompt.Color{R: 0xff, G: 0x9e, B: 0x64},
			table:   prompt.Color{R: 0x7d, G: 0xcf, B: 0xff},
			column:  prompt.Color{R: 0x73, G: 0xda, B: 0xca},
			command: prompt.Color{R: 0x7a, G: 0xa2, B: 0xf7, Bold: true},
		},
		themeCatppuccin: {
			name:    themeCatppuccin,
			prompt:  schemeCatppuccin,
			keyword: prompt.Color{R: 0xcb, G: 0xa6, B: 0xf7, Bold: true},
			str:     prompt.Color{R: 0xa6, G: 0xe3, B: 0xa1},
			comment: prompt.Color{R: 0x6c, G: 0x70, B: 0x86},
			number:  prompt.Color{R: 0xfa, G: 0xb3, B: 0x87},
			table:   prompt.Color{R: 0x89, G: 0xdc, B: 0xeb},
			column:  prompt.Color{R: 0x94, G: 0xe2, B: 0xd5},
			command: prompt.Color{R: 0x89, G: 0xb4, B: 0xfa, Bold: true},
		},
		themeVscode: {
			name:    themeVscode,
			prompt:  schemeVscode,
			keyword: prompt.Color{R: 0x56, G: 0x9c, B: 0xd6, Bold: true},
			str:     prompt.Color{R: 0xce, G: 0x91, B: 0x78},
			comment: prompt.Color{R: 0x6a, G: 0x99, B: 0x55},
			number:  prompt.Color{R: 0xb5, G: 0xce, B: 0xa8},
			table:   prompt.Color{R: 0x4e, G: 0xc9, B: 0xb0},
			column:  prompt.Color{R: 0x9c, G: 0xdc, B: 0xfe},
			command: prompt.Color{R: 0xdc, G: 0xdc, B: 0xaa, Bold: true},
		},
		themeGithubLight: {
			name:    themeGithubLight,
			prompt:  schemeGithubLight,
			keyword: prompt.Color{R: 0xcf, G: 0x22, B: 0x2e, Bold: true},
			str:     prompt.Color{R: 0x0a, G: 0x30, B: 0x69},
			comment: prompt.Color{R: 0x6e, G: 0x77, B: 0x81},
			number:  prompt.Color{R: 0x05, G: 0x50, B: 0xae},
			table:   prompt.Color{R: 0x95, G: 0x38, B: 0x00},
			column:  prompt.Color{R: 0x11, G: 0x63, B: 0x29},
			command: prompt.Color{R: 0x82, G: 0x50, B: 0xdf, Bold: true},
		},
		themeAccessible: {
			name:    themeAccessible,
			prompt:  schemeAccessible,
			keyword: prompt.Color{R: 0xff, G: 0xff, B: 0x00, Bold: true},
			str:     prompt.Color{R: 0x00, G: 0xff, B: 0xff},
			comment: prompt.Color{R: 0xc0, G: 0xc0, B: 0xc0},
			number:  prompt.Color{R: 0xff, G: 0xff, B: 0xff, Bold: true},
			table:   prompt.Color{R: 0x00, G: 0xff, B: 0x00},
			column:  prompt.Color{R: 0x00, G: 0xff, B: 0xff},
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
