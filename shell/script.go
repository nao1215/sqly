package shell

import (
	"fmt"
	"strings"

	"github.com/nao1215/sqly/domain/sqltext"

	"github.com/nao1215/filesql/dialect"
)

// A script is what sqly reads when it is not being typed at: a `--sql-file`, a
// piped batch, or the `--sql` one-liner. It holds two kinds of thing, and every
// question sqly asks about a script — will it write files back? does it import
// its own input? how many statements is this? — is a question about those.
//
// They are parsed once, here, into the list below. Before this existed, each
// question had its own walk over the text: one scanned for ".save", another for
// ".import", a third counted statements, and the batch reader parsed it again on
// the way to running it. Four walks meant four chances to disagree, and they did:
// a dot-command written after any SQL statement was invisible to three of them,
// because they tracked the statement boundary with a buffer they never drained.
// Parsing once and executing the result is what keeps the answer to "does this
// script save?" the same as what running it actually does.

// scriptElementKind distinguishes the two things a script line can be.
type scriptElementKind int

const (
	// elementSQL is one complete SQL statement, which may span lines.
	elementSQL scriptElementKind = iota
	// elementDotCommand is one helper command, which is always a single line.
	elementDotCommand
)

// scriptElement is one executable unit of a script, with the source lines it
// came from so a failure can name them.
type scriptElement struct {
	kind scriptElementKind
	// text is the SQL statement, or the dot-command line with surrounding
	// whitespace removed.
	text string
	// startLine and endLine are 1-based and inclusive. A dot-command and a
	// single-line statement have startLine == endLine.
	startLine int
	endLine   int
}

// isDotCommand reports whether this element is a helper command.
func (e scriptElement) isDotCommand() bool { return e.kind == elementDotCommand }

