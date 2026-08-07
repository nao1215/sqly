// Package persistence handle sqlite3, csv
package persistence

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
)

// History is a text file: one entry per line, appended as it is typed.
//
// It was a second SQLite database, which bought nothing a file does not already
// give. History is an append-only log — the shell reads the tail at startup and
// adds one entry per statement — and an append to a file opened O_APPEND is
// atomic per write on Linux, macOS, and Windows. That is the same interleaving
// guarantee the database's serialization provided, without the lock contention
// its five-second busy timeout existed to survive: two sqly processes sharing a
// history file interleave their lines instead of one of them waiting and then
// disabling history with a misleading "set a writable path" warning.
//
// What a file cannot make atomic is the trim, which rewrites the whole thing.
// That is a deliberate trade: a concurrent trim can drop a few of the other
// session's most recent entries, where the database dropped every entry of a
// session that lost the lock for five seconds.

// _ historyRepository implement HistoryRepository
var _ repository.HistoryRepository = (*historyRepository)(nil)

// maxHistoryEntries caps how many entries the file keeps. The shell preloads a
// hundred, so anything past a few thousand is weight nothing reads; the trim
// keeps the newest, which is the end a person scrolls back into.
const maxHistoryEntries = 5000

// maxHistoryLineBytes caps how long one encoded entry may be when reading. A
// statement can be long, so the default 64KiB scanner buffer is too small; past
// this, the line is treated as damage rather than as history.
const maxHistoryLineBytes = 8 * 1024 * 1024

// historyFilePerm is the permission a new history file gets. It records what
// someone typed, including a statement naming a path or a value they would not
// publish, so it is readable by its owner alone.
const historyFilePerm = 0o600

type historyRepository struct {
	// path is the file entries are appended to.
	path string
	// writable is false once Init finds the file cannot be written, so Append
	// stops trying. The shell disables history on the same signal; this keeps a
	// repository handed around after that from reopening the file per entry.
	writable bool
}

// NewHistoryRepository returns a HistoryRepository backed by the file at path.
func NewHistoryRepository(path string) repository.HistoryRepository {
	return &historyRepository{path: path}
}

// Init creates the history file if it is not there and reports whether it can be
// appended to, so an unwritable location is found once at startup rather than at
// the first statement.
func (h *historyRepository) Init(_ context.Context) error {
	if h.path == "" {
		return errors.New("history path is empty")
	}
	// The directory is not created here. config makes the default one under the
	// user's config home; a path the caller named with SQLY_HISTORY_PATH is
	// theirs, and a session that quietly built a tree for a mistyped one would
	// leave history somewhere nobody looks.
	f, err := os.OpenFile(filepath.Clean(h.path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, historyFilePerm)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close history file: %w", err)
	}
	h.writable = true
	return nil
}

// Append writes one entry, encoded onto a single line.
//
// The file is opened per call rather than held open for the session: a shell
// spends its life waiting for input, and a held descriptor is one a log rotation
// or a manual deletion leaves writing into a file nobody can see.
func (h *historyRepository) Append(_ context.Context, history model.History) error {
	if !h.writable {
		return errors.New("history is not writable")
	}
	line := encodeHistoryLine(history.Request)
	if line == "" {
		return nil
	}
	f, err := os.OpenFile(filepath.Clean(h.path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, historyFilePerm)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// One Write, so the kernel's O_APPEND guarantee covers the whole entry and
	// two sessions cannot interleave halves of a line.
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("append history: %w", err)
	}
	return nil
}

