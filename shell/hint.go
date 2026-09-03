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
		return fmt.Errorf("%w\n%s", err, s.missingColumnHint(ctx, name))
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
	// The engine quotes a name it read as a quoted identifier -- "no such
	// column: \"namee\" - should this be a string literal in single-quotes?" --
	// and the hint quotes what it is given, so the quotes come off here rather
	// than reaching the reader doubled.
	rest = strings.Trim(rest, `"`)
	if rest == "" {
		return "", false
	}
	return rest, true
}

// asMissingTableError rewrites the engine's report of an unknown table into the
// wording every helper command uses, and leaves any other error alone.
//
// A helper command names a table directly, so the engine's own text says nothing
// the user can act on that the command cannot say better: "SQL logic error: no
// such table: nope (1)" offers an error number and an engine nobody addressed,
// for what is almost always a typo. .describe and .schema check the table
// themselves and answer plainly; the commands that reach the engine first now
// report the same thing.
func asMissingTableError(err error, tableName string) error {
	if err == nil {
		return nil
	}
	if _, ok := missingName(err.Error(), "no such table: "); !ok {
		return err
	}
	return fmt.Errorf("no such table: %s", tableName)
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
	sentences := []string{fmt.Sprintf("hint: this session has no table %q.", missing)}
	if guess := didYouMean(missing, names); guess != "" {
		sentences = append(sentences, guess)
	}
	sentences = append(sentences,
		fmt.Sprintf("Available tables: %s.", strings.Join(listed, ", ")),
		"sqly derives table names from file names: "+tableNameRulesURL)
	return strings.Join(sentences, " "), true
}

// missingColumnHint is what to say about a column the engine could not find.
//
// The advice on its own — run .describe — asks the reader to do what the shell
// can already do: it holds every column of every table it imported, so a name
// that is one typo from one of them can be named outright. The advice stays for
// the names that are not, which is when listing the columns is the only answer
// there is.
//
// The columns of every table are searched, in the order the metadata reports
// them, because the message SQLite gives says which column is missing and not
// which table was supposed to have it.
func (s *Shell) missingColumnHint(ctx context.Context, missing string) string {
	schema := s.schemaForCompletion(ctx)
	var columns []string
	for _, table := range schema.tables {
		columns = append(columns, schema.columns[table]...)
	}

	sentences := []string{fmt.Sprintf("hint: no column %q.", missing)}
	if guess := didYouMean(missing, columns); guess != "" {
		sentences = append(sentences, guess)
	}
	sentences = append(sentences, "Run .describe TABLE in the shell, or sqly --inspect FILE, to list columns.")
	return strings.Join(sentences, " ")
}

// didYouMean is the sentence naming what a mistyped name is a typo of, or "" when
// nothing is close enough. A guess at a name nothing resembles is worse than no
// guess: it sends the reader after a name that was never the one they wanted.
//
// It is a sentence of its own rather than a clause, so a message can carry it
// between what it already said and what it already advised, and so a message
// that has no guess to offer reads exactly as it did before.
func didYouMean(missing string, candidates []string) string {
	nearest, ok := nearestName(missing, candidates)
	if !ok {
		return ""
	}
	return fmt.Sprintf("Did you mean %q?", nearest)
}
