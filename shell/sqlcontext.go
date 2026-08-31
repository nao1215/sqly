package shell

import (
	"strings"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/sqltext"
)

// sqlPosition is what the statement says about the identifier being typed at
// the cursor. It is the question completion needs answered: "SELECT * FROM u"
// and "SELECT u" are the same three characters typed after the same statement
// verb, and only one of them can be a table.
type sqlPosition int

const (
	// posAny is "the statement does not say". Completion offers everything, the
	// way it does on an empty line.
	posAny sqlPosition = iota
	// posTable is a table position: right after FROM, JOIN, INTO, UPDATE, or a
	// comma continuing a FROM list. A column name is never valid here.
	posTable
	// posColumn is a column position: inside a SELECT list, a WHERE/ON/HAVING
	// condition, a GROUP BY or ORDER BY, or an UPDATE ... SET.
	posColumn
)

// tableRef is one table the statement names, with the alias it was given. The
// alias is "" when the statement did not give it one.
type tableRef struct {
	name  string
	alias string
}

// sqlAnalysis is what the text before the cursor says about the identifier
// being typed there.
type sqlAnalysis struct {
	// position is the kind of identifier the statement expects at the cursor.
	position sqlPosition
	// refs are the tables the statement names, in the order it names them.
	refs []tableRef
	// qualifier is the part of the identifier before its "." — the "u" of
	// "u.na". It is "" when the identifier being typed is unqualified.
	qualifier string
	// partial is the identifier being typed: the part after the "." when there
	// is a qualifier, otherwise the whole word.
	partial string
}

// tableOf returns the table the qualifier names, resolving it as an alias
// first and as a table name second. known is the session's table-name set,
// which is what makes an unresolvable qualifier distinguishable from one that
// simply has not been typed in full yet. The match is case-insensitive because
// SQL identifiers are.
//
// Why the alias wins: a statement may alias one table with another's name
// ("FROM orders users"), and inside it the alias is what "users." means.
func (a sqlAnalysis) tableOf(qualifier string, known []string) (string, bool) {
	if qualifier == "" {
		return "", false
	}
	for _, ref := range a.refs {
		if strings.EqualFold(ref.alias, qualifier) {
			return ref.name, true
		}
	}
	for _, name := range known {
		if strings.EqualFold(name, qualifier) {
			return name, true
		}
	}
	return "", false
}

// tablesInScope returns the names of the tables the statement references, in
// the order it names them and without repeats. It is what tells a column that
// belongs to this statement from one that merely exists in the session.
func (a sqlAnalysis) tablesInScope() []string {
	seen := make(map[string]bool, len(a.refs))
	out := make([]string, 0, len(a.refs))
	for _, ref := range a.refs {
		key := strings.ToLower(ref.name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ref.name)
	}
	return out
}

// tableIntroducers are the keywords whose next identifier names a table.
// UPDATE and INTO are here for the same reason FROM is: what follows them is
// the table being written to.
var tableIntroducers = map[string]bool{
	"FROM":   true,
	"JOIN":   true,
	"INTO":   true,
	"UPDATE": true,
}

// columnIntroducers are the keywords after which an identifier names a column
// (or an expression over columns). They are also what ends a FROM list: once
// one is seen, a following identifier is no longer a table or an alias.
//
// BY covers GROUP BY and ORDER BY without either word needing its own entry,
// because "GROUP"/"ORDER" alone are never followed directly by a column.
var columnIntroducers = map[string]bool{
	"SELECT": true, "WHERE": true, "ON": true, "HAVING": true, "BY": true,
	"SET": true, "USING": true, "AND": true, "OR": true, "NOT": true,
	"DISTINCT": true, "RETURNING": true, kwCase: true, "WHEN": true,
	"THEN": true, "ELSE": true, "BETWEEN": true, "LIKE": true, "IN": true,
	"VALUES": true, "LIMIT": true, "OFFSET": true, "AS": true,
}

// joinModifiers are the words that decorate a JOIN without introducing
// anything themselves. Left in the table-list state they would be read as a
// table's alias: "FROM users u LEFT JOIN orders" would make "LEFT" the second
// table.
var joinModifiers = map[string]bool{
	"INNER": true, "LEFT": true, "RIGHT": true, "FULL": true,
	"CROSS": true, "NATURAL": true, "OUTER": true,
}

// tableListState is where the walk is inside a FROM/JOIN item list.
type tableListState int

const (
	// stStart is before any clause keyword has been seen: the statement has not
	// said what it is yet, so nothing is claimed about what comes next.
	stStart tableListState = iota
	// stOutside is inside a clause that takes columns — a SELECT list, a
	// condition, a GROUP BY — and outside any table list.
	stOutside
	// stWantTable is right after FROM/JOIN/INTO/UPDATE or a comma continuing
	// the list: the next identifier is a table name.
	stWantTable
	// stWantAlias is just after a table name: an identifier here is its alias.
	stWantAlias
	// stNamed is after a table and its alias: only a comma starts another item.
	stNamed
)

