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
// It is a leaf, like domain/cleanup, because both the shell and the interactor
// ask these questions and neither owns the other. Its one dependency is the
// dialect vocabulary of filesql, which is where sqly's --dialect value comes
// from: every function here takes the dialect the text is written in, because
// the answers differ by it. "#" opens a line comment in MySQL and GoogleSQL and
// is the start of an operator in PostgreSQL; a backslash escapes a quote in
// MySQL and stands for itself in SQLite; PostgreSQL alone writes a dollar-quoted
// string and nests its block comments. Reading every dialect by SQLite's rules
// cut a statement in the wrong place, or invented one that was never there.
//
// A caller holding text that has already been translated passes dialect.SQLite,
// which is what the text is by then.
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

	"github.com/nao1215/filesql/dialect"
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
	// String is a string literal, from its opening delimiter to its closing
	// one, or to the end of the text when it was never closed.
	//
	// A doubled quote inside a literal closes one and opens the next, so such a
	// literal arrives as two String regions that touch. Nothing between them is
	// code, which is what the distinction is for.
	String
	// QuotedIdentifier is a name written in quotes, delimiters included. Which
	// quotes those are is the dialect's business: a backtick everywhere but
	// PostgreSQL, a bracket in SQLite, and a double quote everywhere MySQL's
	// reading of it as a string does not apply.
	QuotedIdentifier
	// Comment is a line or block comment, opener included, and closer where it
	// has one -- for a line comment that is the newline ending it.
	Comment
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

// Tokens iterates the code tokens of s in order, reading s as d spells SQL: the
// words and semicolons that are code, and nothing that is a string literal, a
// quoted identifier, or a comment. It is what a caller asking "what does this
// statement say" wants, because those regions say nothing.
//
// See Regions for the walk that reports them too.
func Tokens(s string, d dialect.Dialect) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		scan(s, syntaxOf(d), func(tok Token) bool {
			if tok.Kind != Word && tok.Kind != Semicolon {
				return true
			}
			return yield(tok)
		})
	}
}

// Regions iterates everything s is made of, in order and without overlapping:
// the code tokens Tokens yields, and the string literals, quoted identifiers,
// and comments it does not.
//
// It is the walk for a caller that needs to say something about those regions
// rather than see past them -- coloring them on screen, counting them, pointing
// at one. Whitespace, operators, and parentheses are not reported, so a caller
// covering the whole of s has to fill the gaps between what it is given.
func Regions(s string, d dialect.Dialect) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		scan(s, syntaxOf(d), yield)
	}
}

// OpenDepth reports how many parentheses are open at the end of s: the nesting
// a statement continued on the next line would be written at.
//
// A parenthesis inside a string literal, a quoted identifier, or a comment is
// text and is not counted, which is the reason this cannot be a subtraction of
// two counts over the raw string.
func OpenDepth(s string, d dialect.Dialect) int {
	return scanned(s, syntaxOf(d), func(Token) bool { return true }).depth
}

// EndsInsideBlockComment reports whether s stops before the "*/" that would
// close a block comment it opened. A "/*" inside a string literal or a line
// comment opens nothing, which is the reason this cannot be a search for the
// last "/*".
func EndsInsideBlockComment(s string, d dialect.Dialect) bool {
	return scan(s, syntaxOf(d), func(Token) bool { return true })
}

