package interactor

import "slices"

import "strings"

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

// trimWordGaps trims extra spaces between words in a string.
func trimWordGaps(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
