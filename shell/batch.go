package shell

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nao1215/sqly/config"
)

// maxBatchLineBytes caps a single batch input line, preventing unbounded memory
// growth on input without newlines.
const maxBatchLineBytes = 10 * 1024 * 1024

// runBatch executes SQL statements and helper commands read from stdin until
// EOF. It is used when sqly runs without a TTY (piped stdin), where the
// interactive prompt cannot start.
//
// Input is parsed into statements, not raw lines, so SQL can span multiple
// lines (e.g. a formatted CTE). A SQL statement ends at a top-level ";"; helper
// commands (lines beginning with ".") are single-line. A trailing statement
// without ";" at EOF still runs, so one-shot queries keep working; incomplete
// SQL surfaces SQLite's error. Errors are reported to
// stderr with the statement index and do not stop processing; if any statement
// failed, runBatch returns an error so the process exits non-zero. A ".exit"
// command stops early with a success status, mirroring the interactive shell.
func (s *Shell) runBatch(ctx context.Context) error {
	scanner := bufio.NewScanner(s.stdin)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxBatchLineBytes)

	failed := false
	stmtNo := 0
	exited := false

	run := func(stmt string) {
		stmtNo++
		if err := s.exec(ctx, stmt); err != nil {
			if errors.Is(err, ErrExitSqly) {
				exited = true
				return
			}
			fmt.Fprintf(config.Stderr, "batch statement %d failed: %q: %v\n", stmtNo, stmt, err)
			failed = true
		}
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
				run(trimmed)
				if exited {
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
			run(stmt)
			if exited {
				break scan
			}
		}
	}

	// On ".exit", stop reading but still report any earlier failure below.
	if !exited {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read batch input: %w", err)
		}
		// Execute any trailing statement that was not terminated by ";".
		if leftover := strings.TrimSpace(pending.String()); leftover != "" {
			run(leftover)
		}
	}

	if failed {
		return errors.New("one or more batch statements failed")
	}
	return nil
}

// splitSQLStatements splits accumulated text into complete statements terminated
// by a top-level ";" and returns the trailing unterminated remainder. Semicolons
// inside string literals, identifiers, and comments are ignored so they do not
// split a statement mid-value.
func splitSQLStatements(s string) (stmts []string, remainder string) {
	runes := []rune(s)
	var (
		start                         int
		inSingle, inDouble            bool
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
		default:
			switch {
			case c == '\'':
				inSingle = true
			case c == '"':
				inDouble = true
			case c == '-' && i+1 < len(runes) && runes[i+1] == '-':
				inLineComment = true
				i++
			case c == '/' && i+1 < len(runes) && runes[i+1] == '*':
				inBlockComment = true
				i++
			case c == ';':
				if stmt := strings.TrimSpace(string(runes[start:i])); stmt != "" {
					stmts = append(stmts, stmt)
				}
				start = i + 1
			}
		}
	}
	return stmts, string(runes[start:])
}
