package usecase

import "strings"

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
		ddl: []string{"CREATE", "DROP", "ALTER", "REINDEX"},
		dml: []string{"SELECT", "INSERT", "UPDATE", "DELETE", "EXPLAIN"},
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

// isDCL return wherther string is dcl or not.
func (sql *SQL) isDCL(s string) bool {
	return contains(sql.dcl, strings.ToUpper(s))
}

func (sql *SQL) isSelect(s string) bool {
	return strings.ToUpper(s) == "SELECT"
}

func (sql *SQL) isInsert(s string) bool {
	return strings.ToUpper(s) == "INSERT"
}

func (sql *SQL) isUpdate(s string) bool {
	return strings.ToUpper(s) == "UPDATE"
}

func (sql *SQL) isDelete(s string) bool {
	return strings.ToUpper(s) == "DELETE"
}

func (sql *SQL) isExpalin(s string) bool {
	return strings.ToUpper(s) == "EXPLAIN"
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if v == s {
			return true
		}
	}
	return false
}

func trimWordGaps(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
