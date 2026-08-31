package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/sqltext"

	"github.com/nao1215/filesql/dialect"
)

// SQL keyword tokens used by statement classification, named once to avoid
// repeating the literals across the quote-aware scanners.
const (
	kwSelect  = "SELECT"
	kwInsert  = "INSERT"
	kwUpdate  = "UPDATE"
	kwDelete  = "DELETE"
	kwReplace = "REPLACE"
	kwValues  = "VALUES"
	kwCase    = "CASE"
	kwAnalyze = "ANALYZE"
	kwReindex = "REINDEX"
)

// utf8BOM is the UTF-8 byte order mark stripped from the start of batch input
// and --sql-file scripts so BOM-prefixed files parse like plain UTF-8.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// runBatchReader executes SQL statements and helper commands read from r. It is
// shared by batch stdin mode and --sql-file so both follow identical
// statement-splitting and error reporting; --sql-file passes a file reader
// instead of stdin, which frees stdin to carry a piped --stdin-format dataset.
//
// Input is parsed into statements, not raw lines, so SQL can span multiple
// lines (e.g. a formatted CTE). A SQL statement ends at a top-level ";"; helper
// commands (lines beginning with ".") are single-line. A trailing statement
// without ";" at EOF still runs.
//
// Execution is fail-fast: the first failed statement or helper command stops
// the run and returns an error, so later statements never execute and their
// output cannot leak into a pipeline that the process then reports as failed.
// A ".exit" command stops early with success, mirroring the interactive shell.
// ranAny reports whether at least one statement or command was executed, so
// callers can skip post-run side effects (e.g. a .save write-back) for an empty
// batch.
func (s *Shell) runScriptElements(ctx context.Context, elements []scriptElement) (ranAny bool, err error) {
	var failErr error
	for i, element := range elements {
		ranAny = true
		if runErr := s.exec(ctx, element.text); runErr != nil {
			if errors.Is(runErr, ErrExitSqly) {
				break
			}
			// --sql is one statement, so there is no position to report and no run
			// to stop: "batch statement 1 failed at line 1" and "batch stopped"
			// describe a script the user did not write, and the statement text
			// already arrives once inside the error. The failure is returned bare,
			// and main prints it.
			if s.plan.mode == modeInlineSQL {
				failErr = runErr
				break
			}
			loc := fmt.Sprintf("line %d", element.startLine)
			if element.endLine > element.startLine {
				loc = fmt.Sprintf("lines %d-%d", element.startLine, element.endLine)
			}
			fmt.Fprintf(config.Stderr, "batch statement %d failed at %s: %q: %v\n",
				i+1, loc, previewStatement(element.text), runErr)
			failErr = &batchStopError{Err: runErr}
			break
		}
	}
	return ranAny, failErr
}

// maxStatementPreview caps the characters of a failing statement shown in the
// batch error, so a long statement does not flood stderr.
const maxStatementPreview = 200

// previewStatement renders a failing statement for an error message: whitespace
// runs are collapsed so a multiline statement stays on one line, and a statement
// longer than maxStatementPreview is truncated with a marker and its full length.
func previewStatement(stmt string) string {
	oneLine := strings.Join(strings.Fields(stmt), " ")
	runes := []rune(oneLine)
	if len(runes) <= maxStatementPreview {
		return oneLine
	}
	return fmt.Sprintf("%s… (%d chars total)", string(runes[:maxStatementPreview]), len(runes))
}

// statementLineSpan returns the 1-based start and end line of stmt within buf,
// where buf's first line is bufStartLine. Lines are counted from the newlines
// preceding the statement, so leading comment lines advance the start to the
// first SQL line. A statement not found in buf falls back to the buffer start.
func statementLineSpan(buf string, bufStartLine int, stmt string) (start, end int) {
	idx := strings.Index(buf, stmt)
	if idx < 0 {
		return bufStartLine, bufStartLine
	}
	start = bufStartLine + strings.Count(buf[:idx], "\n")
	end = start + strings.Count(stmt, "\n")
	return start, end
}

