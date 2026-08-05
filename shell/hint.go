package shell

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// The two failures a caller hits most are "no such table" and "no such column",
// and they were the only ones sqly answered with nothing but SQLite's own text.
//
// That is backwards for the table one in particular. sqly's whole premise is
// that a file is a table, but the name is derived rather than given: a hyphen
// becomes an underscore, a leading digit gains a prefix, punctuation is dropped,
// and a workbook sheet becomes file_sheet. So "no such table: staf" leaves the
// question that actually matters — what is it called, then — unanswered, while
// sqly already goes out of its way to warn about a name that collides with a SQL
// keyword and to name both sources when two files claim one table.
//
// The hint is appended to the error rather than printed, so it arrives once, on
// stderr, next to the message it explains, whatever ran the statement: a --sql
// run, a line of a script (where the line number stays on the error), or the
// interactive prompt. Nothing about the exit code changes: the statement ran and
// failed, which is still a 1.

// tableNameRulesURL is where the derivation is written down. A hint that says a
// name is wrong without saying what decides names sends the reader looking.
const tableNameRulesURL = "https://nao1215.github.io/sqly/reference/#table-name-rules"

// maxHintedTables caps how many names a hint lists. A session can hold a table
// per sheet of a large workbook, and a hundred names on one line is not a hint;
// the count that follows says how much was left out.
const maxHintedTables = 20

// withMissingNameHint appends a hint to a statement error that failed on a name
// SQLite could not find. Any other error is returned as it came.
//
// The match is on SQLite's wording, which is safe here in a way it would not be
// across a boundary: this is sqly reading the engine it embeds, at a fixed
// version, and a wording change makes the hint disappear rather than corrupt
// anything.
func (s *Shell) withMissingNameHint(ctx context.Context, err error) error {
	if name, ok := missingName(err.Error(), "no such table: "); ok {
		hint, ok := s.missingTableHint(ctx, name)
		if !ok {
			return err
		}
		return fmt.Errorf("%w\n%s", err, hint)
	}
	if name, ok := missingName(err.Error(), "no such column: "); ok {
		hint := fmt.Sprintf("hint: no column %q. Run .describe TABLE in the shell, or sqly --inspect FILE, to list columns.", name)
		return fmt.Errorf("%w\n%s", err, hint)
	}
	return err
}

// missingName extracts the identifier SQLite reported after marker. The message
// continues with the statement that failed ("no such table: staf (1): SELECT
// ..."), so the name ends at the first space.
func missingName(msg, marker string) (string, bool) {
	i := strings.Index(msg, marker)
	if i < 0 {
		return "", false
	}
	rest := msg[i+len(marker):]
	if end := strings.IndexAny(rest, " \t\n"); end >= 0 {
		rest = rest[:end]
	}
	if rest == "" {
		return "", false
	}
	return rest, true
}

// missingTableHint builds the hint for a table this session does not have, and
// reports whether there is one to give. There is not when the table list cannot
// be read: a session that holds tables and one whose list failed would otherwise
// both be described as empty, and telling someone to pass a file they already
// passed is worse than saying nothing.
func (s *Shell) missingTableHint(ctx context.Context, missing string) (string, bool) {
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return "", false
	}
	if len(tables) == 0 {
		return "hint: this session has no tables. Pass a file, directory, or URL as an argument, or use .import inside the shell.", true
	}

	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name())
	}
	// Sorted, so the same session lists them the same way every time.
	slices.Sort(names)
	listed := names
	if len(names) > maxHintedTables {
		listed = append(names[:maxHintedTables:maxHintedTables], fmt.Sprintf("... (%d total)", len(names)))
	}
	return fmt.Sprintf("hint: this session has no table %q. Available tables: %s. sqly derives table names from file names: %s",
		missing, strings.Join(listed, ", "), tableNameRulesURL), true
}
