package sqltext

import (
	"slices"
	"testing"
)

// TestMainVerb covers what the scanner has to get right to answer "which verb do
// these CTEs feed": parenthesis depth, every quoting style SQLite accepts, and
// both comment forms. The cases come from the four copies of this walk that used
// to live in the interactor and the shell, so nothing they each proved is lost by
// there now being one.
func TestMainVerb(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stmt string
		want string
	}{
		{"plain select", "SELECT * FROM t", "SELECT"},
		{"plain insert", "INSERT INTO t VALUES (1)", "INSERT"},
		{"plain update", "UPDATE t SET a = 1", "UPDATE"},
		{"plain delete", "DELETE FROM t", "DELETE"},
		{"plain replace", "REPLACE INTO t VALUES (1)", "REPLACE"},
		{"plain values", "VALUES (1),(2)", "VALUES"},
		{"lowercase select is upper-cased", "select * from t", "SELECT"},

		{"cte feeding select", "WITH c AS (SELECT 1) SELECT * FROM c", "SELECT"},
		{"cte feeding update", "WITH c AS (SELECT 1) UPDATE t SET a = (SELECT 1 FROM c)", "UPDATE"},
		{"cte feeding delete", "WITH c AS (SELECT id FROM t) DELETE FROM t WHERE id IN (SELECT id FROM c)", "DELETE"},
		{"cte feeding insert", "WITH c AS (SELECT 1 AS x) INSERT INTO t SELECT x FROM c", "INSERT"},
		{"cte feeding replace", "WITH c AS (SELECT 1) REPLACE INTO t VALUES(1)", "REPLACE"},
		{"cte feeding values", "WITH c AS (SELECT 1) VALUES (1)", "VALUES"},
		{"cte body select ignored at depth > 0", "WITH s AS (SELECT 1) UPDATE t SET x=1", "UPDATE"},
		{"nested parens skipped", "WITH c AS (SELECT (SELECT 1)) SELECT * FROM c", "SELECT"},
		{"doubly nested cte body ignored", "WITH s AS (SELECT 1 FROM (SELECT 2)) UPDATE t SET x=1", "UPDATE"},

		{"verb inside a single-quoted literal ignored", "SELECT 'INSERT' FROM t", "SELECT"},
		{"verb inside a double-quoted identifier ignored", `SELECT "update" FROM t`, "SELECT"},
		{"verb inside a cte body literal ignored", "WITH s AS (SELECT 'UPDATE') SELECT * FROM s", "SELECT"},
		{"verb inside a block comment ignored", "/* UPDATE */ SELECT 1", "SELECT"},
		{"verb inside a line comment ignored", "-- UPDATE\nSELECT 1", "SELECT"},

		{"double-quoted cte name skipped", `WITH "my cte" AS (SELECT 1) SELECT * FROM "my cte"`, "SELECT"},
		{"backtick-quoted cte name skipped", "WITH `my cte` AS (SELECT 1) UPDATE t SET a = 1", "UPDATE"},
		{"bracket-quoted cte name skipped", "WITH [my cte] AS (SELECT 1) DELETE FROM t", "DELETE"},
		{"line comment before the verb skipped", "WITH c AS (SELECT 1) -- comment\n REPLACE INTO t VALUES (1)", "REPLACE"},
		{"block comment before the verb skipped", "WITH c AS (SELECT 1) /* c */ UPDATE t SET x=1", "UPDATE"},

		{"leading whitespace", "   \n  SELECT 1", "SELECT"},
		{"statement with no main verb", "PRAGMA table_info(t)", ""},
		{"cte with nothing after it", "WITH c AS (SELECT 1)", ""},
		{"verb sqly does not route on", "WITH c AS (SELECT 1) VACUUM", ""},
		{"empty statement", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MainVerb(tt.stmt); got != tt.want {
				t.Errorf("MainVerb(%q) = %q, want %q", tt.stmt, got, tt.want)
			}
		})
	}
}

// TestHasWord locks the two properties a keyword search must have and a
// strings.Contains cannot: it matches whole words only, and it sees code only.
// RETURNING is the keyword that made this necessary, since it turns a DML
// statement into one that returns rows.
func TestHasWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stmt string
		want bool
	}{
		{"real RETURNING clause", "INSERT INTO t(a) VALUES (1) RETURNING a", true},
		{"update with returning", "UPDATE t SET a = 1 RETURNING *", true},
		{"delete with returning", "DELETE FROM t WHERE a = 1 RETURNING id", true},
		{"lowercase", "insert into t(a) values (1) returning a", true},
		{"mixed case", "Insert Into t(a) Values (1) ReTurNiNg a", true},
		{"real clause after a comment", "UPDATE t SET a=1 -- note\n RETURNING a", true},

		{"no returning clause", "INSERT INTO t(a) VALUES (1)", false},
		{"inside a single-quoted literal", "INSERT INTO t(a) VALUES ('returning')", false},
		{"inside a double-quoted identifier", `INSERT INTO t("returning") VALUES (1)`, false},
		{"inside a backtick identifier", "INSERT INTO t(`returning`) VALUES (1)", false},
		{"inside a bracket identifier", "INSERT INTO t([returning]) VALUES (1)", false},
		{"inside a line comment", "INSERT INTO t(a) VALUES (1) -- returning a\n", false},
		{"inside a block comment", "INSERT INTO t(a) VALUES (1) /* returning a */", false},
		{"word boundary: returning_at is a different word", "INSERT INTO t(returning_at) VALUES (1)", false},

		{"empty statement", "", false},
		{"whitespace only", "   \n\t ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HasWord(tt.stmt, "RETURNING"); got != tt.want {
				t.Errorf("HasWord(%q, RETURNING) = %v, want %v", tt.stmt, got, tt.want)
			}
		})
	}
}

