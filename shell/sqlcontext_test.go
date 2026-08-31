package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
)

// TestAnalyzeSQLPosition covers what the statement says about the identifier
// being typed: whether it can be a table, a column, or anything at all.
func TestAnalyzeSQLPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want sqlPosition
	}{
		{name: "an empty line says nothing", text: "", want: posAny},
		{name: "a bare word says nothing", text: "sel", want: posAny},
		{name: "after SELECT an identifier is a column", text: "SELECT ", want: posColumn},
		{name: "inside a SELECT list an identifier is a column", text: "SELECT id, na", want: posColumn},
		{name: "after FROM an identifier is a table", text: "SELECT * FROM ", want: posTable},
		{name: "a partial table name after FROM is still a table", text: "SELECT * FROM us", want: posTable},
		{name: "after JOIN an identifier is a table", text: "SELECT * FROM users JOIN ", want: posTable},
		{name: "after LEFT JOIN an identifier is a table", text: "SELECT * FROM users u LEFT JOIN ord", want: posTable},
		{name: "a comma continues the FROM list with another table", text: "SELECT * FROM users, ord", want: posTable},
		{name: "a comma after an alias continues the FROM list", text: "SELECT * FROM users u, ord", want: posTable},
		{name: "a word right after a table could be its alias, so nothing is claimed", text: "SELECT * FROM users u", want: posAny},
		{name: "after WHERE an identifier is a column", text: "SELECT * FROM users WHERE ", want: posColumn},
		{name: "after ON an identifier is a column", text: "SELECT * FROM users u JOIN orders o ON ", want: posColumn},
		{name: "after ORDER BY an identifier is a column", text: "SELECT * FROM users ORDER BY na", want: posColumn},
		{name: "after GROUP BY an identifier is a column", text: "SELECT * FROM users GROUP BY ci", want: posColumn},
		{name: "after HAVING an identifier is a column", text: "SELECT city FROM users GROUP BY city HAVING ", want: posColumn},
		{name: "after AND an identifier is a column", text: "SELECT * FROM users WHERE id = 1 AND ", want: posColumn},
		{name: "after UPDATE an identifier is a table", text: "UPDATE ", want: posTable},
		{name: "after SET an identifier is a column", text: "UPDATE users SET ", want: posColumn},
		{name: "after INSERT INTO an identifier is a table", text: "INSERT INTO ", want: posTable},
		{name: "a FROM inside a subquery still opens a table position", text: "SELECT * FROM (SELECT * FROM ", want: posTable},
		{name: "a statement after a terminator is read on its own", text: "SELECT 1; SELECT * FROM ", want: posTable},
		{name: "a FROM in a finished statement does not leak into the next one", text: "SELECT * FROM users; SELECT ", want: posColumn},
		{name: "a FROM inside a string literal is text, not a clause", text: "SELECT 'FROM ' || na", want: posColumn},
		{name: "a FROM inside a line comment is not a clause", text: "SELECT id -- FROM users\nWHERE ", want: posColumn},
		{name: "a multi-line statement keeps the clause of its last line", text: "SELECT name\nFROM ", want: posTable},
		{name: "a fresh continuation line keeps the clause it continues", text: "SELECT name,\n", want: posColumn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := analyzeSQL(tt.text, dialect.SQLite).position; got != tt.want {
				t.Errorf("analyzeSQL(%q).position = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestAnalyzeSQLTableRefs covers the tables a statement names and the aliases
// it gives them, which is what a qualified name is resolved against.
func TestAnalyzeSQLTableRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []tableRef
	}{
		{
			name: "a bare FROM names one table with no alias",
			text: "SELECT * FROM users WHERE ",
			want: []tableRef{{name: "users"}},
		},
		{
			name: "an implicit alias is attached to its table",
			text: "SELECT * FROM users u WHERE ",
			want: []tableRef{{name: "users", alias: "u"}},
		},
		{
			name: "an AS alias is attached to its table",
			text: "SELECT * FROM users AS u WHERE ",
			want: []tableRef{{name: "users", alias: "u"}},
		},
		{
			name: "a join names both tables with their aliases",
			text: "SELECT * FROM users u JOIN orders o ON ",
			want: []tableRef{{name: "users", alias: "u"}, {name: "orders", alias: "o"}},
		},
		{
			name: "a LEFT OUTER JOIN is not read as a table named LEFT",
			text: "SELECT * FROM users u LEFT OUTER JOIN orders o ON ",
			want: []tableRef{{name: "users", alias: "u"}, {name: "orders", alias: "o"}},
		},
		{
			name: "a comma-separated FROM list names every table",
			text: "SELECT * FROM users u, orders o WHERE ",
			want: []tableRef{{name: "users", alias: "u"}, {name: "orders", alias: "o"}},
		},
		{
			name: "a schema-qualified table keeps the table part, not the schema",
			text: "SELECT * FROM main.users WHERE ",
			want: []tableRef{{name: "users"}},
		},
		{
			name: "UPDATE names the table it writes to",
			text: "UPDATE users SET ",
			want: []tableRef{{name: "users"}},
		},
		{
			name: "an outer and an inner FROM are both in scope",
			text: "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE ",
			want: []tableRef{{name: "users"}, {name: "orders"}},
		},
		{
			name: "a table named in a comment is not in scope",
			text: "SELECT * /* FROM orders */ FROM users WHERE ",
			want: []tableRef{{name: "users"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := analyzeSQL(tt.text, dialect.SQLite).refs
			if !slices.Equal(got, tt.want) {
				t.Errorf("analyzeSQL(%q).refs = %+v, want %+v", tt.text, got, tt.want)
			}
		})
	}
}

// TestAnalyzeSQLQualifiedName covers splitting the word at the cursor into the
// qualifier that names a table and the column being typed after it.
func TestAnalyzeSQLQualifiedName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		text          string
		wantQualifier string
		wantPartial   string
	}{
		{name: "a bare word is all partial", text: "SELECT na", wantQualifier: "", wantPartial: "na"},
		{name: "a trailing dot is a qualifier with nothing typed after it", text: "SELECT u.", wantQualifier: "u", wantPartial: ""},
		{name: "a qualified name splits at its dot", text: "SELECT u.na", wantQualifier: "u", wantPartial: "na"},
		{name: "a qualifier is taken from the word at the cursor, not an earlier one", text: "SELECT a.id, b.na", wantQualifier: "b", wantPartial: "na"},
		{name: "a relative path is not a qualified name", text: ".import ./data.cs", wantQualifier: "", wantPartial: "./data.cs"},
		{name: "a number is not a qualified name", text: "SELECT 1.5", wantQualifier: "", wantPartial: "1.5"},
		{name: "a dot command is not a qualified name", text: ".mod", wantQualifier: "", wantPartial: ".mod"},
		{name: "an empty word after a space has no qualifier", text: "SELECT ", wantQualifier: "", wantPartial: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := analyzeSQL(tt.text, dialect.SQLite)
			if got.qualifier != tt.wantQualifier || got.partial != tt.wantPartial {
				t.Errorf("analyzeSQL(%q) qualifier/partial = %q/%q, want %q/%q",
					tt.text, got.qualifier, got.partial, tt.wantQualifier, tt.wantPartial)
			}
		})
	}
}

