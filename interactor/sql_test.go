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