// List returns the retained entries, oldest first, and trims the file when it
// has grown past the cap.
//
// A line that does not decode is skipped rather than failing the read: a torn
// write from a killed process, or a file someone edited, should cost that one
// entry and not the session's whole history.
func (h *historyRepository) List(_ context.Context) (model.Histories, error) {
	// The read is its own call so the file is closed before a trim renames over
	// it. Windows refuses to replace a file that is still open, so holding the
	// descriptor until List returned left the trim silently doing nothing there.
	requests, err := h.readEntries()
	if err != nil {
		return nil, err
	}

	trimmed := false
	if len(requests) > maxHistoryEntries {
		requests = requests[len(requests)-maxHistoryEntries:]
		trimmed = true
	}

	histories := make(model.Histories, 0, len(requests))
	for i, request := range requests {
		// The id is the position in what was kept: the file records order, not
		// identity, and the shell only ever reads these back in order.
		histories = append(histories, model.NewHistory(i+1, request))
	}

	if trimmed && h.writable {
		// Best-effort: a trim that loses a race with another session's trim costs
		// a few entries, and saying so would be noise on a shell that started fine.
		_ = h.rewrite(requests)
	}
	return histories, nil
}

// readEntries decodes every entry the file holds, oldest first, and closes it
// before returning.
func (h *historyRepository) readEntries() (_ []string, err error) {
	f, err := os.Open(filepath.Clean(h.path))
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing typed yet is not a failure.
			return nil, nil
		}
		return nil, fmt.Errorf("open history file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close history file: %w", cerr)
		}
	}()

	requests := make([]string, 0, maxHistoryEntries)
	scanner := bufio.NewScanner(f)
	// A statement can be long, and the default 64KiB limit would end the scan
	// mid-file, silently shortening the history rather than skipping one entry.
	scanner.Buffer(make([]byte, 0, 64*1024), maxHistoryLineBytes)
	for scanner.Scan() {
		request, ok := decodeHistoryLine(scanner.Text())
		if !ok {
			continue
		}
		requests = append(requests, request)
	}
	if err := scanner.Err(); err != nil {
		// A line past the buffer is the same kind of damage as one that does not
		// decode: it costs what is left of the file, not the entries already read.
		// Returning the error instead would hand the shell a failed read, which
		// disables history for the session — one 8MiB line, or a file that is not
		// history at all, would cost every entry before it.
		if errors.Is(err, bufio.ErrTooLong) {
			return requests, nil
		}
		return nil, fmt.Errorf("read history file: %w", err)
	}
	return requests, nil
}

// rewrite replaces the history file with the given entries, staging beside it so
// a failure leaves the previous file whole.
func (h *historyRepository) rewrite(requests []string) (err error) {
	dir := filepath.Dir(h.path)
	tmp, err := os.CreateTemp(dir, ".sqly-history-*")
	if err != nil {
		return err
	}
	staging := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(staging)
		}
	}()

	w := bufio.NewWriter(tmp)
	for _, request := range requests {
		if _, err = w.WriteString(encodeHistoryLine(request) + "\n"); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err = w.Flush(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(staging, historyFilePerm); err != nil {
		return err
	}
	return os.Rename(staging, h.path)
}

// encodeHistoryLine renders one entry as a single line.
//
// The line format is what makes an append atomic, so a statement typed across
// several lines has to survive as one. Backslash escaping is the whole rule:
// every other byte is written as it is, so an ordinary statement is readable in
// the file with a text editor, which a length-prefixed or quoted format would
// have cost.
func encodeHistoryLine(request string) string {
	if strings.TrimSpace(request) == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(request))
	for _, r := range request {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// decodeHistoryLine reads back what encodeHistoryLine wrote, and reports whether
// the line held an entry. A blank line is not one.
func decodeHistoryLine(line string) (string, bool) {
	if strings.TrimSpace(line) == "" {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(line))
	escaped := false
	for _, r := range line {
		if escaped {
			switch r {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			default:
				// An escape nothing defines is written back as it was read, so a
				// hand-edited file loses no characters to a rule it did not follow.
				b.WriteByte('\\')
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	// A line ending in a lone backslash: keep it rather than dropping it.
	if escaped {
		b.WriteByte('\\')
	}
	return b.String(), true
}
