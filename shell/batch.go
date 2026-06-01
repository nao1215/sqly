package shell

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nao1215/sqly/config"
)

// maxBatchLineBytes caps a single batch input line, preventing unbounded memory
// growth on input without newlines.
const maxBatchLineBytes = 10 * 1024 * 1024

// SQL keyword tokens used by statement classification, named once to avoid
// repeating the literals across the quote-aware scanners.
const (
	kwSelect  = "SELECT"
	kwInsert  = "INSERT"
	kwUpdate  = "UPDATE"
	kwDelete  = "DELETE"
	kwReplace = "REPLACE"
	kwValues  = "VALUES"
)

// utf8BOM is the UTF-8 byte order mark stripped from the start of batch input
// and --sql-file scripts so BOM-prefixed files parse like plain UTF-8.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// runBatchReader executes SQL statements and helper commands read from r. It is
// shared by batch stdin mode and --sql-file so both follow identical
// statement-splitting and error reporting; --sql-file passes a file reader
// instead of stdin, which frees stdin to carry a piped --stdin dataset.
//
// Input is parsed into statements, not raw lines, so SQL can span multiple
// lines (e.g. a formatted CTE). A SQL statement ends at a top-level ";"; helper
// commands (lines beginning with ".") are single-line. A trailing statement
// without ";" at EOF still runs.
//
// Execution is fail-fast: the first failed statement or helper command stops
// the run and returns an error, so later statements never execute and their
// output cannot leak into a pipeline that the process then reports as failed
// (). A ".exit" command stops early with success, mirroring the
// interactive shell. ranAny reports whether at least one statement or command
// was executed, so callers can skip post-run side effects (e.g. --save
// write-back) for an empty batch ().
func (s *Shell) runBatchReader(ctx context.Context, r io.Reader) (ranAny bool, err error) {
	// Strip a leading UTF-8 BOM so a BOM-prefixed batch stream (common from
	// Windows editors and export tools) parses the same as plain UTF-8.
	br := bufio.NewReader(r)
	if prefix, perr := br.Peek(len(utf8BOM)); perr == nil && bytes.Equal(prefix, utf8BOM) {
		_, _ = br.Discard(len(utf8BOM))
	}
	scanner := bufio.NewScanner(br)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxBatchLineBytes)

	stmtNo := 0
	exited := false
	var failErr error

	// run executes one statement/command and returns whether to stop the batch
	// (on failure or ".exit"). The first failure records failErr for the caller.
	run := func(stmt string) (stop bool) {
		stmtNo++
		ranAny = true
		if err := s.exec(ctx, stmt); err != nil {
			if errors.Is(err, ErrExitSqly) {
				exited = true
				return true
			}
			fmt.Fprintf(config.Stderr, "batch statement %d failed: %q: %v\n", stmtNo, stmt, err)
			failErr = errors.New("batch stopped: statement failed")
			return true
		}
		return false
	}

	var pending strings.Builder
scan:
	for scanner.Scan() {
		line := scanner.Text()
		// A dot-command is a complete single-line statement when no SQL statement
		// is open. "Open" means the pending buffer holds executable SQL, an
		// unterminated block comment, not just whitespace, newlines left after a
		// terminated statement, or a standalone (closed) leading comment. Checking
		// the boundary (rather than pending.Len() == 0) lets helper commands and SQL
		// alternate line-by-line after a ";" or a leading comment, while a dot-line
		// inside an open block comment stays part of the comment.
		if atStatementBoundary(pending.String()) {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, ".") {
				// Abandon any buffered leading comments/blank lines before running the
				// command, so they do not merge into a later statement.
				pending.Reset()
				if run(trimmed) {
					break scan
				}
				continue
			}
		}

		pending.WriteString(line)
		pending.WriteString("\n")
		stmts, remainder := splitSQLStatements(pending.String())
		pending.Reset()
		pending.WriteString(remainder)
		for _, stmt := range stmts {
			if run(stmt) {
				break scan
			}
		}
	}

	// On ".exit" or a failure, stop reading. Otherwise run any trailing
	// statement that was not terminated by ";".
	if !exited && failErr == nil {
		if err := scanner.Err(); err != nil {
			return ranAny, fmt.Errorf("failed to read batch input: %w", err)
		}
		if leftover := stripLeadingSQLComments(pending.String()); leftover != "" {
			run(leftover)
		}
	}

	return ranAny, failErr
}

