package interactor

import (
	"testing"
)

func TestContains(t *testing.T) {
	t.Parallel()

	type args struct {
		list []string
		v    string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "success to find value",
			args: args{
				list: []string{"a", "b", "c"},
				v:    "b",
			},
			want: true,
		},
		{
			name: "failed to find value",
			args: args{
				list: []string{"a", "b", "c"},
				v:    "d",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := contains(tt.args.list, tt.args.v); got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimWordGaps(t *testing.T) {
	t.Parallel()

	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "success to trim word gaps. delete head and tail spaces",
			args: args{
				s: "  a  b  c  ",
			},
			want: "a b c",
		},
		{
			name: "success to trim word gaps. delete no spaces",
			args: args{
				s: "a b c",
			},
			want: "a b c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := trimWordGaps(tt.args.s); got != tt.want {
				t.Errorf("trimWordGaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsDDL(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is DDL - CREATE",
			arg:  "CREATE",
			want: true,
		},
		{
			name: "is DDL - DROP",
			arg:  "DROP",
			want: true,
		},
		{
			name: "is not DDL - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not DDL - INSERT",
			arg:  "INSERT",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isDDL(tt.arg); got != tt.want {
				t.Errorf("isDDL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsDML(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is DML - SELECT",
			arg:  "SELECT",
			want: true,
		},
		{
			name: "is DML - INSERT",
			arg:  "INSERT",
			want: true,
		},
		{
			name: "is not DML - CREATE",
			arg:  "CREATE",
			want: false,
		},
		{
			name: "is not DML - DROP",
			arg:  "DROP",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isDML(tt.arg); got != tt.want {
				t.Errorf("isDML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsTCL(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is TCL - BEGIN",
			arg:  "BEGIN",
			want: true,
		},
		{
			name: "is TCL - COMMIT",
			arg:  "COMMIT",
			want: true,
		},
		{
			name: "is not TCL - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not TCL - INSERT",
			arg:  "INSERT",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isTCL(tt.arg); got != tt.want {
				t.Errorf("isTCL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsDCL(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is DCL - GRANT",
			arg:  "GRANT",
			want: true,
		},
		{
			name: "is DCL - REVOKE",
			arg:  "REVOKE",
			want: true,
		},
		{
			name: "is not DCL - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not DCL - INSERT",
			arg:  "INSERT",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isDCL(tt.arg); got != tt.want {
				t.Errorf("isDCL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsExplain(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is EXPLAIN - uppercase",
			arg:  "EXPLAIN",
			want: true,
		},
		{
			name: "is EXPLAIN - lowercase",
			arg:  "explain",
			want: true,
		},
		{
			name: "is not EXPLAIN - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not EXPLAIN - INSERT",
			arg:  "INSERT",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isExplain(tt.arg); got != tt.want {
				t.Errorf("isExplain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsSelect(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is SELECT - uppercase",
			arg:  "SELECT",
			want: true,
		},
		{
			name: "is SELECT - lowercase",
			arg:  "select",
			want: true,
		},
		{
			name: "is not SELECT - INSERT",
			arg:  "INSERT",
			want: false,
		},
		{
			name: "is not SELECT - UPDATE",
			arg:  "UPDATE",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isSelect(tt.arg); got != tt.want {
				t.Errorf("isSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsInsert(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is INSERT - uppercase",
			arg:  "INSERT",
			want: true,
		},
		{
			name: "is INSERT - lowercase",
			arg:  "insert",
			want: true,
		},
		{
			name: "is not INSERT - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not INSERT - UPDATE",
			arg:  "UPDATE",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isInsert(tt.arg); got != tt.want {
				t.Errorf("isInsert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsUpdate(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is UPDATE - uppercase",
			arg:  "UPDATE",
			want: true,
		},
		{
			name: "is UPDATE - lowercase",
			arg:  "update",
			want: true,
		},
		{
			name: "is not UPDATE - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not UPDATE - INSERT",
			arg:  "INSERT",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isUpdate(tt.arg); got != tt.want {
				t.Errorf("isUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsDelete(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is DELETE - uppercase",
			arg:  "DELETE",
			want: true,
		},
		{
			name: "is DELETE - lowercase",
			arg:  "delete",
			want: true,
		},
		{
			name: "is not DELETE - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not DELETE - INSERT",
			arg:  "INSERT",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isDelete(tt.arg); got != tt.want {
				t.Errorf("isDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLIsWithCTE(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "is WITH - uppercase",
			arg:  "WITH",
			want: true,
		},
		{
			name: "is WITH - lowercase",
			arg:  "with",
			want: true,
		},
		{
			name: "is WITH - mixedcase",
			arg:  "With",
			want: true,
		},
		{
			name: "is not WITH - SELECT",
			arg:  "SELECT",
			want: false,
		},
		{
			name: "is not WITH - INSERT",
			arg:  "INSERT",
			want: false,
		},
		{
			name: "is not WITH - WITHIN",
			arg:  "WITHIN",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isWithCTE(tt.arg); got != tt.want {
				t.Errorf("isWithCTE() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLWITHIsDML(t *testing.T) {
	t.Parallel()

	sql := NewSQL()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "WITH is DML - uppercase",
			arg:  "WITH",
			want: true,
		},
		{
			name: "WITH is DML - lowercase",
			arg:  "with",
			want: true,
		},
		{
			name: "SELECT is DML",
			arg:  "SELECT",
			want: true,
		},
		{
			name: "INSERT is DML",
			arg:  "INSERT",
			want: true,
		},
		{
			name: "UPDATE is DML",
			arg:  "UPDATE",
			want: true,
		},
		{
			name: "DELETE is DML",
			arg:  "DELETE",
			want: true,
		},
		{
			name: "EXPLAIN is DML",
			arg:  "EXPLAIN",
			want: true,
		},
		{
			name: "CREATE is not DML",
			arg:  "CREATE",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.isDML(tt.arg); got != tt.want {
				t.Errorf("isDML(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestSQLProducesRowset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stmt string
		want bool
	}{
		// Rowset-producing statements run on the query path.
		{"SELECT produces rows", "SELECT 1 AS x", true},
		{"VALUES produces rows", "VALUES (1), (2)", true},
		{"TABLE shorthand produces rows", "TABLE user", true},
		{"EXPLAIN produces rows", "EXPLAIN SELECT 1", true},
		{"EXPLAIN of DML produces rows", "EXPLAIN UPDATE user SET x=1", true},
		{"PRAGMA produces rows", "PRAGMA table_info(user)", true},
		{"WITH feeding SELECT produces rows", "WITH c AS (SELECT 1 AS id) SELECT * FROM c", true},
		{"WITH feeding VALUES produces rows", "WITH c AS (SELECT 1) VALUES (1)", true},
		{"lowercase select produces rows", "select 1", true},
		{"leading line comment then SELECT", "-- header\nSELECT 1", true},
		{"leading block comment then SELECT", "/* header */ SELECT 1", true},
		{"leading BOM then SELECT", "\ufeffSELECT 1", true},
		// RETURNING turns DML into a rowset.
		{"INSERT RETURNING produces rows", "INSERT INTO t(id) VALUES (1) RETURNING id", true},
		{"UPDATE RETURNING produces rows", "UPDATE t SET x=1 RETURNING *", true},
		{"DELETE RETURNING produces rows", "DELETE FROM t RETURNING *", true},
		{"WITH ... UPDATE RETURNING produces rows", "WITH s AS (SELECT 1 AS id) UPDATE t SET x=1 WHERE id IN (SELECT id FROM s) RETURNING *", true},

		// Non-rowset statements run on the exec path.
		{"INSERT without RETURNING is exec", "INSERT INTO t(id) VALUES (1)", false},
		{"UPDATE without RETURNING is exec", "UPDATE t SET x=1", false},
		{"DELETE without RETURNING is exec", "DELETE FROM t", false},
		{"REPLACE without RETURNING is exec", "REPLACE INTO t(id) VALUES (1)", false},
		{"WITH ... UPDATE without RETURNING is exec", "WITH s AS (SELECT 1 AS id) UPDATE t SET x=1 WHERE id IN (SELECT id FROM s)", false},
		{"WITH ... INSERT without RETURNING is exec", "WITH s AS (SELECT 2 AS id, 'b' AS name) INSERT INTO t SELECT * FROM s", false},
		{"WITH ... DELETE without RETURNING is exec", "WITH d AS (SELECT 1 AS id) DELETE FROM t WHERE id IN (SELECT id FROM d)", false},
		{"CREATE is exec", "CREATE TABLE t(x)", false},
		{"DROP is exec", "DROP TABLE t", false},
		{"ALTER is exec", "ALTER TABLE t ADD COLUMN y", false},
		{"BEGIN is exec", "BEGIN", false},
		{"COMMIT is exec", "COMMIT", false},
		{"ATTACH is exec", "ATTACH DATABASE ':memory:' AS aux", false},
		{"ANALYZE is exec", "ANALYZE", false},
		// A literal 'returning' inside a string is not the RETURNING clause.
		{"INSERT with literal returning value is exec", "INSERT INTO t(note) VALUES ('returning soon')", false},
	}

	sql := NewSQL()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sql.producesRowset(tt.stmt); got != tt.want {
				t.Errorf("producesRowset(%q) = %v, want %v", tt.stmt, got, tt.want)
			}
		})
	}
}

func TestMainStatementVerb(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stmt string
		want string
	}{
		{"WITH feeding UPDATE", "WITH s AS (SELECT 1) UPDATE t SET x=1", sqlUPDATE},
		{"WITH feeding INSERT", "WITH s AS (SELECT 1) INSERT INTO t SELECT * FROM s", sqlINSERT},
		{"WITH feeding DELETE", "WITH s AS (SELECT 1) DELETE FROM t", sqlDELETE},
		{"WITH feeding SELECT", "WITH s AS (SELECT 1) SELECT * FROM s", sqlSELECT},
		{"CTE body SELECT ignored at depth>0", "WITH s AS (SELECT 1 FROM (SELECT 2)) UPDATE t SET x=1", sqlUPDATE},
		{"verb inside string literal ignored", "WITH s AS (SELECT 'UPDATE') SELECT * FROM s", sqlSELECT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mainStatementVerb(tt.stmt); got != tt.want {
				t.Errorf("mainStatementVerb(%q) = %q, want %q", tt.stmt, got, tt.want)
			}
		})
	}
}
