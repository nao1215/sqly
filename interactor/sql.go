package interactor

import (
	"strings"

	"github.com/nao1215/sqly/domain/sqltext"

	"github.com/nao1215/filesql/dialect"
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
// to carry: a category list would have to agree with what sqltext already
// decides, and two spellings of the same rule is one more than a rule can have.
// The type stays because it names where "what kind of statement is this" is
// answered, and because the interactor is wired with it.
type SQL struct{}

// NewSQL return *SQL
func NewSQL() *SQL {
	return &SQL{}
}

// Every question this file asks about a statement is asked after the
// translation has run, so the text is SQLite whatever dialect the caller wrote
// it in. That is why each call names dialect.SQLite rather than the session's
// dialect: by this point the session's dialect is not what the text is.

// unsupportedStatementReason reports why sqly cannot run a statement, or "" when
// it is supported. sqly executes every statement in its own transaction on a
// single in-memory connection, so explicit transaction control cannot span
// statements (a BEGIN nests inside that wrapper, and a SAVEPOINT is discarded when
// the wrapper commits or rolls back), and VACUUM cannot run inside a transaction
// at all. ATTACH/DETACH would let a session read or write external SQLite files
// outside the import/save model, bypassing sqly's in-memory-only contract. These
// are rejected up front with a clear sqly-specific error instead of surfacing
// SQLite's confusing internal message or silently escaping the session model.
func unsupportedStatementReason(stmt string) string {
	switch sqltext.LeadingKeyword(stmt, dialect.SQLite) {
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
	if sqltext.LeadingKeyword(stmt, dialect.SQLite) == sqlTABLE {
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
	switch sqltext.LeadingKeyword(stmt, dialect.SQLite) {
	case sqlSELECT, sqlVALUES, sqlTABLE, sqlEXPLAIN, sqlPRAGMA:
		return true
	case sqlINSERT, sqlUPDATE, sqlDELETE, sqlREPLACE:
		return hasReturningClause(stmt)
	case sqlWITH:
		switch sqltext.MainVerb(stmt, dialect.SQLite) {
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

// hasReturningClause reports whether a DML statement carries a RETURNING
// keyword. SQLite's RETURNING turns an INSERT/UPDATE/DELETE into a
// rowset-producing statement, so the caller runs such a statement through the
// query path.
func hasReturningClause(stmt string) bool {
	return sqltext.HasWord(stmt, "RETURNING", dialect.SQLite)
}