// stripLeadingSQLComments removes leading line ("--") and block ("/* */")
// comments, leading empty statements (bare ";"), and surrounding whitespace from a
// statement, returning "" when nothing executable remains. sqly classifies a
// statement by its first token, so a leading comment would otherwise be rejected as
// "not sql"; SQL files commonly open with a header comment, so this lets them run
// unchanged. A leading ";" is an empty statement, so dropping it keeps a statement
// like ";UPDATE t ..." classified by its real verb instead of as a no-rowset
// statement that skips write-back.
func stripLeadingSQLComments(s string) string {
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

// splitSQLStatements splits accumulated text into complete statements terminated
// by a top-level ";" and returns the trailing unterminated remainder. Semicolons
// inside string literals, identifiers, and comments are ignored so they do not
// split a statement mid-value. Each returned statement has leading comments
// stripped so it is classified by its first SQL keyword.
func splitSQLStatements(s string) (stmts []string, remainder string) {
	runes := []rune(s)
	var (
		start                         int
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
	// A CREATE TRIGGER ... BEGIN ... END statement contains inner ";" that must not
	// split it. trig tracks whether the current statement is a CREATE TRIGGER and
	// how deep its BEGIN/CASE ... END nesting is, so a ";" only terminates once the
	// body's END has balanced its BEGIN. It resets after each split.
	var trig triggerState
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
			// SQLite backtick-quoted identifier; a doubled backtick escapes one,
			// which this toggle handles since it re-enters on the next backtick.
			if c == '`' {
				inBacktick = false
			}
		case inBracket:
			// SQLite bracket-quoted identifier; "]" closes it (brackets do not nest).
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
				// Consume a whole identifier/keyword token and feed it to the trigger
				// tracker, so BEGIN/CASE/END inside a CREATE TRIGGER are balanced.
				j := i
				for j+1 < len(runes) && isWordRune(runes[j+1]) {
					j++
				}
				trig.observe(strings.ToUpper(string(runes[i : j+1])))
				i = j
			case c == ';':
				if trig.insideBody() {
					break // ";" inside a trigger body does not terminate the statement
				}
				if stmt := stripLeadingSQLComments(string(runes[start:i])); stmt != "" {
					stmts = append(stmts, stmt)
				}
				start = i + 1
				trig = triggerState{}
			}
		}
	}
	return stmts, string(runes[start:])
}

// triggerState tracks whether the current statement is a CREATE TRIGGER and the
// depth of its BEGIN/CASE ... END nesting, so splitSQLStatements does not split a
// trigger body at its inner semicolons.
type triggerState struct {
	tokens    int  // significant word tokens observed from the statement start
	isTrigger bool // statement starts with CREATE [TEMP|TEMPORARY] TRIGGER
	bodyOpen  bool // the trigger's BEGIN has been seen
	depth     int  // open BEGIN/CASE blocks awaiting an END
}

// observe updates the state from the next upper-cased word token of the statement.
func (t *triggerState) observe(word string) {
	t.tokens++
	if !t.isTrigger {
		switch {
		case t.tokens == 1:
			if word != sqlCreate {
				t.tokens = notTriggerPrefix // first token is not CREATE: not a trigger
			}
		case word == "TEMP" || word == "TEMPORARY":
			// still within the "CREATE [TEMP] TRIGGER" prefix
		case word == "TRIGGER":
			t.isTrigger = true
		default:
			t.tokens = notTriggerPrefix // CREATE <something other than a trigger>
		}
		return
	}
	switch word {
	case "BEGIN":
		t.bodyOpen = true
		t.depth++
	case "CASE":
		if t.bodyOpen {
			t.depth++
		}
	case "END":
		if t.bodyOpen && t.depth > 0 {
			t.depth--
		}
	}
}

// insideBody reports whether a ";" currently sits inside an open trigger body and
// so must not terminate the statement: the statement is a trigger and either its
// BEGIN has not appeared yet or its blocks are not all closed.
func (t *triggerState) insideBody() bool {
	return t.isTrigger && (!t.bodyOpen || t.depth > 0)
}

// notTriggerPrefix is a sentinel token count marking a statement whose prefix has
// already ruled it out as a CREATE TRIGGER, so further tokens are ignored.
const notTriggerPrefix = 99

// sqlCreate is the CREATE keyword, matched when detecting a CREATE TRIGGER prefix.
const sqlCreate = "CREATE"