// splitSQLStatements splits accumulated text into complete statements terminated
// by a top-level ";" and returns the trailing unterminated remainder. Semicolons
// inside string literals, identifiers, and comments are ignored so they do not
// split a statement mid-value. Each returned statement has leading comments
// stripped so it is classified by its first SQL keyword.
func splitSQLStatements(s string, d dialect.Dialect) (stmts []string, remainder string) {
	// A CREATE TRIGGER ... BEGIN ... END statement contains inner ";" that must not
	// split it. trig tracks whether the current statement is a CREATE TRIGGER and
	// how deep its BEGIN/CASE ... END nesting is, so a ";" only terminates once the
	// body's END has balanced its BEGIN. It resets after each split.
	var (
		start int
		trig  triggerState
	)
	for tok := range sqltext.Tokens(s, d) {
		switch tok.Kind {
		case sqltext.Word:
			trig.observe(strings.ToUpper(tok.Text(s)))
		case sqltext.Semicolon:
			if trig.insideBody() {
				continue // ";" inside a trigger body does not terminate the statement
			}
			if stmt := sqltext.StripNoise(s[start:tok.Start], d); stmt != "" {
				stmts = append(stmts, stmt)
			}
			start = tok.End
			trig = triggerState{}
		}
	}
	return stmts, s[start:]
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
	case kwCase:
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
func atStatementBoundary(pending string, d dialect.Dialect) bool {
	if sqltext.StripNoise(pending, d) != "" {
		return false
	}
	// StripNoise also strips to "" for an unterminated block comment, so check
	// that state explicitly to avoid treating an open comment as empty.
	return !sqltext.EndsInsideBlockComment(pending, d)
}

// statementSaveCompatible reports whether a non-interactive write-back run
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
// The statements this refuses are named rather than inferred from what it does
// not recognize. A typo is not a schema change, and reporting one as ".save
// cannot persist ... it changes schema" sent the reader looking for a schema
// change that was never there while hiding the syntax error that actually
// stopped the run. An unrecognized statement is left to run and to fail with
// SQLite's own message.
var saveIncompatibleKeywords = map[string]bool{
	"CREATE": true, "DROP": true, "ALTER": true, "RENAME": true,
	kwReindex: true, kwAnalyze: true, "PRAGMA": true, "VACUUM": true,
	"ATTACH": true, "DETACH": true, "GRANT": true, "REVOKE": true,
}

func statementSaveCompatible(stmt string, d dialect.Dialect) bool {
	if statementModifiesData(stmt, d) {
		return true
	}
	// A TEMP table is scratch space that never reaches a file: it is dropped with
	// the session and write-back skips it like any other SQL-created table. A
	// script is allowed to build one and still save the tables it imported.
	if createsTempTable(stmt, d) {
		return true
	}
	return !saveIncompatibleKeywords[sqltext.LeadingKeyword(stmt, d)]
}

// createsTempTable reports whether a statement creates a temporary table or
// view: "CREATE TEMP ..." or "CREATE TEMPORARY ...", in any case.
func createsTempTable(stmt string, d dialect.Dialect) bool {
	fields := strings.Fields(strings.ToUpper(sqltext.StripNoise(stmt, d)))
	if len(fields) < 2 || fields[0] != sqlCreate {
		return false
	}
	return fields[1] == "TEMP" || fields[1] == "TEMPORARY"
}

// firstSaveIncompatibleStatement returns the first statement a non-interactive
// save run cannot persist, or "" when every statement a save could reach is a
// read-only query or a row-modifying DML.
//
// Only the statements before the last .save are looked at. A .save writes what
// the session has changed by the time it runs, so a statement after the final
// one cannot alter what was written: a script that saves and then builds a
// scratch table was refused outright, and the save it asked for never happened.
func firstSaveIncompatibleStatement(elements []scriptElement, d dialect.Dialect) string {
	last := -1
	for i, e := range elements {
		if e.commandName() == saveCommand {
			last = i
		}
	}
	if last < 0 {
		return ""
	}
	for _, stmt := range sqlStatements(elements[:last]) {
		if !statementSaveCompatible(stmt, d) {
			return stmt
		}
	}
	return ""
}

// statementModifiesData reports whether a single statement changes table data:
// an INSERT/UPDATE/DELETE/REPLACE, or a WITH whose main statement is one of those.
// An EXPLAIN of such a statement is read-only and reports false, so it never
// triggers write-back.
func statementModifiesData(stmt string, d dialect.Dialect) bool {
	switch sqltext.LeadingKeyword(stmt, d) {
	case kwInsert, kwUpdate, kwDelete, kwReplace:
		return true
	case "WITH":
		switch sqltext.MainVerb(stmt, d) {
		case kwInsert, kwUpdate, kwDelete, kwReplace:
			return true
		}
	}
	return false
}

// statementChangesSchema reports whether a statement can change what tables the
// session holds or what columns they have: the DDL verbs, plus ATTACH/DETACH,
// which add and remove whole sets of tables.
//
// It is what tells completion to drop its cached schema. The cache is keyed by
// the table-name set, so a CREATE or DROP rebuilds it on its own, but an
// "ALTER TABLE t ADD COLUMN c" leaves that set untouched and the cached columns
// would stay as they were before the statement ran.
func statementChangesSchema(stmt string, d dialect.Dialect) bool {
	switch sqltext.LeadingKeyword(stmt, d) {
	case "CREATE", "ALTER", "DROP", "ATTACH", "DETACH", "RENAME":
		return true
	}
	return false
}

// statementResultMessage returns the stdout line for a no-rowset statement. A
// data-modifying statement (INSERT/UPDATE/DELETE/REPLACE, or a WITH feeding one)
// reports its affected-row count; any other no-rowset statement (DDL, PRAGMA,
// maintenance) reports neutral success, because an "affected is N row(s)" line for
// a CREATE VIEW, PRAGMA, or ANALYZE implies a row change that did not happen.
func statementResultMessage(stmt string, affected int64, d dialect.Dialect) string {
	if statementModifiesData(stmt, d) {
		return fmt.Sprintf("affected is %d row(s)\n", affected)
	}
	return msgStatementExecuted
}

// msgStatementExecuted is the neutral stdout line printed for a no-rowset
// statement that does not change table data (DDL, PRAGMA, maintenance).
const msgStatementExecuted = "statement executed successfully\n"

// readScriptFile reads a --script-file. It is the sibling of readSQLFile and
// differs in what counts as empty: a script whose only content is dot-commands
// has no SQL statement and is perfectly valid, so the "no executable SQL" check
// does not apply here. A file with nothing in it at all is still rejected, for
// the same reason as a --sql-file: a run that does nothing and exits 0 is
// indistinguishable from one that worked.
func readScriptFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the user-specified --script-file
	if err != nil {
		return "", &scriptSourceError{Err: fmt.Errorf("failed to read --script-file %q: %w", path, err)}
	}
	content := strings.TrimPrefix(string(data), "\ufeff")
	if strings.TrimSpace(content) == "" {
		return "", &scriptError{Err: fmt.Errorf("--script-file %q is empty", path)}
	}
	return content, nil
}