// TestEndsInsideBlockComment drives the scanner through every state, because a
// "/*" only opens a comment outside strings, quoted identifiers, and a line
// comment. The batch reader asks this to decide whether the next line can be a
// helper command or is still inside a comment.
func TestEndsInsideBlockComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"open block", "SELECT 1 /* open", true},
		{"closed block", "SELECT 1 /* closed */", false},
		{"opener in a single quote", "SELECT '/*'", false},
		{"opener in a double quote", `SELECT "/*"`, false},
		{"opener in a backtick", "SELECT `/*`", false},
		{"opener in a bracket", "SELECT [/*]", false},
		{"opener in a line comment", "-- /* not a block\n", false},
		{"line comment then open block", "-- note\n/* open", true},
		{"plain line comment", "SELECT 1 -- trailing", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EndsInsideBlockComment(tt.in); got != tt.want {
				t.Errorf("EndsInsideBlockComment(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestStripNoise covers what can sit in front of the first executable character.
// A statement is classified by its first keyword, so anything left here is read
// as that keyword.
func TestStripNoise(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"block comment with no closing marker returns empty", "/* this comment never ends", ""},
		{"whitespace then unterminated block comment returns empty", "   \n\t/* still open", ""},
		{"line comment running to end of input returns empty", "-- only a line comment", ""},
		{"block comment then statement keeps the statement", "/* header */ SELECT 1", "SELECT 1"},
		{"line comment then statement keeps the statement", "-- header\nSELECT 1", "SELECT 1"},
		{"leading empty statement is dropped", ";SELECT 1", "SELECT 1"},
		{"several empty statements are dropped", " ; ; SELECT 1", "SELECT 1"},
		{"BOM is dropped", "\ufeffSELECT 1", "SELECT 1"},
		{"nothing to strip", "SELECT 1", "SELECT 1"},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StripNoise(tt.in); got != tt.want {
				t.Errorf("StripNoise(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestLeadingKeyword checks that the keyword stops at the first byte that cannot
// be part of one, so a statement needs no space after its verb.
func TestLeadingKeyword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain keyword", "SELECT 1", "SELECT"},
		{"lowercase is upper-cased", "select 1", "SELECT"},
		{"stops at an opening paren", "VALUES(1)", "VALUES"},
		{"stops at a paren after a keyword with an argument", "PRAGMA table_info(x)", "PRAGMA"},
		{"reads past a leading comment", "/* header */ UPDATE t SET a=1", "UPDATE"},
		{"reads past a leading empty statement", ";DELETE FROM t", "DELETE"},
		{"nothing executable", "-- just a comment", ""},
		{"empty input", "", ""},
		{"statement starting with a digit has no keyword", "1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := LeadingKeyword(tt.in); got != tt.want {
				t.Errorf("LeadingKeyword(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestTokensReportsOffsetsAndDepth checks the two things a caller slices with:
// the offsets have to bracket the token exactly, and Depth has to describe where
// the token sits, because splitSQLStatements cuts statements at Semicolon offsets
// and MainVerb decides on Depth.
func TestTokensReportsOffsetsAndDepth(t *testing.T) {
	t.Parallel()

	const stmt = "WITH c AS (SELECT 1) UPDATE t;"

	type got struct {
		kind  Kind
		text  string
		depth int
	}
	var tokens []got
	for tok := range Tokens(stmt) {
		tokens = append(tokens, got{tok.Kind, tok.Text(stmt), tok.Depth})
	}

	want := []got{
		{Word, "WITH", 0},
		{Word, "c", 0},
		{Word, "AS", 0},
		{Word, "SELECT", 1},
		{Word, "1", 1},
		{Word, "UPDATE", 0},
		{Word, "t", 0},
		{Semicolon, ";", 0},
	}
	if !slices.Equal(tokens, want) {
		t.Errorf("Tokens(%q) =\n  %v\nwant\n  %v", stmt, tokens, want)
	}
}

// TestTokensStopsWhenCallerBreaks confirms the iterator honors an early break,
// so a caller looking for one keyword does not pay for the rest of the statement.
func TestTokensStopsWhenCallerBreaks(t *testing.T) {
	t.Parallel()

	seen := 0
	for range Tokens("SELECT a, b, c, d, e FROM t") {
		seen++
		break
	}
	if seen != 1 {
		t.Errorf("iteration yielded %d tokens after break, want 1", seen)
	}
}

// TestScannerIgnoresMultibyteText is the reason byte-wise scanning is safe: UTF-8
// never puts an ASCII byte inside a multi-byte sequence, so a Japanese identifier
// or literal cannot fake a quote, a comment opener, or a semicolon.
func TestScannerIgnoresMultibyteText(t *testing.T) {
	t.Parallel()

	if got := MainVerb("WITH c AS (SELECT '日本語;--/*') UPDATE 表 SET a = 1"); got != "UPDATE" {
		t.Errorf("MainVerb over multibyte text = %q, want UPDATE", got)
	}
	if HasWord("SELECT '返却RETURNING'", "RETURNING") {
		t.Error("RETURNING inside a multibyte literal was matched")
	}
	if EndsInsideBlockComment("SELECT '/*日本語*/'") {
		t.Error("a comment opener inside a multibyte literal opened a comment")
	}
}
