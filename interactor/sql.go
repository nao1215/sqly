package interactor

import (
	"strings"
)

const (
	sqlSELECT    = "SELECT"
	sqlINSERT    = "INSERT"
	sqlUPDATE    = "UPDATE"
	sqlDELETE    = "DELETE"
	sqlEXPLAIN   = "EXPLAIN"
	sqlWITH      = "WITH"
	sqlVALUES    = "VALUES"
	sqlTABLE     = "TABLE"
	sqlPRAGMA    = "PRAGMA"
	sqlREPLACE   = "REPLACE"
	sqlBEGIN     = "BEGIN"
	sqlCOMMIT    = "COMMIT"
	sqlEND       = "END"
	sqlROLLBACK  = "ROLLBACK"
	sqlSAVEPOINT = "SAVEPOINT"
	sqlRELEASE   = "RELEASE"
	sqlVACUUM    = "VACUUM"
	sqlATTACH    = "ATTACH"
	sqlDETACH    = "DETACH"
)

// SQL classifies the statements a session runs.
//
// It holds nothing. Classification reads the statement itself — its leading
// keyword, and for a WITH the verb its CTEs feed — so there is no keyword table
// to carry: a category list would have to agree with what leadingKeyword and
// mainStatementVerb already decide, and two spellings of the same rule is one
// more than a rule can have. The type stays because it names where "what kind
// of statement is this" is answered, and because the interactor is wired with
// it.
type SQL struct{}

// NewSQL return *SQL
func NewSQL() *SQL {
	return &SQL{}
}

// hasReturningClause reports whether a DML statement contains a RETURNING
// keyword outside of string literals, quoted identifiers, and comments. SQLite's
// RETURNING turns an INSERT/UPDATE/DELETE into a rowset-producing statement, so
// the caller runs such a statement through the query path. The scan ignores
// quoted regions so a literal value like 'returning' is not mistaken for the
// clause.
func hasReturningClause(stmt string) bool {
	runes := []rune(stmt)
	var (
		inSingle, inDouble            bool
		inBacktick, inBracket         bool
		inLineComment, inBlockComment bool
	)
	isWordRune := func(r rune) bool {
		return r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')
	}
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case inLineComment:
			if c == '\n' {
				inLineComment = false
			}
		case inBlockComment:
			if c == '*' && i+1 < len(runes) && runes[i+1] == '/' {
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
			if c == '`' {
				inBacktick = false
			}
		case inBracket:
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
			case c == '-' && i+1 < len(runes) && runes[i+1] == '-':
				inLineComment = true
				i++
			case c == '/' && i+1 < len(runes) && runes[i+1] == '*':
				inBlockComment = true
				i++
			case isWordRune(c):
				// Read a whole identifier token and compare it to RETURNING, so the
				// match respects word boundaries (e.g. "RETURNING_AT" does not match).
				start := i
				for i+1 < len(runes) && isWordRune(runes[i+1]) {
					i++
				}
				if strings.EqualFold(string(runes[start:i+1]), "RETURNING") {
					return true
				}
			}
		}
	}
	return false
}

// stripSQLNoise removes a leading UTF-8 BOM, any leading line ("--") or block
// ("/* */") comments, leading empty statements (bare ";"), plus surrounding
// whitespace, returning the first executable portion. The batch and --sql-file
// paths already accept a BOM and leading comments; the direct --sql path classifies
// a statement by its first keyword, so it must strip the same noise to stay
// consistent. A leading ";" is an empty statement: dropping it makes ";SELECT 1"
// classify and run as the SELECT rather than as a no-rowset statement that discards
// the query.
func stripSQLNoise(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	for {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "--"):
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				return "" // line comment runs to the end of the input
			}
			s = s[i+1:]
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