// TestAnalysisTableOf covers resolving the qualifier at the cursor to a table:
// an alias the statement declared, or a table the session holds.
func TestAnalysisTableOf(t *testing.T) {
	t.Parallel()

	known := []string{"users", "orders"}

	tests := []struct {
		name      string
		text      string
		wantTable string
		wantOK    bool
	}{
		{name: "an alias resolves to the table it names", text: "SELECT * FROM users u WHERE u.", wantTable: "users", wantOK: true},
		{name: "a table name resolves to itself", text: "SELECT * FROM users WHERE users.", wantTable: "users", wantOK: true},
		{name: "resolution is case-insensitive", text: "SELECT * FROM users u WHERE U.", wantTable: "users", wantOK: true},
		{name: "an unknown qualifier resolves to nothing", text: "SELECT * FROM users u WHERE x.", wantTable: "", wantOK: false},
		{name: "an alias shadows a table of the same name", text: "SELECT * FROM orders users WHERE users.", wantTable: "orders", wantOK: true},
		{name: "a table not in the statement still resolves if the session holds it", text: "SELECT * FROM users WHERE orders.", wantTable: "orders", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := analyzeSQL(tt.text, dialect.SQLite)
			gotTable, gotOK := a.tableOf(a.qualifier, known)
			if gotTable != tt.wantTable || gotOK != tt.wantOK {
				t.Errorf("tableOf(%q) = %q/%v, want %q/%v", a.qualifier, gotTable, gotOK, tt.wantTable, tt.wantOK)
			}
		})
	}
}

// TestAnalyzeSQLReadsDialectSpecificComments checks that the walk asks the
// dialect what a comment is. "#" opens a line comment in MySQL, so a FROM
// behind one is not a clause there, while in PostgreSQL the same text is code.
func TestAnalyzeSQLReadsDialectSpecificComments(t *testing.T) {
	t.Parallel()

	const text = "SELECT id # FROM users\nWHERE "

	if got := analyzeSQL(text, dialect.MySQL).refs; len(got) != 0 {
		t.Errorf("MySQL: refs = %+v, want none (the FROM is inside a # comment)", got)
	}
	if got := analyzeSQL(text, dialect.PostgreSQL).refs; !slices.Equal(got, []tableRef{{name: "users"}}) {
		t.Errorf("PostgreSQL: refs = %+v, want users (# does not open a comment)", got)
	}
}

// TestAnalyzeSQLNeverPanics is a fuzz-style sweep over malformed and partial
// input. Completion runs on every keystroke, so the analysis sees every prefix
// of everything a user types, including text that is not valid SQL at any
// point. Nothing it is given may take the shell down.
func TestAnalyzeSQLNeverPanics(t *testing.T) {
	t.Parallel()

	seeds := []string{
		"", " ", ".", "..", "...", "'", "\"", "`", "[", "/*", "--", "#",
		"SELECT", "SELECT *", "SELECT * FROM", "FROM FROM FROM", "JOIN",
		"UPDATE SET SET", "INSERT INTO INTO", "SELECT a.b.c.d FROM x",
		"SELECT * FROM 'users", "SELECT * FROM \"users", "SELECT * FROM [users",
		"SELECT * FROM users AS", "SELECT * FROM users,", "SELECT * FROM ,users",
		"WITH c AS (SELECT 1) SELECT * FROM c WHERE ", "SELECT 日本語.列 FROM 表",
		";;;", "; SELECT ", "SELECT 1;;", "SELECT\n\n\nFROM\n\n",
	}
	dialects := []dialect.Dialect{dialect.SQLite, dialect.MySQL, dialect.PostgreSQL, dialect.GoogleSQL}

	for _, seed := range seeds {
		// Every prefix is a state the completer really sees, one keystroke at a
		// time, so each is exercised rather than only the finished string.
		runes := []rune(seed)
		for i := range len(runes) + 1 {
			prefix := string(runes[:i])
			for _, d := range dialects {
				a := analyzeSQL(prefix, d)
				if a.qualifier != "" && !strings.Contains(prefix, a.qualifier) {
					t.Errorf("analyzeSQL(%q, %v): qualifier %q is not part of the input", prefix, d, a.qualifier)
				}
			}
		}
	}
}
