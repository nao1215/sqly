package shell

import (
	"strings"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/domain/sqltext"
)

// indentUnit is what one level of nesting is worth. Two spaces, because the
// terminal is narrow and a statement typed at a prompt is usually shallow.
const indentUnit = "  "

// sqlIndent returns what the line continuing before should open with.
//
// It keeps the indentation of the line being continued and adds or removes a
// level for each parenthesis that line opened or closed, so a continuation
// lands under the clause it belongs to and the body of a subquery or a long
// argument list steps in once:
//
//	sqly:~(table)$ SELECT * FROM users WHERE id IN (
//	   ...>   SELECT user_id FROM orders WHERE total > (
//	   ...>     SELECT avg(total) FROM orders
//
// Why the previous line's indentation is the base rather than the depth alone:
// indentation typed by hand is a decision, and a rule that recomputes every
// line from its depth throws it away on the next Enter. Only the change in
// depth is sqly's to add.
//
// The count comes from sqltext, so a parenthesis inside a string literal, a
// quoted identifier, or a comment is text and does not indent anything, and the
// dialect decides how those are spelled.
func sqlIndent(before string, d dialect.Dialect) string {
	// A helper command is one line and takes no continuation, so nothing here
	// applies to it.
	if looksLikeCommand(strings.TrimSpace(before)) && !strings.Contains(before, "\n") {
		return ""
	}

	line := before[strings.LastIndex(before, "\n")+1:]
	base := leadingBlanks(line)

	// The depth the line opened at, and the depth it leaves behind.
	opened := sqltext.OpenDepth(before[:len(before)-len(line)], d)
	closing := sqltext.OpenDepth(before, d)

	levels := closing - opened
	if levels > 0 {
		return base + strings.Repeat(indentUnit, levels)
	}
	// The line closed more than it opened, so the next one steps back out --
	// but only as far as the indentation it was given.
	return trimIndentLevels(base, -levels)
}

// leadingBlanks returns the spaces and tabs a line opens with.
func leadingBlanks(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// trimIndentLevels removes up to levels units of indentation from the end of
// base, stopping at whatever is left rather than going past the margin. A tab
// counts as a whole level, because a writer using tabs means one per level.
func trimIndentLevels(base string, levels int) string {
	for range levels {
		switch {
		case strings.HasSuffix(base, indentUnit):
			base = base[:len(base)-len(indentUnit)]
		case strings.HasSuffix(base, "\t"):
			base = base[:len(base)-1]
		default:
			return strings.TrimRight(base, " ")
		}
	}
	return base
}