// readSQLFile reads the SQL script at path for --sql-file. It returns a clear
// error for a missing or unreadable file (wrapping the OS error so callers can
// inspect it with errors.Is) and rejects a file with no SQL, so an empty or
// whitespace-only script fails loudly instead of running nothing.
func readSQLFile(path string, d dialect.Dialect) (string, error) {
	// The two kinds of failure below are told apart deliberately. A path that
	// cannot be read is a problem with the file, like any other input; a file that
	// was read and holds no statement is a problem with what the user wrote. They
	// exit with different codes, so they are different types here.
	data, err := os.ReadFile(path) //nolint:gosec // path is the user-specified --sql-file
	if err != nil {
		return "", &scriptSourceError{Err: fmt.Errorf("failed to read --sql-file %q: %w", path, err)}
	}
	// Strip a leading UTF-8 BOM so a BOM-prefixed script (common from Windows
	// editors and export tools) parses the same as plain UTF-8.
	content := strings.TrimPrefix(string(data), "\ufeff")
	if strings.TrimSpace(content) == "" {
		return "", &scriptError{Err: fmt.Errorf("--sql-file %q is empty", path)}
	}
	// A comment-only script has no executable SQL, which is the same failure as
	// an empty file: splitting yields no terminated statements and the remainder
	// strips down to nothing once leading comments are removed. Reject it instead
	// of silently running nothing.
	stmts, remainder := splitSQLStatements(content, d)
	if len(stmts) == 0 && sqltext.StripNoise(remainder, d) == "" {
		return "", &scriptError{Err: fmt.Errorf("--sql-file %q contains no executable SQL statements", path)}
	}
	return content, nil
}