// atStatementBoundary reports whether the pending batch buffer holds no open
// statement: only whitespace and complete (closed) leading comments, with no
// unterminated block comment. At a boundary the next line may start a new
// statement or a helper command. An unterminated block comment is not a boundary,
// because following lines (including dot-lines) are still inside the comment.
func atStatementBoundary(pending string) bool {
	if strings.TrimSpace(stripLeadingSQLComments(pending)) != "" {
		return false
	}
	// stripLeadingSQLComments also strips to "" for an unterminated block comment,
	// so check that state explicitly to avoid treating an open comment as empty.
	return !endsInsideBlockComment(pending)
}

// endsInsideBlockComment reports whether s ends inside an unterminated "/* ... */"
// block comment, scanning quote- and comment-aware so a "/*" inside a string
// literal or line comment is not mistaken for a comment opener.
func endsInsideBlockComment(s string) bool {
	runes := []rune(s)
	var (
		inSingle, inDouble            bool
		inBacktick, inBracket         bool
		inLineComment, inBlockComment bool
	)
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
			}
		}
	}
	return inBlockComment
}

// scriptModifiesData reports whether any SQL statement in a batch script is a
// data-modifying statement (INSERT, UPDATE, DELETE, REPLACE, or a WITH that feeds
// one). Helper commands (lines beginning with "." at a statement boundary) are
// not SQL, so they are dropped before classification; otherwise a script like
// ".mode csv\nUPDATE t SET x=1;" would hide the UPDATE behind the dot-line.
// Classification is per statement so an EXPLAIN of a DML statement counts as
// read-only. It lets a non-interactive run skip write-back preflight for a
// read-only script. Whether write-back actually runs is
// decided dynamically by the rows a statement changes (see Shell.dataChanged).
func scriptModifiesData(script string) bool {
	for _, stmt := range scriptSQLStatements(script) {
		if statementModifiesData(stmt) {
			return true
		}
	}
	return false
}

// scriptSQLStatements extracts the SQL statements from a batch or --sql-file
// script, dropping helper command lines (a "." line at a statement boundary) so
// they are not misclassified as SQL. It is the shared front end for the write-back
// classifiers below.
func scriptSQLStatements(script string) []string {
	var sql strings.Builder
	for _, line := range strings.Split(script, "\n") {
		if atStatementBoundary(sql.String()) && strings.HasPrefix(strings.TrimSpace(line), ".") {
			continue // a helper command, not part of any SQL statement
		}
		sql.WriteString(line)
		sql.WriteString("\n")
	}
	stmts, remainder := splitSQLStatements(sql.String())
	if leftover := stripLeadingSQLComments(remainder); leftover != "" {
		stmts = append(stmts, leftover)
	}
	return stmts
}

// countSQLStatements returns the number of complete SQL statements in s, counting
// a trailing statement that is not terminated by ";". It is quote-, comment-, and
// CREATE TRIGGER-aware (via splitSQLStatements), so semicolons inside string
// literals, identifiers, comments, and trigger bodies do not inflate the count. It
// backs the direct --sql single-statement guard.
func countSQLStatements(s string) int {
	stmts, remainder := splitSQLStatements(s)
	count := len(stmts)
	if stripLeadingSQLComments(remainder) != "" {
		count++
	}
	return count
}

// statementSaveCompatible reports whether a non-interactive --save/--save-dir run
// can handle a statement: a read-only query (which skips write-back) or a
// row-modifying DML on an imported table (which write-back persists). Any other
// statement — DDL (CREATE/DROP/ALTER/REINDEX and CREATE VIEW/INDEX/TRIGGER),
// ANALYZE, or other schema/maintenance work — has no file write-back
// representation, so it is save-incompatible and the run must fail loudly instead
// of exiting 0 while leaving the source unchanged.
//
// PRAGMA is save-incompatible too: a setter (PRAGMA user_version=1), command
// (PRAGMA incremental_vacuum), or rowset (PRAGMA journal_mode=OFF) PRAGMA only
// changes the transient in-memory session, with no file representation, so a save
// run that includes one must fail rather than imply a durable effect.
func statementSaveCompatible(stmt string) bool {
	if statementModifiesData(stmt) {
		return true
	}
	switch leadingSQLKeyword(stmt) {
	case kwSelect, kwValues, "WITH", "EXPLAIN", "TABLE":
		return true
	default:
		return false
	}
}

// firstSaveIncompatibleStatement returns the first statement a non-interactive
// save run cannot persist, or "" when every statement is a read-only query or a
// row-modifying DML.
func firstSaveIncompatibleStatement(script string) string {
	for _, stmt := range scriptSQLStatements(script) {
		if !statementSaveCompatible(stmt) {
			return stmt
		}
	}
	return ""
}

