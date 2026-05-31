package interactor

import (
	"slices"
	"strings"
)

const (
	sqlSELECT  = "SELECT"
	sqlINSERT  = "INSERT"
	sqlUPDATE  = "UPDATE"
	sqlDELETE  = "DELETE"
	sqlCREATE  = "CREATE"
	sqlDROP    = "DROP"
	sqlALTER   = "ALTER"
	sqlREINDEX = "REINDEX"
	sqlEXPLAIN = "EXPLAIN"
	sqlWITH    = "WITH"
)

// ddl is Data Definition Language List
type ddl []string

// dml is Data Manipulation Language List
type dml []string

// tcl is Transaction Control Language List
type tcl []string

// dcl is Data Control Language List
type dcl []string

// SQL is sql information
type SQL struct {
	ddl ddl
	dml dml
	tcl tcl
	dcl dcl
}

// NewSQL return *SQL
func NewSQL() *SQL {
	return &SQL{
		ddl: []string{sqlCREATE, sqlDROP, sqlALTER, sqlREINDEX},
		dml: []string{sqlSELECT, sqlINSERT, sqlUPDATE, sqlDELETE, sqlEXPLAIN, sqlWITH},
		tcl: []string{"BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT", "RELEASE"},
		dcl: []string{"GRANT", "REVOKE"},
	}
}

// isDDL return wherther string is ddl or not.
func (sql *SQL) isDDL(s string) bool {
	return contains(sql.ddl, strings.ToUpper(s))
}

// isDML return wherther string is dml or not.
func (sql *SQL) isDML(s string) bool {
	return contains(sql.dml, strings.ToUpper(s))
}

// isTCL return wherther string is tcl or not.
func (sql *SQL) isTCL(s string) bool {
	return contains(sql.tcl, strings.ToUpper(s))
}

// isDCL returns true if the given string represents a Data Control Language (DCL) statement.
func (sql *SQL) isDCL(s string) bool {
	return contains(sql.dcl, strings.ToUpper(s))
}

// isSelect returns true if the given string represents a SELECT statement.
func (sql *SQL) isSelect(s string) bool {
	return strings.ToUpper(s) == sqlSELECT
}

// isInsert returns true if the given string represents an INSERT statement.
func (sql *SQL) isInsert(s string) bool {
	return strings.ToUpper(s) == sqlINSERT
}

// isUpdate returns true if the given string represents an UPDATE statement.
func (sql *SQL) isUpdate(s string) bool {
	return strings.ToUpper(s) == sqlUPDATE
}

// isDelete returns true if the given string represents a DELETE statement.
func (sql *SQL) isDelete(s string) bool {
	return strings.ToUpper(s) == sqlDELETE
}

// isExplain returns true if the given string represents an EXPLAIN statement.
func (sql *SQL) isExplain(s string) bool {
	return strings.ToUpper(s) == sqlEXPLAIN
}

// isWithCTE checks if the statement is a WITH (CTE) query.
func (sql *SQL) isWithCTE(s string) bool {
	return strings.ToUpper(s) == sqlWITH
}

// contains checks if a string exists in a slice of strings.
func contains(list []string, v string) bool {
	return slices.Contains(list, v)
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

// trimWordGaps trims extra spaces between words in a string.
func trimWordGaps(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
