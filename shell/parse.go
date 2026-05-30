package shell

import (
	"errors"
	"strings"
)

// splitArgs splits a helper-command line into arguments while honoring single
// quotes, double quotes, and backslash-escaped whitespace. This lets helper
// commands accept file paths and --sheet values that contain spaces, e.g.
//
//	.import "my data.csv"
//	.import --sheet "Q1 Sales" report.xlsx
//	.import --sheet='Q1 Sales' report.xlsx
//	.import my\ data.csv
//
// Adjacent quoted and unquoted segments are concatenated into a single argument
// (so --sheet="Q1 Sales" yields one token --sheet=Q1 Sales).
//
// A bare backslash is kept literally (e.g. Windows paths like C:\data\x.csv) and
// only escapes the next character when it is whitespace, a quote, or a backslash.
func splitArgs(input string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		inWord  bool // true once a token has started, so "" yields an empty arg
	)

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			if inWord {
				args = append(args, current.String())
				current.Reset()
				inWord = false
			}
		case '\'':
			inWord = true
			for i++; ; i++ {
				if i >= len(runes) {
					return nil, errors.New("unterminated single quote in command")
				}
				if runes[i] == '\'' {
					break
				}
				current.WriteRune(runes[i])
			}
		case '"':
			inWord = true
			for i++; ; i++ {
				if i >= len(runes) {
					return nil, errors.New("unterminated double quote in command")
				}
				if runes[i] == '\\' && i+1 < len(runes) {
					if next := runes[i+1]; next == '"' || next == '\\' {
						current.WriteRune(next)
						i++
						continue
					}
				}
				if runes[i] == '"' {
					break
				}
				current.WriteRune(runes[i])
			}
		case '\\':
			inWord = true
			if i+1 < len(runes) {
				switch next := runes[i+1]; next {
				case ' ', '\t', '\\', '\'', '"':
					current.WriteRune(next)
					i++
				default:
					current.WriteRune('\\') // keep literal (e.g. Windows path separator)
				}
			} else {
				current.WriteRune('\\')
			}
		default:
			inWord = true
			current.WriteRune(c)
		}
	}
	if inWord {
		args = append(args, current.String())
	}
	return args, nil
}