// scriptImportsInput reports whether a batch or --sql-file script imports its own
// input with a ".import" helper command. Such a script creates its tables while it
// runs, so save preflight cannot list them up front and instead defers write-back
// validation until after the run.
func scriptImportsInput(script string) bool {
	var sql strings.Builder
	for _, line := range strings.Split(script, "\n") {
		if !atStatementBoundary(sql.String()) {
			sql.WriteString(line)
			sql.WriteString("\n")
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".") {
			if fields := strings.Fields(trimmed); len(fields) > 0 && fields[0] == importCommand {
				return true
			}
			continue // another helper command, not part of any SQL statement
		}
		sql.WriteString(line)
		sql.WriteString("\n")
	}
	return false
}

// statementModifiesData reports whether a single statement changes table data:
// an INSERT/UPDATE/DELETE/REPLACE, or a WITH whose main statement is one of those.
// An EXPLAIN of such a statement is read-only and reports false, so it never
// triggers write-back.
func statementModifiesData(stmt string) bool {
	switch leadingSQLKeyword(stmt) {
	case kwInsert, kwUpdate, kwDelete, kwReplace:
		return true
	case "WITH":
		switch withMainVerb(stmt) {
		case kwInsert, kwUpdate, kwDelete, kwReplace:
			return true
		}
	}
	return false
}

// statementResultMessage returns the stdout line for a no-rowset statement. A
// data-modifying statement (INSERT/UPDATE/DELETE/REPLACE, or a WITH feeding one)
// reports its affected-row count; any other no-rowset statement (DDL, PRAGMA,
// maintenance) reports neutral success, because an "affected is N row(s)" line for
// a CREATE VIEW, PRAGMA, or ANALYZE implies a row change that did not happen.
func statementResultMessage(stmt string, affected int64) string {
	if statementModifiesData(stmt) {
		return fmt.Sprintf("affected is %d row(s)\n", affected)
	}
	return msgStatementExecuted
}

// msgStatementExecuted is the neutral stdout line printed for a no-rowset
// statement that does not change table data (DDL, PRAGMA, maintenance).
const msgStatementExecuted = "statement executed successfully\n"

// leadingSQLKeyword returns the upper-cased first keyword of a statement after
// leading comments are stripped, reading only the leading ASCII letters.
func leadingSQLKeyword(stmt string) string {
	s := stripLeadingSQLComments(stmt)
	i := 0
	for i < len(s) && ((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z')) {
		i++
	}
	return strings.ToUpper(s[:i])
}

// withMainVerb returns the main statement verb of a WITH statement: the first
// INSERT/UPDATE/DELETE/REPLACE/SELECT/VALUES token at parenthesis depth 0 outside
// quotes and comments, so the CTE bodies (inside parentheses) are skipped. It
// lets write-back detection see that a WITH ... UPDATE modifies data while a
// WITH ... SELECT does not.
func withMainVerb(stmt string) string {
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
				case kwSelect, kwValues, kwInsert, kwUpdate, kwDelete, kwReplace:
					return strings.ToUpper(string(runes[start : i+1]))
				}
			}
		}
	}
	return ""
}

// readSQLFile reads the SQL script at path for --sql-file. It returns a clear
// error for a missing or unreadable file (wrapping the OS error so callers can
// inspect it with errors.Is) and rejects a file with no SQL, so an empty or
// whitespace-only script fails loudly instead of running nothing.
func readSQLFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the user-specified --sql-file
	if err != nil {
		return "", fmt.Errorf("failed to read --sql-file %q: %w", path, err)
	}
	// Strip a leading UTF-8 BOM so a BOM-prefixed script (common from Windows
	// editors and export tools) parses the same as plain UTF-8.
	content := strings.TrimPrefix(string(data), "\ufeff")
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("--sql-file %q is empty", path)
	}
	// A comment-only script has no executable SQL, which is the same failure as
	// an empty file: splitting yields no terminated statements and the remainder
	// strips down to nothing once leading comments are removed. Reject it instead
	// of silently running nothing.
	stmts, remainder := splitSQLStatements(content)
	if len(stmts) == 0 && stripLeadingSQLComments(remainder) == "" {
		return "", fmt.Errorf("--sql-file %q contains no executable SQL statements", path)
	}
	return content, nil
}
