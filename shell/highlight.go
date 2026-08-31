package shell

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/prompt"
	"github.com/nao1215/sqly/domain/sqltext"
)

// highlightKeywords is the vocabulary drawn as keywords, built once from the
// places that already hold it: the words the completer offers, the words the
// context analysis reads, and the statement vocabulary neither needs but a
// reader still expects to see colored.
//
// Deriving it rather than restating it is what keeps the three from drifting.
// A word none of them holds is drawn as an identifier, which is the safe way to
// be wrong: an unrecognized keyword looks like a name, where a name mistaken
// for a keyword would say the statement means something it does not.
var highlightKeywords = sync.OnceValue(func() map[string]bool {
	words := make(map[string]bool)
	add := func(phrase string) {
		for _, word := range strings.Fields(phrase) {
			words[strings.ToUpper(word)] = true
		}
	}

	for _, suggestion := range sqlKeywordSuggestions() {
		add(suggestion.Text) // "GROUP BY" and "INSERT INTO" are two words each
	}
	for word := range tableIntroducers {
		add(word)
	}
	for word := range columnIntroducers {
		add(word)
	}
	for word := range joinModifiers {
		add(word)
	}
	for word := range ddlKeywords {
		add(word)
	}
	for _, word := range statementVocabulary {
		add(word)
	}
	return words
})

// statementVocabulary is the rest of what a reader expects colored: the verbs
// and nouns of statements sqly runs but does not complete, because completion
// offers what someone is likely to be typing and a CREATE TRIGGER is not it.
var statementVocabulary = []string{
	"BEGIN", "COLLATE", "COLUMN", "COMMIT", "CONSTRAINT", "DEFAULT", "END",
	"ESCAPE", "EXCEPT", "EXISTS", "EXPLAIN", "FILTER", "FOREIGN", "IF", "INDEX",
	"INTERSECT", "INTO", "KEY", "OVER", "PARTITION", "PRAGMA", "PRIMARY",
	"RECURSIVE", "REFERENCES", "ROLLBACK", "TABLE", "THEN", "TRANSACTION",
	"TRIGGER", "UNION", "UNIQUE", "VACUUM", "VIEW", "WINDOW", "WITH",
}

// highlightSQL returns the runs of input to draw in the theme's colors, as rune
// offsets, which is what the prompt measures its spans in.
//
// The regions come from domain/sqltext, so what is a string literal, a quoted
// name, or a comment is decided by the dialect in effect rather than guessed at
// -- which is the difference between coloring "SELECT '(' -- not a comment'"
// correctly and coloring most of it as a comment.
//
// A theme that colors nothing returns no runs, and so does a line still being
// typed that holds nothing worth coloring. Both leave the prompt drawing the
// input the way it always did.
func highlightSQL(input string, theme syntaxTheme, d dialect.Dialect) []prompt.StyleSpan {
	if !theme.highlights() || input == "" {
		return nil
	}

	// A helper command is not SQL: its name is the thing worth marking, and the
	// rest is a path or a value that reads better plain.
	if span, ok := dotCommandSpan(input, theme); ok {
		return []prompt.StyleSpan{span}
	}

	var spans []prompt.StyleSpan
	offsets := &runeCursor{s: input}
	for tok := range sqltext.Regions(input, d) {
		color, ok := regionColor(tok, input, theme)
		if !ok {
			continue
		}
		spans = append(spans, prompt.StyleSpan{
			Start: offsets.at(tok.Start),
			End:   offsets.at(tok.End),
			Color: color,
		})
	}
	return spans
}

// regionColor is what one region is drawn in, and whether it is drawn at all.
// An identifier is not: leaving the names the user chose in the prompt's own
// color is what makes the colored words stand out.
func regionColor(tok sqltext.Token, input string, theme syntaxTheme) (prompt.Color, bool) {
	switch tok.Kind {
	case sqltext.String:
		return theme.str, true
	case sqltext.Comment:
		return theme.comment, true
	case sqltext.QuotedIdentifier:
		return theme.quoted, true
	case sqltext.Semicolon:
		return prompt.Color{}, false
	case sqltext.Word:
		text := tok.Text(input)
		switch {
		case highlightKeywords()[strings.ToUpper(text)]:
			return theme.keyword, true
		case startsWithDigit(text):
			// A word starting with a digit is a number: SQL has no identifier
			// that may, so nothing else can reach here.
			return theme.number, true
		default:
			return prompt.Color{}, false
		}
	default:
		return prompt.Color{}, false
	}
}

// dotCommandSpan marks the name of a helper command, and reports whether the
// input is one. Only the name is marked: what follows it is a path or a value,
// and coloring those would say they are something they are not.
func dotCommandSpan(input string, theme syntaxTheme) (prompt.StyleSpan, bool) {
	trimmed := strings.TrimLeft(input, " \t")
	if !looksLikeCommand(trimmed) {
		return prompt.StyleSpan{}, false
	}
	lead := len(input) - len(trimmed)
	name := trimmed
	if i := strings.IndexAny(trimmed, " \t"); i >= 0 {
		name = trimmed[:i]
	}
	offsets := &runeCursor{s: input}
	return prompt.StyleSpan{
		Start: offsets.at(lead),
		End:   offsets.at(lead + len(name)),
		Color: theme.command,
	}, true
}

// runeCursor converts byte offsets into s to the rune offsets the prompt
// measures spans in. sqltext reports byte offsets, which is what a caller
// slicing the string wants and not what a caller counting cells does.
//
// It is a cursor rather than a function because the offsets arrive in order:
// counting from the start of the string for each one walks the whole input per
// token, which is quadratic in a long statement -- and this runs on every
// keystroke. Walking forward from the last answer counts each byte once, which
// is worth about thirty times the wall clock over a two-hundred-column SELECT
// holding multi-byte names.
type runeCursor struct {
	s      string
	byteAt int
	runeAt int
}

// at returns the rune offset of byteOffset. An offset that goes backwards is
// answered from the start, so a caller that does not walk forward is slower
// rather than wrong.
func (c *runeCursor) at(byteOffset int) int {
	byteOffset = min(max(byteOffset, 0), len(c.s))
	if byteOffset < c.byteAt {
		c.byteAt, c.runeAt = 0, 0
	}
	c.runeAt += utf8.RuneCountInString(c.s[c.byteAt:byteOffset])
	c.byteAt = byteOffset
	return c.runeAt
}

// startsWithDigit reports whether s opens with an ASCII digit.
func startsWithDigit(s string) bool {
	return s != "" && s[0] >= '0' && s[0] <= '9'
}
