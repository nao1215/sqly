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
func highlightSQL(input string, theme syntaxTheme, d dialect.Dialect, names schemaNames) []prompt.StyleSpan {
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
	position := identifierPosition{}
	for tok := range sqltext.Regions(input, d) {
		color, ok := regionColor(tok, input, theme, names, &position, d)
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

// identifierPosition is what the words already read say about the next one: a
// name right after FROM, JOIN, INTO or UPDATE is a table however else it reads.
// It is a running state rather than a parse, because this walks a line that is
// still being typed.
type identifierPosition struct {
	wantTable bool
	// nameWantedTable is whether the name just read was in a table position.
	// A name before a "." does not spend that position -- "FROM main.actor"
	// reads from actor, not from main -- and the name after the dot is what it
	// belongs to. It cannot be decided when the first name is read, because
	// whether a dot follows is not known yet, so it is remembered instead.
	nameWantedTable bool
	// prevEnd is where the last region ended, so the punctuation before the
	// next one can be read.
	prevEnd int
}

// regionColor is what one region is drawn in, and whether it is drawn at all.
//
// A name the session does not have keeps the input color -- an alias, a
// function, a table not imported yet -- so a misspelled table is visible as the
// one word on the line with no color of its own. That is the point of coloring
// names at all: sqly knows which ones exist, which an editor highlighting the
// same text does not.
func regionColor(tok sqltext.Token, input string, theme syntaxTheme, names schemaNames, position *identifierPosition, d dialect.Dialect) (prompt.Color, bool) {
	// A "." joins two names into one, and the name that matters is the one
	// after it, so the table position carries across rather than being spent
	// on the schema qualifier.
	qualified := position.prevEnd > 0 && separatorBetween(input, position.prevEnd, tok.Start) == "."
	position.prevEnd = tok.End

	switch tok.Kind {
	case sqltext.String:
		return theme.str, true
	case sqltext.Comment:
		return theme.comment, true
	case sqltext.QuotedIdentifier:
		// A name in quotes is a name: it is looked up like a bare one, with the
		// quotes taken off first. Unknown, it keeps the input color, the same
		// answer a bare name it does not recognize gets.
		return wordColor(sqltext.Unquote(tok.Text(input), d), theme, names, position, qualified)
	case sqltext.Semicolon:
		position.wantTable = false
		position.nameWantedTable = false
		return prompt.Color{}, false
	case sqltext.Word:
		return wordColor(tok.Text(input), theme, names, position, qualified)
	default:
		return prompt.Color{}, false
	}
}

// wordColor is what one word in code is drawn in. qualified says the word is
// the part of a dotted name after the dot, which is what keeps a table position
// alive across a schema qualifier.
func wordColor(text string, theme syntaxTheme, names schemaNames, position *identifierPosition, qualified bool) (prompt.Color, bool) {
	upper := strings.ToUpper(text)
	if highlightKeywords()[upper] {
		// A keyword decides what the next word is: after these, a name is the
		// table being read from or written to, whatever else it might name.
		position.wantTable = tableIntroducers[upper]
		position.nameWantedTable = false
		return theme.keyword, true
	}

	// A name after a dot inherits the position of the name before it, so
	// "FROM main.actor" still expects a table when it reaches actor.
	wantTable := position.wantTable
	if qualified {
		wantTable = position.nameWantedTable
	}
	position.wantTable = false
	position.nameWantedTable = wantTable

	switch {
	case startsWithDigit(text):
		// A word starting with a digit is a number: SQL has no identifier that
		// may, so nothing else can reach here.
		return theme.number, true
	case wantTable && names.hasTable(text):
		return theme.table, true
	case names.hasColumn(text):
		return theme.column, true
	case names.hasTable(text):
		// Not in a table position, but the session has a table by this name:
		// the qualifier of "users.id", or a name repeated in a WHERE.
		return theme.table, true
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

// schemaNames are the table and column names the session has, lower-cased,
// which is what tells a name sqly knows from one it does not.
//
// It is a value rather than a lookup so the highlighter stays a pure function
// of what it is given: the shell holds the cache, and this is a view of it.
type schemaNames struct {
	tables  map[string]bool
	columns map[string]bool
}

// hasTable reports whether the session has a table by this name. SQL
// identifiers are case-insensitive, so the comparison is too.
func (n schemaNames) hasTable(name string) bool {
	return n.tables[strings.ToLower(name)]
}

// hasColumn reports whether any table in the session has a column by this name.
// Which table is not asked: a column named in a statement that does not say
// which table it belongs to is still that column, and resolving it properly
// would mean parsing what is being typed rather than reading it.
func (n schemaNames) hasColumn(name string) bool {
	return n.columns[strings.ToLower(name)]
}
