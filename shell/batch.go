package shell

import (
	"bufio"
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

// runBatch executes SQL statements and helper commands read from stdin until
// EOF. It is used when sqly runs without a TTY (piped stdin), where the
// interactive prompt cannot start.
func (s *Shell) runBatch(ctx context.Context) (ranAny bool, err error) {
	return s.runBatchReader(ctx, s.stdin)
}

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
// (Ref #308). A ".exit" command stops early with success, mirroring the
// interactive shell. ranAny reports whether at least one statement or command
// was executed, so callers can skip post-run side effects (e.g. --save
// write-back) for an empty batch (Ref #330).
func (s *Shell) runBatchReader(ctx context.Context, r io.Reader) (ranAny bool, err error) {
	scanner := bufio.NewScanner(r)
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
		// At a statement boundary, a dot-command is a complete single-line
		// statement. Inside an open SQL statement, the line is SQL.
		if pending.Len() == 0 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, ".") {
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
// comments and surrounding whitespace from a statement, returning "" when
// nothing executable remains. sqly classifies a statement by its first token, so
// a leading comment would otherwise be rejected as "not sql"; SQL files commonly
// open with a header comment, so this lets them run unchanged.
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
			case c == ';':
				if stmt := stripLeadingSQLComments(string(runes[start:i])); stmt != "" {
					stmts = append(stmts, stmt)
				}
				start = i + 1
			}
		}
	}
	return stmts, string(runes[start:])
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
	if strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("--sql-file %q is empty", path)
	}
	return string(data), nil
}