// leadingKeyword returns the upper-cased first keyword of a statement after a BOM
// and leading comments are stripped, or "" when nothing executable remains. Only
// the leading ASCII letters are read, so "PRAGMA table_info(x)" yields "PRAGMA"
// and "VALUES(1)" yields "VALUES".
func leadingKeyword(s string) string {
	s = stripSQLNoise(s)
	i := 0
	for i < len(s) && ((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z')) {
		i++
	}
	return strings.ToUpper(s[:i])
}

// mainStatementVerb returns the main statement verb of a possibly WITH-prefixed
// statement: the first SELECT/VALUES/INSERT/UPDATE/DELETE/REPLACE token found at
// parenthesis depth 0, outside string literals, quoted identifiers, and comments.
// The CTE bodies live inside parentheses (depth > 0), so this skips them and
// returns the verb of the statement the CTEs feed. It lets a WITH ... UPDATE run
// as DML and a WITH ... SELECT run as a query.
func mainStatementVerb(stmt string) string {
	runes := []rune(stmt)
	var (
		depth                         int
		inSingle, inDouble            bool
		inBacktick, inBracket         bool
		inLineComment, inBlockComment bool
	)
	isWordRune := func(r rune) bool {
		return r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')
	}
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case inLineComment:
			if c == '\n' {
				inLineComment = false
			}
		case inBlockComment:
			if c == '*' && i+1 < len(runes) && runes[i+1] == '/' {
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
			if c == '`' {
				inBacktick = false
			}
		case inBracket:
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
			case c == '-' && i+1 < len(runes) && runes[i+1] == '-':
				inLineComment = true
				i++
			case c == '/' && i+1 < len(runes) && runes[i+1] == '*':
				inBlockComment = true
				i++
			case c == '(':
				depth++
			case c == ')':
				if depth > 0 {
					depth--
				}
			case depth == 0 && isWordRune(c):
				start := i
				for i+1 < len(runes) && isWordRune(runes[i+1]) {
					i++
				}
				switch strings.ToUpper(string(runes[start : i+1])) {
				case sqlSELECT, sqlVALUES, sqlINSERT, sqlUPDATE, sqlDELETE, sqlREPLACE:
					return strings.ToUpper(string(runes[start : i+1]))
				}
			}
		}
	}
	return ""
}

// unsupportedStatementReason reports why sqly cannot run a statement, or "" when
// it is supported. sqly executes every statement in its own transaction on a
// single in-memory connection, so explicit transaction control cannot span
// statements (a BEGIN nests inside that wrapper, and a SAVEPOINT is discarded when
// the wrapper commits or rolls back), and VACUUM cannot run inside a transaction
// at all. ATTACH/DETACH would let a session read or write external SQLite files
// outside the import/save model, bypassing sqly's in-memory-only contract. These
// are rejected up front with a clear sqly-specific error instead of surfacing
// SQLite's confusing internal message or silently escaping the session model.
// The input must already be noise-stripped and normalized.
func unsupportedStatementReason(stmt string) string {
	switch leadingKeyword(stmt) {
	case sqlBEGIN, sqlCOMMIT, sqlEND, sqlROLLBACK, sqlSAVEPOINT, sqlRELEASE:
		// END (and END TRANSACTION) is an alias for COMMIT, so it must be rejected
		// with the same sqly-specific message rather than falling through to
		// SQLite's "cannot commit - no transaction is active".
		return "explicit transaction control is not supported; sqly runs each statement in its own transaction"
	case sqlVACUUM:
		return "VACUUM is not supported; sqly runs every statement inside a transaction, which SQLite forbids for VACUUM"
	case sqlATTACH, sqlDETACH:
		return "ATTACH/DETACH DATABASE is not supported; sqly runs an in-memory session, so import files as tables instead"
	}
	return ""
}

// normalizeStatement rewrites a SQLite shorthand the pure-Go engine does not
// accept into an equivalent statement it does. The PostgreSQL-style "TABLE name"
// shorthand (which the sqlite3 CLI accepts but modernc.org/sqlite rejects) is
// rewritten to "SELECT * FROM name". The input must already be noise-stripped.
func normalizeStatement(stmt string) string {
	if leadingKeyword(stmt) == sqlTABLE {
		if rest := strings.TrimSpace(stmt[len(sqlTABLE):]); rest != "" {
			return "SELECT * FROM " + rest
		}
	}
	return stmt
}

// producesRowset reports whether a statement returns a result set (so it runs on
// the query path) rather than only an affected-row count (the exec path). sqly
// targets SQLite, so every valid SQLite statement is accepted and routed by shape
// instead of being rejected by category: SELECT/VALUES/TABLE/EXPLAIN/PRAGMA and a
// WITH that feeds a SELECT/VALUES produce rows, an INSERT/UPDATE/DELETE/REPLACE
// produces rows only with RETURNING, and everything else (DDL, transaction
// control, ATTACH, ANALYZE, ...) runs as a no-rowset statement.
func (sql *SQL) producesRowset(stmt string) bool {
	switch leadingKeyword(stmt) {
	case sqlSELECT, sqlVALUES, sqlTABLE, sqlEXPLAIN, sqlPRAGMA:
		return true
	case sqlINSERT, sqlUPDATE, sqlDELETE, sqlREPLACE:
		return hasReturningClause(stmt)
	case sqlWITH:
		switch mainStatementVerb(stmt) {
		case sqlINSERT, sqlUPDATE, sqlDELETE, sqlREPLACE:
			return hasReturningClause(stmt)
		default:
			// WITH ... SELECT/VALUES, or a WITH whose verb could not be found, runs
			// on the query path so its rows are returned.
			return true
		}
	default:
		return false
	}
}
