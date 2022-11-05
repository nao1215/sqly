package shell

// ddl is Data Definition Language List
type ddl []string

// dml is Data Manipulation Language List
type dml []string

// tcl is Transaction Control Language List
type tcl []string

// dcl is Data Control Language List
type dcl []string

// Completion is sqly shell completion
type Completion struct {
	ddl ddl
	dml dml
	tcl tcl
	dcl dcl
}

// NewCompletion return *Completion
func NewCompletion() *Completion {
	return &Completion{
		ddl: []string{"CREATE", "DROP", "ALTER", "REINDEX"},
		dml: []string{"SELECT", "INSERT", "UPDATE", "DELETE", "EXPLAIN"},
		tcl: []string{"BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT", "RELEASE"},
		dcl: []string{"GRANT", "REVOKE"},
	}
}

// isDDL return wherther string is ddl or not.
func (c *Completion) isDDL(s string) bool {
	return false
}