// commandName returns a dot-command's name (".save", ".import"), or "" for a SQL
// statement.
func (e scriptElement) commandName() string {
	if !e.isDotCommand() {
		return ""
	}
	if fields := strings.Fields(e.text); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// dialectSelectedBy reports the dialect a ".dialect NAME" line selects. It
// answers false for every other line, and for a .dialect the command itself
// would reject — a bare one, which only prints the setting, one with too many
// arguments, and an unknown name — so a line that changes nothing when it runs
// changes nothing about how the script is read either.
func dialectSelectedBy(line string) (dialect.Dialect, bool) {
	argv, err := splitArgs(line)
	if err != nil || len(argv) != 2 || argv[0] != dialectCommand {
		return dialect.SQLite, false
	}
	d, err := dialect.Parse(argv[1])
	if err != nil {
		return dialect.SQLite, false
	}
	return d, true
}

// parseScript splits a script into the statements and helper commands it runs.
//
// A line is a helper command when it begins with "." and no SQL statement is
// open. "Open" means the buffered text holds executable SQL or an unterminated
// block comment; whitespace and closed leading comments are not open, so SQL and
// helper commands can alternate freely. That rule is what keeps ".save" inside a
// string literal, inside a line comment, and inside a block comment from being
// mistaken for the command — the text is still part of an open statement there.
//
// There is no limit on how long a line may be. One used to reject a line over a
// megabyte as "not a SQL script", which protected nothing — the whole script is
// already a string by the time it arrives here — while breaking the scripts that
// really do have one enormous line: a dump's multi-row INSERT, a base64 literal,
// a minified query.
//
// Leading whitespace before a helper command is allowed, so a script can indent
// its commands. A helper command sharing a line with SQL is rejected: reading
// "SELECT 1; .save" as two things depends on knowing where the statement ended,
// which is exactly what the reader cannot show the writer.
//
// d is the dialect the script opens in, not the one it keeps: a ".dialect" the
// script runs changes how the lines after it are read. Where a statement ends
// is a question only the dialect answers — "#" opens a comment in MySQL and
// GoogleSQL, PostgreSQL alone writes a dollar-quoted string — and the whole
// script used to be split by the dialect the process started with, so a
// statement written for the dialect the script had just selected was cut where
// that dialect has no boundary, or run together where it has one.
func parseScript(script string, d dialect.Dialect) ([]scriptElement, error) {
	script = strings.TrimPrefix(script, string(utf8BOM))

	var (
		elements     []scriptElement
		pending      strings.Builder
		pendingStart int
	)
	for lineIndex, line := range strings.Split(script, "\n") {
		lineNo := lineIndex + 1
		if atStatementBoundary(pending.String(), d) {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, ".") {
				// Buffered blank lines and leading comments belong to nothing now, so
				// they are dropped rather than merged into a later statement.
				pending.Reset()
				elements = append(elements, scriptElement{
					kind: elementDotCommand, text: trimmed, startLine: lineNo, endLine: lineNo,
				})
				// The one helper command that changes how the rest of the script is
				// read. It is applied here, as the line is parsed, rather than when
				// the script runs: by then the split has already happened.
				if selected, ok := dialectSelectedBy(trimmed); ok {
					d = selected
				}
				continue
			}
		}

		if pending.Len() == 0 {
			pendingStart = lineNo
		}
		pending.WriteString(line)
		pending.WriteString("\n")

		buf := pending.String()
		stmts, remainder := splitSQLStatements(buf, d)
		if len(stmts) == 0 {
			continue
		}

		// Attribute each statement to its own lines: stmtBuf shrinks past each one,
		// so two identical statements in one flush do not both report the first.
		stmtBuf, stmtStart := buf, pendingStart
		for _, stmt := range stmts {
			start, end := statementLineSpan(stmtBuf, stmtStart, stmt)
			elements = append(elements, scriptElement{
				kind: elementSQL, text: stmt, startLine: start, endLine: end,
			})
			if idx := strings.Index(stmtBuf, stmt); idx >= 0 {
				consumed := idx + len(stmt)
				stmtStart += strings.Count(stmtBuf[:consumed], "\n")
				stmtBuf = stmtBuf[consumed:]
			}
		}

		consumedPrefix := buf[:len(buf)-len(remainder)]
		pendingStart += strings.Count(consumedPrefix, "\n")

		// What is left on this line after a statement closed. A helper command there
		// would have to be told apart from SQL by position alone, so it is rejected.
		if rest, _, _ := strings.Cut(remainder, "\n"); strings.HasPrefix(strings.TrimSpace(rest), ".") {
			return nil, fmt.Errorf(
				"line %d puts %q on the same line as a SQL statement; a helper command must start its own line",
				lineNo, strings.TrimSpace(rest))
		}

		pending.Reset()
		pending.WriteString(remainder)
	}

	// A trailing statement with no ";" still runs; trailing comments alone do not.
	if leftover := sqltext.StripNoise(pending.String(), d); leftover != "" {
		start, end := statementLineSpan(pending.String(), pendingStart, leftover)
		elements = append(elements, scriptElement{
			kind: elementSQL, text: leftover, startLine: start, endLine: end,
		})
	}
	return elements, nil
}

// sqlStatements returns the SQL statements of a parsed script, dropping the
// helper commands. Classification (does this script modify data? can its
// statements be written back?) asks about SQL only.
func sqlStatements(elements []scriptElement) []string {
	stmts := make([]string, 0, len(elements))
	for _, e := range elements {
		if !e.isDotCommand() {
			stmts = append(stmts, e.text)
		}
	}
	return stmts
}

// runsHelper reports whether a parsed script issues the named helper command.
func runsHelper(elements []scriptElement, command string) bool {
	for _, e := range elements {
		if e.commandName() == command {
			return true
		}
	}
	return false
}

// firstHelper returns the first helper command in a parsed script, and whether
// there is one. It backs the `--sql-file` rejection, which names the line.
func firstHelper(elements []scriptElement) (scriptElement, bool) {
	for _, e := range elements {
		if e.isDotCommand() {
			return e, true
		}
	}
	return scriptElement{}, false
}
