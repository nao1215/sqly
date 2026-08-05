// Package sqltext reads SQL as text: what is code, and what is a string
// literal, a quoted identifier, or a comment.
//
// Every question sqly asks about a statement before running it needs that
// distinction. Does this script end mid-comment, so the next line is still
// inside it? Where does this statement end — is that ";" a terminator or a
// character in a value? Does this INSERT have a RETURNING clause, or a column
// named `returning_at`? Which verb do these CTEs feed? Each is answered by
// walking the text and knowing which regions are code.
//
// The walk lived five times, in two packages, in slightly different spellings.
// Nothing made them agree: a fix to the way one of them handled a bracket-quoted
// identifier or a doubled backtick reached that copy alone, and the other four
// went on being subtly wrong in whichever direction they had always been wrong.
// This package is that walk, once, so the answers cannot diverge.
//
// It is a dependency-free leaf, like domain/cleanup, because both the shell and
// the interactor ask these questions and neither owns the other.
//
// Scanning is byte-wise, not rune-wise. Every character that means anything here
// — the quotes, the brackets, the comment openers, the parentheses, the
// semicolon, and the bytes a word is made of — is ASCII, and UTF-8 never puts an
// ASCII byte inside a multi-byte sequence. So a multi-byte character can only
// ever be part of a word or of some region this skips, and decoding it would
// change no decision. Offsets are byte offsets, which is what a caller slicing
// the original string wants anyway.
package sqltext

import (
	"iter"
	"strings"
)

// utf8BOM is the byte order mark a Windows editor or export tool puts at the
// start of a file. It is not part of the SQL.
const utf8BOM = "\ufeff"

// Kind is what a Token is.
type Kind int

const (
	// Word is an identifier or keyword: a run of [0-9A-Za-z_]. The scanner does
	// not know which words are keywords; that is the caller's question.
	Word Kind = iota
	// Semicolon is a ";" in code, which is where a statement can end.
	Semicolon
)

// Token is one significant thing found in code — outside every string literal,
// quoted identifier, and comment.
type Token struct {
	Kind Kind
	// Start and End are byte offsets into the scanned string, End exclusive.
	Start int
	End   int
	// Depth is the parenthesis nesting at Start: 0 at the top level of the
	// statement, 1 inside one pair, and so on. It is what tells a CTE's body
	// apart from the statement the CTE feeds.
	Depth int
}

// Text returns the token's text. s must be the string the token was scanned
// from; passing another one yields nonsense or panics, which is why Token
// carries offsets rather than a copy of every word in the statement.
func (t Token) Text(s string) string { return s[t.Start:t.End] }

// Tokens iterates the code tokens of s in order.
func Tokens(s string) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		scan(s, yield)
	}
}

// EndsInsideBlockComment reports whether s stops before the "*/" that would
// close a block comment it opened. A "/*" inside a string literal or a line
// comment opens nothing, which is the reason this cannot be a search for the
// last "/*".
func EndsInsideBlockComment(s string) bool {
	return scan(s, func(Token) bool { return true })
}

// HasWord reports whether word appears in s as a whole word in code. It is not a
// substring search: "RETURNING" does not match a column named `returning_at`,
// and neither a string literal 'returning' nor a commented-out clause counts.
// The comparison is case-insensitive, because SQL keywords are.
func HasWord(s, word string) bool {
	for tok := range Tokens(s) {
		if tok.Kind == Word && strings.EqualFold(tok.Text(s), word) {
			return true
		}
	}
	return false
}

// The statement verbs MainVerb reports. They are the vocabulary of this package
// rather than of any one caller, which is why the shell and the interactor both
// compare against their own spellings of the same words.
const (
	VerbSelect  = "SELECT"
	VerbValues  = "VALUES"
	VerbInsert  = "INSERT"
	VerbUpdate  = "UPDATE"
	VerbDelete  = "DELETE"
	VerbReplace = "REPLACE"
)

// mainVerbs are the statement verbs a WITH can feed. A WITH that reaches one of
// the row-modifying four is DML however many CTEs precede it; one that reaches
// SELECT or VALUES is a query.
var mainVerbs = map[string]bool{
	VerbSelect: true, VerbValues: true,
	VerbInsert: true, VerbUpdate: true, VerbDelete: true, VerbReplace: true,
}