// HasWord reports whether word appears in s as a whole word in code. It is not a
// substring search: "RETURNING" does not match a column named `returning_at`,
// and neither a string literal 'returning' nor a commented-out clause counts.
// The comparison is case-insensitive, because SQL keywords are.
func HasWord(s, word string, d dialect.Dialect) bool {
	for tok := range Tokens(s, d) {
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
func MainVerb(s string, d dialect.Dialect) string {
	for tok := range Tokens(s, d) {
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
func StripNoise(s string, d dialect.Dialect) string {
	rules := syntaxOf(d)
	s = strings.TrimPrefix(s, utf8BOM)
	for {
		s = strings.TrimSpace(s)
		switch {
		case rules.opensLineComment(s):
			_, rest, found := strings.Cut(s, "\n")
			if !found {
				return "" // the line comment runs to the end of the input
			}
			s = rest
		case strings.HasPrefix(s, "/*"):
			rest, closed := afterBlockComment(s, rules)
			if !closed {
				return "" // unterminated block comment, nothing executable
			}
			s = rest
		case strings.HasPrefix(s, ";"):
			s = s[1:] // leading empty statement
		default:
			return s
		}
	}
}

// afterBlockComment is what follows the block comment s opens, and whether it
// closed. PostgreSQL nests them, so the first "*/" is not always the end.
func afterBlockComment(s string, rules syntax) (rest string, closed bool) {
	depth := 0
	for i := 0; i+1 < len(s); i++ {
		switch {
		case s[i] == '/' && s[i+1] == '*':
			if depth == 0 || rules.nestedComment {
				depth++
				i++
			}
		case s[i] == '*' && s[i+1] == '/':
			depth--
			if depth == 0 {
				return s[i+2:], true
			}
			i++
		}
	}
	return "", false
}

// LeadingKeyword returns the upper-cased first keyword of s, after StripNoise,
// or "" when nothing executable remains. Only the leading ASCII letters are
// read, so "PRAGMA table_info(x)" yields "PRAGMA" and "VALUES(1)" yields
// "VALUES".
func LeadingKeyword(s string, d dialect.Dialect) string {
	s = StripNoise(s, d)
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

// syntax is the lexical rules that differ between the dialects sqly accepts.
// Only what changes where a statement ends is here: what opens a comment, what
// closes a quoted region, and what a backslash means inside one.
type syntax struct {
	// hashComment says "#" opens a line comment. MySQL and GoogleSQL write one
	// that way; in PostgreSQL "#" begins the "#>" and "#>>" operators, and in
	// SQLite it is nothing.
	hashComment bool
	// backslashEscape says a backslash inside a quoted string escapes the byte
	// after it, so 'a\';b' is one string rather than two.
	backslashEscape bool
	// escapeStringPrefix says an E before a single quote turns on the escaping
	// above for that string alone, which is how PostgreSQL writes one.
	escapeStringPrefix bool
	// dollarQuote says a dollar-tagged string is a string, which PostgreSQL
	// alone writes.
	dollarQuote bool
	// nestedComment says a block comment nests, which PostgreSQL alone does.
	nestedComment bool
	// bracketIdent says a bracketed name is an identifier. SQLite alone reads it
	// that way; elsewhere brackets subscript an array.
	bracketIdent bool
	// backtickIdent says a backtick quotes an identifier. PostgreSQL alone does
	// not.
	backtickIdent bool
	// doubleQuoteIsString says a double-quoted run is a string rather than a
	// name, which is MySQL's default reading and no other dialect's. It changes
	// nothing about where the run ends, only what it is called.
	doubleQuoteIsString bool
	// tripleQuote says a tripled quote opens a string, which GoogleSQL alone
	// writes.
	tripleQuote bool
	// dashNeedsBlank says a double dash opens a line comment only when a blank
	// or a control character follows it, which is MySQL's rule and no other
	// dialect's: MySQL answers 2 for "SELECT 1--1" where PostgreSQL answers 1.
	dashNeedsBlank bool
	// backtickEscape says a backslash escapes inside a backtick-quoted
	// identifier, which GoogleSQL alone does.
	backtickEscape bool
}

// syntaxOf is the rules of one dialect. An unknown one reads as SQLite, which is
// the engine underneath and the dialect a caller means when it has none.
func syntaxOf(d dialect.Dialect) syntax {
	switch d {
	case dialect.MySQL:
		return syntax{
			hashComment: true, backslashEscape: true, backtickIdent: true,
			dashNeedsBlank: true, doubleQuoteIsString: true,
		}
	case dialect.PostgreSQL:
		return syntax{escapeStringPrefix: true, dollarQuote: true, nestedComment: true}
	case dialect.GoogleSQL:
		return syntax{
			hashComment: true, backslashEscape: true, backtickIdent: true,
			tripleQuote: true, backtickEscape: true,
		}
	default:
		return syntax{bracketIdent: true, backtickIdent: true}
	}
}

// scanner walks one string under one dialect's rules.
type scanner struct {
	s     string
	rules syntax
	i     int
	depth int
	// commentDepth is how deeply nested the block comment now open is, and zero
	// when none is.
	commentDepth int
}

// scan walks s, yielding each code token, and reports whether s ends inside an
// unterminated block comment. A yield returning false stops the walk, in which
// case that answer describes only the part reached -- which is why
// EndsInsideBlockComment never stops early.
func scan(s string, rules syntax, yield func(Token) bool) (inBlockComment bool) {
	return scanned(s, rules, yield).commentDepth > 0
}

// scanned is scan with the scanner handed back, for a caller that wants what
// the walk ended in rather than what it found on the way: the parenthesis
// nesting still open, or whether a block comment is still unclosed.
func scanned(s string, rules syntax, yield func(Token) bool) *scanner {
	sc := &scanner{s: s, rules: rules}
	for sc.i < len(sc.s) {
		start := sc.i
		if kind, ok := sc.skipRegion(); ok {
			if !yield(Token{Kind: kind, Start: start, End: sc.i, Depth: sc.depth}) {
				return sc
			}
			continue
		}
		switch c := sc.s[sc.i]; {
		case c == '(':
			sc.depth++
			sc.i++
		case c == ')':
			if sc.depth > 0 {
				sc.depth--
			}
			sc.i++
		case c == ';':
			if !yield(Token{Kind: Semicolon, Start: sc.i, End: sc.i + 1, Depth: sc.depth}) {
				return sc
			}
			sc.i++
		case isWordByte(c):
			start := sc.i
			for sc.i < len(sc.s) && isWordByte(sc.s[sc.i]) {
				sc.i++
			}
			if !yield(Token{Kind: Word, Start: start, End: sc.i, Depth: sc.depth}) {
				return sc
			}
		default:
			sc.i++
		}
	}
	return sc
}

// skipRegion steps over a comment or a quoted region beginning at the cursor,
// reporting whether it stepped over one. A region that never closes runs to the
// end of the input, which is what leaves an unterminated block comment open for
// EndsInsideBlockComment to report.
func (sc *scanner) skipRegion() (Kind, bool) {
	rest := sc.s[sc.i:]
	switch {
	case sc.rules.opensLineComment(rest):
		sc.skipLineComment()
		return Comment, true
	case strings.HasPrefix(rest, "/*"):
		sc.skipBlockComment()
		return Comment, true
	case sc.rules.tripleQuote && (strings.HasPrefix(rest, tripleSingle) || strings.HasPrefix(rest, tripleDouble)):
		sc.skipQuoted(rest[:3], 3, sc.rules.backslashEscape)
		return String, true
	case rest[0] == '\'':
		sc.skipQuoted("'", 1, sc.rules.backslashEscape || sc.escapeStringOpens())
		return String, true
	case rest[0] == '"':
		sc.skipQuoted(`"`, 1, sc.rules.backslashEscape)
		// A double-quoted run is a name in standard SQL and a string in MySQL,
		// which is the one dialect that reads it that way by default.
		if sc.rules.doubleQuoteIsString {
			return String, true
		}
		return QuotedIdentifier, true
	case sc.rules.backtickIdent && rest[0] == '`':
		sc.skipQuoted("`", 1, sc.rules.backtickEscape)
		return QuotedIdentifier, true
	case sc.rules.bracketIdent && rest[0] == '[':
		sc.skipQuoted("]", 1, false)
		return QuotedIdentifier, true
	case sc.rules.dollarQuote && rest[0] == '$':
		if sc.skipDollarQuoted() {
			return String, true
		}
		return Word, false
	default:
		return Word, false
	}
}

// opensLineComment reports whether s opens a line comment under these rules.
// MySQL alone asks for a blank after the double dash, which is why "1--1" is
// arithmetic there and a comment everywhere else.
func (r syntax) opensLineComment(s string) bool {
	if r.hashComment && strings.HasPrefix(s, "#") {
		return true
	}
	if !strings.HasPrefix(s, "--") {
		return false
	}
	if !r.dashNeedsBlank {
		return true
	}
	return len(s) == 2 || s[2] <= ' '
}

// The GoogleSQL string delimiters that are three characters long.
const (
	tripleSingle = "'''"
	tripleDouble = `"""`
)

// escapeStringOpens reports whether the single quote at the cursor opens a
// PostgreSQL escape string, whose backslash escapes the byte after it.
func (sc *scanner) escapeStringOpens() bool {
	if !sc.rules.escapeStringPrefix || sc.i == 0 {
		return false
	}
	if c := sc.s[sc.i-1]; c != 'E' && c != 'e' {
		return false
	}
	// The E must stand on its own rather than end a word, so a word ending in E
	// beside a string is not read as an escape prefix.
	return sc.i < 2 || !isWordByte(sc.s[sc.i-2])
}

// skipLineComment steps to the end of the line, or of the input.
func (sc *scanner) skipLineComment() {
	if end := strings.IndexByte(sc.s[sc.i:], '\n'); end >= 0 {
		sc.i += end + 1
		return
	}
	sc.i = len(sc.s)
}

// skipBlockComment steps past the comment the cursor opens, following the
// nesting where the dialect nests.
func (sc *scanner) skipBlockComment() {
	sc.commentDepth = 1
	sc.i += 2
	for sc.i+1 < len(sc.s) {
		switch {
		case sc.rules.nestedComment && sc.s[sc.i] == '/' && sc.s[sc.i+1] == '*':
			sc.commentDepth++
			sc.i += 2
		case sc.s[sc.i] == '*' && sc.s[sc.i+1] == '/':
			sc.commentDepth--
			sc.i += 2
			if sc.commentDepth == 0 {
				return
			}
		default:
			sc.i++
		}
	}
	sc.i = len(sc.s)
}

// skipQuoted steps past a region the cursor opens, which closer ends. The opener
// is as long as the closer for a quote that is its own delimiter, which is what
// makes a tripled quote one region rather than three.
func (sc *scanner) skipQuoted(closer string, opener int, backslashEscapes bool) {
	sc.i += opener
	for sc.i < len(sc.s) {
		if backslashEscapes && sc.s[sc.i] == '\\' {
			sc.i += 2
			continue
		}
		if strings.HasPrefix(sc.s[sc.i:], closer) {
			sc.i += len(closer)
			return
		}
		sc.i++
	}
}

// skipDollarQuoted steps past a dollar-tagged string, reporting whether the
// cursor was on one. A dollar that opens no tag is an ordinary character: it
// begins a PostgreSQL parameter such as the first one.
func (sc *scanner) skipDollarQuoted() bool {
	rest := sc.s[sc.i:]
	end := strings.IndexByte(rest[1:], '$')
	if end < 0 {
		return false
	}
	tag := rest[:end+2] // the dollar, the tag, and the closing dollar
	name := tag[1 : len(tag)-1]
	for i := range len(name) {
		if !isWordByte(name[i]) || (i == 0 && name[i] >= '0' && name[i] <= '9') {
			return false // a parameter, not a tag
		}
	}
	body := rest[len(tag):]
	closing := strings.Index(body, tag)
	if closing < 0 {
		sc.i = len(sc.s)
		return true
	}
	sc.i += len(tag) + closing + len(tag)
	return true
}