// analyzeSQL reads the text before the cursor and reports what the statement
// expects there.
//
// It is deliberately not a parser. Completion runs on every keystroke over text
// that is, by definition, unfinished, so what is needed is a reading of the
// words already typed that degrades to "no opinion" rather than to an error.
// The words come from sqltext, so a keyword inside a string literal, a quoted
// identifier, or a comment is not mistaken for one in code, and the dialect
// decides how those regions are spelled.
func analyzeSQL(text string, d dialect.Dialect) sqlAnalysis {
	// Only the statement being typed matters. An earlier one on the same line
	// has already ended, and its FROM clause names tables this one may not.
	if _, remainder := splitSQLStatements(text, d); len(remainder) < len(text) {
		text = remainder
	}

	// The word at the cursor is what is being completed, not a decided part of
	// the statement, so the walk stops before it. It is taken whole (as the
	// prompt takes it) rather than as tokens, because "u.na" is one word to the
	// line editor and two words to the tokenizer.
	word := currentCompletionWord(text)
	wordStart := len(text) - len(word)

	var (
		analysis sqlAnalysis
		prev     sqltext.Token
		havePrev bool
	)
	state := stStart
	analysis.qualifier, analysis.partial = splitQualifiedName(word)

	for tok := range sqltext.Tokens(text, d) {
		if tok.Kind != sqltext.Word || tok.Start >= wordStart {
			continue
		}
		separator := ""
		if havePrev {
			separator = separatorBetween(text, prev.End, tok.Start)
		}
		prev, havePrev = tok, true

		// "main.users" is one name in two tokens. Without this the part after
		// the dot would be read as the first part's alias.
		if separator == "." {
			if state == stWantAlias && len(analysis.refs) > 0 {
				analysis.refs[len(analysis.refs)-1].name = tok.Text(text)
			}
			continue
		}

		upper := strings.ToUpper(tok.Text(text))
		switch {
		case tableIntroducers[upper]:
			state = stWantTable
		case columnIntroducers[upper]:
			// AS names this table's alias rather than ending the item, but only
			// where an item is open.
			if upper == "AS" && (state == stWantAlias || state == stNamed) {
				state = stWantAlias
				continue
			}
			state = stOutside
		case joinModifiers[upper]:
			// Decoration around a JOIN: it names nothing, so the state is left
			// as it was.
		default:
			state = analysis.observeIdentifier(state, text, tok, strings.Contains(separator, ","))
		}
	}

	analysis.position = positionAt(state, text, wordStart, prev, havePrev)
	return analysis
}

// observeIdentifier records the identifier tok in whatever table-list state the
// walk is in, and returns the state that follows it. commaBefore says whether a
// "," separates tok from the word before it, which is what continues a FROM
// list ("FROM users u, orders") rather than naming an alias.
func (a *sqlAnalysis) observeIdentifier(state tableListState, text string, tok sqltext.Token, commaBefore bool) tableListState {
	switch {
	case state == stWantTable, commaBefore && (state == stWantAlias || state == stNamed):
		a.refs = append(a.refs, tableRef{name: tok.Text(text)})
		return stWantAlias
	case state == stWantAlias:
		if len(a.refs) > 0 {
			a.refs[len(a.refs)-1].alias = tok.Text(text)
		}
		return stNamed
	default:
		return state
	}
}

// positionAt reports what the cursor's position expects, given the state the
// walk ended in and whether a comma separates the last decided word from the
// word being typed.
func positionAt(state tableListState, text string, wordStart int, prev sqltext.Token, havePrev bool) sqlPosition {
	commaBefore := havePrev && strings.Contains(separatorBetween(text, prev.End, wordStart), ",")

	switch state {
	case stWantTable:
		return posTable
	case stWantAlias, stNamed:
		// A comma continues the FROM list, so what follows names a table. Without
		// one the word could be this table's alias or the next clause keyword,
		// and the statement does not yet say which.
		if commaBefore {
			return posTable
		}
		return posAny
	case stOutside:
		return posColumn
	default: // stStart: no clause keyword has been seen, so nothing is claimed
		return posAny
	}
}

// separatorBetween returns the punctuation that lies between two code tokens:
// the text between them with whitespace removed. It answers the two questions
// the word tokenizer cannot, since it yields only words and semicolons —
// whether a "." joins two words into one qualified name, and whether a ","
// starts the next item of a list.
//
// A region the tokenizer skipped (a string literal, a quoted identifier, a
// comment) can also fall in the gap and can hold either character as ordinary
// text. Such a gap is reported as empty rather than guessed at: reading
// "WHERE note = 'a,b' AND x" as a list would be worse than having no opinion.
func separatorBetween(text string, start, end int) string {
	if start < 0 || end > len(text) || start >= end {
		return ""
	}
	gap := text[start:end]
	if strings.ContainsAny(gap, "'\"`[") || strings.Contains(gap, "--") || strings.Contains(gap, "/*") {
		return ""
	}
	return strings.Join(strings.Fields(gap), "")
}

// splitQualifiedName splits the word being typed into its qualifier and the
// part after the ".": "u.na" is ("u", "na"), "u." is ("u", ""), and "na" is
// ("", "na").
//
// A word that is not a qualified identifier keeps the whole of itself as the
// partial. A path ("./data.csv"), a decimal number, and a word whose qualifier
// would be empty (".mode") all reach here, and none of them names a column of
// anything.
func splitQualifiedName(word string) (qualifier, partial string) {
	dot := strings.LastIndex(word, ".")
	if dot <= 0 {
		return "", word
	}
	qualifier = word[:dot]
	if !isIdentifier(qualifier) {
		return "", word
	}
	return qualifier, word[dot+1:]
}

// isIdentifier reports whether s is a bare SQL identifier: a non-empty run of
// letters, digits, and underscores that does not start with a digit. It is what
// keeps "./data" and "1.5" from being read as a qualified name.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r > 127:
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