// MainVerb returns the upper-cased main statement verb of s: the first
// SELECT/VALUES/INSERT/UPDATE/DELETE/REPLACE found at parenthesis depth 0. It
// returns "" when none is there.
//
// Depth is what makes this useful on a WITH. The CTE bodies live inside
// parentheses, so skipping everything nested leaves the verb of the statement
// they feed — which is what decides whether `WITH ... UPDATE` writes and
// `WITH ... SELECT` reads.
func MainVerb(s string) string {
	for tok := range Tokens(s) {
		if tok.Kind != Word || tok.Depth != 0 {
			continue
		}
		if upper := strings.ToUpper(tok.Text(s)); mainVerbs[upper] {
			return upper
		}
	}
	return ""
}

// StripNoise removes what can precede the first executable character of a
// statement: a UTF-8 BOM, leading line ("--") and block ("/* */") comments, bare
// ";" empty statements, and the whitespace around all of it. It returns "" when
// nothing executable is left.
//
// sqly classifies a statement by its first keyword, so this has to run first or
// a script opening with a header comment — which is most of them — is rejected
// as not being SQL. The leading ";" matters for the same reason: ";UPDATE t ..."
// is an empty statement followed by an UPDATE, and reading it as a statement
// starting with ";" made it a no-rowset statement that skipped write-back.
func StripNoise(s string) string {
	s = strings.TrimPrefix(s, utf8BOM)
	for {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "--"):
			_, rest, found := strings.Cut(s, "\n")
			if !found {
				return "" // the line comment runs to the end of the input
			}
			s = rest
		case strings.HasPrefix(s, "/*"):
			i := strings.Index(s, "*/")
			if i < 0 {
				return "" // unterminated block comment, nothing executable
			}
			s = s[i+2:]
		case strings.HasPrefix(s, ";"):
			s = s[1:] // leading empty statement
		default:
			return s
		}
	}
}

// LeadingKeyword returns the upper-cased first keyword of s, after StripNoise,
// or "" when nothing executable remains. Only the leading ASCII letters are
// read, so "PRAGMA table_info(x)" yields "PRAGMA" and "VALUES(1)" yields
// "VALUES".
func LeadingKeyword(s string) string {
	s = StripNoise(s)
	i := 0
	for i < len(s) && isLetter(s[i]) {
		i++
	}
	return strings.ToUpper(s[:i])
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isWordByte reports whether c can be part of an identifier or keyword.
func isWordByte(c byte) bool {
	return c == '_' || isLetter(c) || (c >= '0' && c <= '9')
}

// scan walks s, yielding each code token, and reports whether s ends inside an
// unterminated block comment. A yield returning false stops the walk, in which
// case that answer describes only the part reached — which is why
// EndsInsideBlockComment never stops early.
func scan(s string, yield func(Token) bool) (inBlockComment bool) {
	var (
		depth                 int
		inSingle, inDouble    bool
		inBacktick, inBracket bool
		inLineComment         bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inLineComment:
			if c == '\n' {
				inLineComment = false
			}
		case inBlockComment:
			if c == '*' && i+1 < len(s) && s[i+1] == '/' {
				inBlockComment = false
				i++
			}
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case inBacktick:
			// A SQLite backtick-quoted identifier. A doubled backtick escapes one,
			// which this handles by itself: the toggle closes on the first and
			// re-opens on the second.
			if c == '`' {
				inBacktick = false
			}
		case inBracket:
			// A SQLite bracket-quoted identifier. "]" closes it; brackets do not nest.
			if c == ']' {
				inBracket = false
			}
		default:
			switch {
			case c == '\'':
				inSingle = true
			case c == '"':
				inDouble = true
			case c == '`':
				inBacktick = true
			case c == '[':
				inBracket = true
			case c == '-' && i+1 < len(s) && s[i+1] == '-':
				inLineComment = true
				i++
			case c == '/' && i+1 < len(s) && s[i+1] == '*':
				inBlockComment = true
				i++
			case c == '(':
				depth++
			case c == ')':
				if depth > 0 {
					depth--
				}
			case c == ';':
				if !yield(Token{Kind: Semicolon, Start: i, End: i + 1, Depth: depth}) {
					return inBlockComment
				}
			case isWordByte(c):
				start := i
				for i+1 < len(s) && isWordByte(s[i+1]) {
					i++
				}
				if !yield(Token{Kind: Word, Start: start, End: i + 1, Depth: depth}) {
					return inBlockComment
				}
			}
		}
	}
	return inBlockComment
}
