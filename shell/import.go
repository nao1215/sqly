package shell

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// errPartialImport is returned when some explicitly requested inputs imported
// successfully and at least one failed. Callers use errors.Is to decide whether
// to continue (interactive shell) or fail the run (non-interactive modes).
var errPartialImport = errors.New("one or more inputs failed to import")

// partialImportError carries the success and failure counts of a partial import
// alongside the one-line summary. It wraps errPartialImport so errors.Is still
// detects the condition, while errors.As lets the interactive shell report how
// much data loaded; non-interactive callers see the same detailed final line.
type partialImportError struct {
	succeeded int
	failed    int
	summary   string
}

// Error renders the detailed final line for non-interactive callers: the
// sentinel message followed by the per-input summary.
func (e *partialImportError) Error() string {
	return fmt.Sprintf("%s: %s", errPartialImport.Error(), e.summary)
}

// Unwrap exposes errPartialImport so errors.Is keeps matching the sentinel.
func (e *partialImportError) Unwrap() error { return errPartialImport }

// importFailedError is an import where nothing loaded: every requested input
// failed. It is the same class of problem as a partial import — an input sqly
// could not read — and is a type for the same reason, so the exit code is
// decided from what happened rather than from the shape of the message.
type importFailedError struct {
	failed  int
	summary string
}

func (e *importFailedError) Error() string {
	return fmt.Sprintf("all %d import(s) failed: %s", e.failed, e.summary)
}

// importCommand imports files into the in-memory database.
// Each file/directory is loaded individually so that same-name tables from
// different directories are overwritten (last-wins) rather than failing.
func (c CommandList) importCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		// A missing path argument is a command error so a batch script fails fast
		// instead of skipping the import and exiting 0. The usage rides on the error.
		//
		// It is an invocationError, not an import failure: the command as written
		// cannot be run, and nothing was read to fail at. That keeps a malformed
		// .import in the usage class with every other "you typed it wrong", rather
		// than reporting it as an input sqly could not read.
		return &invocationError{Err: errors.New(".import requires at least one file or directory path\n" + importUsageText())}
	}

	var errorMessages []string
	var successCount int

	for _, path := range argv {
		// Reject an empty path so `.import ""` does not silently import the
		// current working directory. Like a missing argument, this is decided from
		// the command line alone, so it is a usage error rather than an input that
		// could not be read.
		if strings.TrimSpace(path) == "" {
			return &invocationError{Err: errors.New(".import was given an empty path\n" + importUsageText())}
		}

		var pathImported bool
		err := func() error {
			cleanPath, cleanup, info, err := s.resolveImportTarget(ctx, path)
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			if info.IsDir() {
				imported, err := s.importDirectory(ctx, cleanPath, path)
				pathImported = imported
				return err
			}

			if err := s.importFile(ctx, cleanPath, path); err != nil {
				return err
			}
			pathImported = true
			return nil
		}()
		if err != nil {
			errorMessages = append(errorMessages, err.Error())
			continue
		}
		if pathImported {
			successCount++
		}
	}

	// A successful import can change a table's columns without changing the
	// table-name set (re-import), so drop the cached completion suggestions.
	if successCount > 0 {
		s.invalidateCompletionCache()
	}

	if len(errorMessages) > 0 {
		statusOut := s.importStatusWriter()
		if successCount > 0 {
			fmt.Fprintf(statusOut, "\nImport completed with %d successful import(s) and %d error(s):\n", successCount, len(errorMessages))
		} else {
			fmt.Fprintf(statusOut, "\nImport failed with %d error(s):\n", len(errorMessages))
		}
		for _, errMsg := range errorMessages {
			fmt.Fprintf(statusOut, "  - %s\n", errMsg)
		}
		// Also carry the per-input detail in the returned error, since wrappers and
		// logs often surface only the final error line; a generic message would
		// drop context already computed here.
		summary := summarizeImportErrors(errorMessages)
		if successCount == 0 {
			return &importFailedError{failed: len(errorMessages), summary: summary}
		}
		// A requested input failed while others succeeded. Return a
		// partialImportError so non-interactive runs exit non-zero and callers can
		// detect the sentinel with errors.Is, while the interactive shell reads the
		// counts to explain the shell state and starts with the tables that loaded.
		return &partialImportError{succeeded: successCount, failed: len(errorMessages), summary: summary}
	}

	return nil
}

// summarizeImportErrors condenses per-input failure messages into one line for
// the returned error: the first failure plus a "(+N more)" count when several
// inputs failed. The full list still goes to stderr; this keeps the final error
// line informative when only it is surfaced.
func summarizeImportErrors(messages []string) string {
	switch len(messages) {
	case 0:
		return ""
	case 1:
		return messages[0]
	default:
		return fmt.Sprintf("%s (+%d more)", messages[0], len(messages)-1)
	}
}

// importDirectory loads every supported file in a directory into the database,
// one file at a time, so each table can be mapped back to the exact file that
// produced it. Returns imported=true when at least one table was loaded or
// overwritten.
//
// Importing per file (rather than handing the whole directory to filesql) lets
// importDirectory:
//   - record each table's real source file even when the basename is sanitized
//     or the file yields several tables (Excel/ACH/Fedwire), so --inspect reports
//     per-file provenance ();
//   - reject two files in the tree that map to the same table name instead of
//     silently overwriting one with the other ();
//   - treat re-importing over an existing table as a reported overwrite, not "no
//     supported files", and re-point that table's source so write-back targets
//     the directory file rather than the original ().
//
// Every imported table is marked as a directory import so write-back still
// rejects it: a directory is not a single editable source the session owns.
func (s *Shell) importDirectory(ctx context.Context, cleanPath, displayPath string) (imported bool, err error) {
	files, err := s.supportedFilesInDir(cleanPath)
	if err != nil {
		return false, fmt.Errorf("failed to scan directory %s: %w", displayPath, err)
	}
	if len(files) == 0 {
		return false, fmt.Errorf("no supported files found in directory %s", displayPath)
	}

	// producedHere maps a table name to the file in this directory import that
	// produced it, so a later file mapping to the same name is a collision rather
	// than a silent overwrite.
	producedHere := make(map[string]string)
	var importedTables []string

	for _, file := range files {
		before, err := s.usecases.importer.GetTableNames(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to get table names before importing %s: %w", file, err)
		}
		beforeSet := tableNameSet(before)

		loadPath := file
		cleanupLoad := func() {}
		if prepared, cleanup, perr := s.prepareImportLoadPath(file); perr != nil {
			return false, fmt.Errorf("failed to prepare %s from directory %s: %w", file, displayPath, perr)
		} else if cleanup != nil {
			loadPath, cleanupLoad = prepared, cleanup
		}

		if err := s.usecases.importer.LoadFiles(ctx, loadPath); err != nil {
			cleanupLoad()
			return false, fmt.Errorf("failed to import file %s from directory %s: %w", file, displayPath, err)
		}
		s.warnSkippedExcelSheets(loadPath, file)
		cleanupLoad()

		// The tables this file owns are the ones it newly created. When it only
		// overwrote tables that already existed (a re-import), fall back to the
		// existing tables whose name matches this file's signature.
		fileTables := diffTableNames(mustTables(ctx, s), beforeSet)
		if len(fileTables) == 0 {
			// The file overwrote tables that already existed. Which ones is decided
			// by the record and by an exact name claim — never by a prefix another
			// file produces exactly.
			fileTables = s.tablesOverwrittenBy(file, beforeSet, true)
		}

		for _, name := range fileTables {
			if prev, ok := producedHere[name]; ok && prev != file {
				return false, fmt.Errorf("table-name collision: %s and %s both map to table %q in directory %s; rename a file to disambiguate", prev, file, name, displayPath)
			}
		}
		for _, name := range fileTables {
			producedHere[name] = file
			s.recordTableSources(ctx, []string{name}, file)
			s.markDirImported(name)
		}
		s.warnKeywordTableNames(fileTables)
		importedTables = append(importedTables, fileTables...)
	}

	if len(importedTables) == 0 {
		// The directory held supported files but none of them produced a table.
		return false, nil
	}

	sort.Strings(importedTables)
	// In inspect mode the structured report is the only intended output, so a
	// successful directory import stays quiet on stderr. Warnings (e.g. keyword
	// table names) and errors still print.
	if !s.reportOnly() {
		fmt.Fprintf(s.importStatusWriter(), "Successfully imported %d table(s) from directory %s: %v\n", len(importedTables), displayPath, importedTables)
	}
	return true, nil
}

// mustTables returns the current table names, or nil on error. importDirectory
// already validated the session by an earlier GetTableNames call in the same
// loop iteration, so a transient error here degrades to "no new tables" (the
// overwrite fallback) rather than aborting a successful import.
func mustTables(ctx context.Context, s *Shell) []*model.Table {
	tables, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return nil
	}
	return tables
}

// supportedFilesInDir returns the supported files under dir in deterministic
// order. A traversal error (e.g. an unreadable directory) is returned so the
// caller can surface the real access error.
func (s *Shell) supportedFilesInDir(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if s.usecases.importer.IsSupportedFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// tablesNamedAfterFile returns the table names in the given set that this file
// would produce: the sanitized base name, plus the "<base>_" prefixed names for
// the formats where one file becomes several tables.
//
// It answers "did something already take the name this import wanted?", and
// nothing else. It must not be used to decide what a file owns, because a name
// is not ownership: sample_test.csv and sample.xlsx produce names that look
// related and are not. tablesFromSource answers that question.
func (s *Shell) tablesNamedAfterFile(file string, names map[string]struct{}) []string {
	base := s.usecases.importer.GetTableNameFromFilePath(file)
	var matched []string
	if _, ok := names[base]; ok {
		matched = append(matched, base)
	}
	if s.usecases.importer.IsExcelFile(file) || model.IsInputOnlyExtension(file) {
		prefix := base + "_"
		for name := range names {
			if name != base && strings.HasPrefix(name, prefix) {
				matched = append(matched, name)
			}
		}
	}
	sort.Strings(matched)
	return matched
}

// tablesOverwrittenBy returns the tables an import of this file just replaced:
// the ones already recorded against it, plus its base name when nothing with a
// stronger claim holds it.
//
// A "<base>_" prefix is never a claim on its own. It looks like one for the
// formats where a file becomes several tables, and that is where this went
// wrong: a directory holding sample.xlsx and sample_test.csv let the workbook
// claim sample_test, so re-importing the workbook took a table it had never
// produced — clearing its directory marker and making a file the session must
// not write suddenly writable. A workbook's own sheet tables are recorded
// against it, so the record already covers them and the guess adds nothing but
// the bug.
//
// The base name is a real claim: this file does produce that table. Who may take
// it from whom depends on which side asked for the import, which is what
// fromDirectory says.
//
// A directory sweep takes any name it produces: loading a tree is last-wins by
// definition, and the alternative is a directory that refuses to load because
// something in it shares a name with an earlier argument. An individually named
// file is narrower: it takes a free name, a name it already holds, and a name
// held by a directory import — a bulk sweep yields to a file the user named —
// but not one another explicitly named file produced, which the caller reports
// as a collision.
func (s *Shell) tablesOverwrittenBy(file string, names map[string]struct{}, fromDirectory bool) []string {
	claimed := make(map[string]struct{})
	for _, name := range s.tablesFromSource(file, names) {
		claimed[name] = struct{}{}
	}
	base := s.usecases.importer.GetTableNameFromFilePath(file)
	if _, exists := names[base]; exists && (fromDirectory || !s.heldByAnotherExplicitFile(base)) {
		claimed[base] = struct{}{}
	}

	owned := make([]string, 0, len(claimed))
	for name := range claimed {
		owned = append(owned, name)
	}
	sort.Strings(owned)
	return owned
}

// heldByAnotherExplicitFile reports whether a different, individually named
// input produced this table. A table that arrived through a directory import
// does not count: that is the case an explicit import is allowed to take over.
func (s *Shell) heldByAnotherExplicitFile(name string) bool {
	if s.dirImported[name] {
		return false
	}
	source, ok := s.tableSources[name]
	return ok && source != stdinTableSource
}

// tablesFromSource returns the tables that were recorded as coming from this
// exact source when they were imported.
//
// This is what "the tables this file owns" means, and it is a lookup rather than
// a guess. The guess it replaces was the base name plus everything sharing it as
// a prefix, which is right for a workbook's own sheets and wrong for anything
// else named similarly: a directory holding sample.xlsx and sample_test.csv gave
// the workbook ownership of sample_test as well, so re-importing the workbook
// cleared the directory marker from a table it had never produced and made a
// file the session must not write become writable. The record is exact for every
// input, including directory imports, which register each file individually.
func (s *Shell) tablesFromSource(source string, names map[string]struct{}) []string {
	var owned []string
	for name := range names {
		recorded, ok := s.tableSources[name]
		if !ok || recorded == stdinTableSource {
			continue
		}
		if sameSourceLocation(source, recorded) {
			owned = append(owned, name)
		}
	}
	sort.Strings(owned)
	return owned
}

// warnKeywordTableNames warns when an imported table's name is a SQLite keyword.
// Such a name is created from the file name but is not queryable as a bare
// identifier ("SELECT * FROM select" is a syntax error); it must be quoted
// ("SELECT * FROM \"select\""). Warning at import time documents the gotcha
// instead of leaving the user with a table that silently fails in bare SQL. The
// table is still imported and is fully usable when quoted.
func (s *Shell) warnKeywordTableNames(names []string) {
	for _, name := range names {
		if model.IsReservedSQLiteKeyword(name) {
			fmt.Fprintf(s.importStatusWriter(),
				"warning: table %q is a SQLite keyword; quote it in queries, e.g. SELECT * FROM %s\n",
				name, s.usecases.importer.QuoteIdentifier(name))
		}
	}
}

// markDirImported records that a table came from a directory import, so
// write-back can reject it even when its source points at a single file.
func (s *Shell) markDirImported(name string) {
	if s.dirImported == nil {
		s.dirImported = make(map[string]bool)
	}
	s.dirImported[name] = true
}

// importFile loads a single file into the database, recording which tables it
// produced so write-back knows what the file owns.
// stdinImportErrorMessage renders an import failure for the staged --stdin-format
// dataset with the random temp staging path replaced by a stable
// "stdin (--stdin-format FORMAT)" reference. ok is false when displayPath is not the
// staged stdin file, so a normal file import keeps its real path and error
// wrapping. The candidate paths (the cleaned path and the path actually handed
// to filesql, plus their temp directories) are all scrubbed because filesql
// embeds the path it streamed inside its own error text.
func (s *Shell) stdinImportErrorMessage(displayPath string, err error, candidates ...string) (string, bool) {
	if s.stdinStagedPath == "" || displayPath != s.stdinStagedPath {
		return "", false
	}
	label := fmt.Sprintf("stdin (--stdin-format %s)", s.argument.StdinFormat)
	msg := fmt.Sprintf("failed to import file %s: %v", label, err)
	for _, p := range append(candidates, displayPath) {
		if p == "" {
			continue
		}
		msg = strings.ReplaceAll(msg, p, label)
		msg = strings.ReplaceAll(msg, filepath.Dir(p), label)
	}
	return msg, true
}

func (s *Shell) importFile(ctx context.Context, cleanPath, displayPath string) error {
	// loadPath is the path actually handed to filesql. It differs from cleanPath
	// only for an extensionless pseudo-file (e.g. /dev/stdin, /proc/self/fd/0),
	// which is staged to a temporary CSV so it imports end-to-end.
	loadPath := cleanPath
	cleanupLoad := func() {}
	if !s.usecases.importer.IsSupportedFile(cleanPath) {
		staged, cleanup, ok := s.stagePseudoFileAsCSV(cleanPath)
		if !ok {
			return fmt.Errorf("unsupported file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed)", filepath.Base(cleanPath))
		}
		cleanupLoad = cleanup
		loadPath = staged
	}
	preparedPath, cleanupPrepared, err := s.prepareImportLoadPath(loadPath)
	if err != nil {
		cleanupLoad()
		return err
	}
	defer cleanupLoad()
	if cleanupPrepared != nil {
		defer cleanupPrepared()
	}
	loadPath = preparedPath

	// Capture which tables this file creates so --inspect and write-back (.save)
	// can map them back to their source path.
	before, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to get table names before importing %s: %w", displayPath, err)
	}
	existingTables := tableNameSet(before)

	if err := s.usecases.importer.LoadFiles(ctx, loadPath); err != nil {
		// A failed --stdin-format import would otherwise leak the random staging temp
		// path (in both this wrapper and the path filesql embeds in its own
		// error). Map it back to a stable "stdin (--stdin-format FORMAT)" reference.
		if msg, ok := s.stdinImportErrorMessage(displayPath, err, cleanPath, loadPath); ok {
			return errors.New(msg)
		}
		return fmt.Errorf("failed to import file %s: %w", displayPath, err)
	}
	s.warnSkippedExcelSheets(loadPath, displayPath)

	after, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to get table names after importing %s: %w", displayPath, err)
	}

	// A successful import that produced no new table means this file overwrote one
	// or more tables that already existed in the session.
	newNames := diffTableNames(after, existingTables)
	if len(newNames) == 0 {
		// Ownership is read from the record, never inferred from the name. Only the
		// tables this exact source produced are re-pointed or unmarked below; a
		// sibling table that merely shares a prefix keeps whatever it had, and a
		// table another file produced is a collision rather than a takeover —
		// which is the difference between this and a directory import, where
		// last-wins across the tree is the point.
		owned := s.tablesOverwrittenBy(displayPath, existingTables, false)
		switch {
		case len(owned) > 0:
			// Re-import of the same source path (including a symlink alias) is a
			// harmless last-wins overwrite. Take clean ownership so a table first
			// seen via a directory import becomes a normal file-backed table that
			// write-back accepts, and re-point it at the path the user named.
			s.recordTableSources(ctx, owned, displayPath)
			s.clearDirImported(owned)
			s.warnKeywordTableNames(owned)
			return nil
		case len(s.tablesNamedAfterFile(loadPath, existingTables)) == 0:
			// No table was created, and no table carries this file's name — so the
			// file did not collide with anything, it simply held no data. An Excel
			// workbook whose only sheet has no cells arrives here. Saying "collision"
			// would send the user looking for a second input that does not exist.
			return fmt.Errorf("%s produced no table; the file has no rows to import", displayPath)
		default:
			// Two distinct plain-file inputs sanitized to the same table name (for
			// example "a-b.csv" and "a_b.csv" both becoming "a_b"). filesql overwrote
			// the earlier table, which would leave the first file's source mapped to
			// the second file's rows, so fail instead of silently overwriting. Ref
			return fmt.Errorf("table-name collision: %s sanitizes to a table name already imported from another input; rename the file to disambiguate", displayPath)
		}
	}
	s.recordTableSources(ctx, newNames, displayPath)
	s.warnKeywordTableNames(newNames)

	return nil
}

// clearDirImported removes the directory-import marker from the given tables, so
// a table first seen via a directory import becomes a normal file-backed table
// that write-back accepts once it is re-imported directly from a single file.
func (s *Shell) clearDirImported(names []string) {
	for _, name := range names {
		delete(s.dirImported, name)
	}
}

// recordTableSources maps each table to the file it was read from, which is what
// --inspect reports and what write-back writes to. The path is made absolute so
// it still names the right file after .cd. A table loaded as part of a directory
// import records that file too; whether it came from a directory is a separate
// mark (see markDirImported), because they answer different questions.
func (s *Shell) recordTableSources(ctx context.Context, tableNames []string, source string) {
	if !isRemoteURL(source) {
		if abs, err := filepath.Abs(source); err == nil {
			source = abs
		}
	}
	if s.tableSources == nil {
		s.tableSources = make(map[string]string)
	}
	for _, name := range tableNames {
		s.tableSources[name] = source
		s.snapshotBaseline(ctx, name)
	}
}

func (s *Shell) resolveImportTarget(ctx context.Context, input string) (cleanPath string, cleanup func(), info os.FileInfo, err error) {
	if isRemoteURL(input) {
		staged, cleanup, err := s.downloadRemoteInput(ctx, input)
		if err != nil {
			return "", nil, nil, err
		}
		info, err := os.Stat(staged)
		if err != nil {
			cleanup()
			return "", nil, nil, fmt.Errorf("failed to access downloaded file for %s: %w", input, err)
		}
		return staged, cleanup, info, nil
	}

	expanded, err := expandTilde(input)
	if err != nil {
		return "", nil, nil, fmt.Errorf("invalid path %s: %w", input, err)
	}
	cleanPath, err = validatePath(expanded)
	if err != nil {
		return "", nil, nil, fmt.Errorf("invalid path %s: %w", input, err)
	}
	info, err = os.Stat(cleanPath)
	if err != nil {
		return "", nil, nil, localImportAccessError(input, err)
	}
	return cleanPath, nil, info, nil
}

func localImportAccessError(path string, err error) error {
	// A URL sqly cannot fetch reaches here as a local path the filesystem could
	// not open, and which error it produced depends on the platform: a missing
	// file on Unix, an invalid filename on Windows, where "s3://bucket/x.csv"
	// becomes ".\\s3:\\bucket\\x.csv" and the drive-letter colon is rejected before
	// anything is looked up. The scheme is the useful thing to say either way, so
	// it is checked before the error kind rather than inside one branch.
	if scheme := unfetchableURLScheme(path); scheme != "" {
		return fmt.Errorf(
			"cannot import %s: only http and https URLs are downloaded, but this one uses the %q scheme; download the file first and pass its local path",
			path, scheme)
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		return errors.New("path does not exist: " + path)
	case errors.Is(err, os.ErrPermission):
		return errors.New("permission denied accessing path: " + path)
	default:
		return fmt.Errorf("failed to access path %s: %w", path, err)
	}
}

// snapshotBaseline records the content fingerprint of a freshly imported table so
// write-back can later tell whether the table changed. A fingerprint that cannot be
// computed is left unset, which makes the table count as changed (a safe default
// that never skips a real change).
func (s *Shell) snapshotBaseline(ctx context.Context, name string) {
	fp, err := s.tableContentFingerprint(ctx, name)
	if err != nil {
		return
	}
	if s.importBaseline == nil {
		s.importBaseline = make(map[string]string)
	}
	s.importBaseline[name] = fp
	s.snapshotSourceBaseline(name, fp)
}

// snapshotSourceBaseline records what the table's source file now holds. Import
// sets it alongside the import baseline; an in-place save moves it on its own,
// because that is the only operation that makes the source match the table.
func (s *Shell) snapshotSourceBaseline(name, fingerprint string) {
	if s.sourceBaseline == nil {
		s.sourceBaseline = make(map[string]string)
	}
	s.sourceBaseline[name] = fingerprint
}

// snapshotSourceFromTable recomputes the fingerprint and records it as the
// source's, for use after an in-place save has written the table out.
func (s *Shell) snapshotSourceFromTable(ctx context.Context, name string) {
	fp, err := s.tableContentFingerprint(ctx, name)
	if err != nil {
		return
	}
	s.snapshotSourceBaseline(name, fp)
}

// tableContentFingerprint returns a hash of a table's current relational content
// (header then every record, in row order). Fields are length-prefixed so distinct
// shapes cannot collide (["a","b"] differs from ["ab"]). Write-back compares this
// against the import baseline to skip a table whose content did not change.
func (s *Shell) tableContentFingerprint(ctx context.Context, name string) (string, error) {
	if s.usecases.metadata == nil {
		return "", errors.New("metadata usecase is unavailable")
	}
	t, err := s.usecases.metadata.List(ctx, name)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	var lenBuf [8]byte
	writeField := func(f string) {
		binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(f)))
		_, _ = h.Write(lenBuf[:])
		_, _ = h.Write([]byte(f))
	}
	for _, col := range t.Columns {
		writeField(col)
	}
	for _, rec := range t.Rows {
		writeField("\x00") // row separator that no column value can forge
		for i := range rec.Len() {
			writeField(rec.At(i))
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// tableChanged reports whether a table's current content differs from the baseline
// captured at import. A table with no baseline (never imported, or whose baseline
// could not be computed) is treated as changed, so write-back never skips a table
// it is unsure about.
func (s *Shell) tableChanged(ctx context.Context, name string) bool {
	return s.tableDiffersFrom(ctx, s.importBaseline, name)
}

// tableNeedsSourceWrite reports whether a table's content differs from what its
// source file holds. An in-place save asks this: a table already written out
// needs no second write, and rewriting it would churn the file's bytes for no
// change.
func (s *Shell) tableNeedsSourceWrite(ctx context.Context, name string) bool {
	return s.tableDiffersFrom(ctx, s.sourceBaseline, name)
}

// tableDiffersFrom compares a table's current content against one of the two
// baselines. A table with no entry is treated as different, so write-back never
// skips a table it is unsure about.
func (s *Shell) tableDiffersFrom(ctx context.Context, baselines map[string]string, name string) bool {
	baseline, ok := baselines[name]
	if !ok {
		return true
	}
	current, err := s.tableContentFingerprint(ctx, name)
	if err != nil {
		return true
	}
	return current != baseline
}

// importStatusWriter returns where import progress and error messages go.
// Import diagnostics are control-plane output, so they always go to stderr.
// This keeps stdout reserved for query results and the --inspect JSON report,
// so machine-readable output is never mixed with import banners.
func (s *Shell) importStatusWriter() io.Writer {
	return config.Stderr
}

// tableNameSet creates a set of table names from a slice for O(1) lookup.
func tableNameSet(tables []*model.Table) map[string]struct{} {
	set := make(map[string]struct{}, len(tables))
	for _, t := range tables {
		set[t.Name()] = struct{}{}
	}
	return set
}

// diffTableNames returns table names present in after but not in existingSet.
func diffTableNames(tables []*model.Table, existingSet map[string]struct{}) []string {
	var names []string
	for _, t := range tables {
		if _, exists := existingSet[t.Name()]; !exists {
			names = append(names, t.Name())
		}
	}
	return names
}

// validatePath validates a path to prevent directory traversal attacks.
// It returns the cleaned path and an error if the path contains dangerous patterns.
func validatePath(path string) (string, error) {
	// Clean the path to resolve any ".." or "." components
	cleanPath := filepath.Clean(path)

	// No directory-depth limit: sqly is a local CLI run with the user's own
	// permissions, so legitimate deeply nested workspace paths must import. Ref

	// Check for dangerous patterns that could indicate path traversal attacks.
	// URL-encoded sequences (..%2f, ..%5c) are intentionally NOT matched: the
	// filesystem never URL-decodes a path, so those bytes only ever appear in a
	// legitimate literal filename, and matching them rejected real files. Ref
	dangerousPatterns := []string{
		"../../../",    // Multiple levels up
		"..\\..\\..\\", // Windows path traversal
		"....//",       // Double encoding attempts
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(strings.ToLower(path), pattern) {
			return "", fmt.Errorf("potentially dangerous path pattern detected: %s", path)
		}
	}

	// Prevent access to system directories on Unix-like systems. Validate both
	// the raw absolute path and its symlink-resolved target, so a symlink to
	// /etc/hosts is rejected like the direct path. EvalSymlinks only resolves
	// existing paths, so its error is ignored for paths that do not exist yet.
	absPath, err := filepath.Abs(cleanPath)
	if err == nil {
		if isBlockedSystemPath(absPath) {
			return "", fmt.Errorf("access to system directory not allowed: %s", path)
		}
		// Skip symlink resolution for allowed pseudo-files: their fd aliases
		// (e.g. /dev/stdin, /proc/self/fd/0) legitimately resolve to devices or
		// pipes under /dev or /proc, which would otherwise look blocked.
		if !isAllowedPseudoFile(absPath) {
			if resolved, rerr := filepath.EvalSymlinks(absPath); rerr == nil && isBlockedSystemPath(resolved) {
				return "", fmt.Errorf("access to system directory not allowed: %s", path)
			}
		}
	}

	return cleanPath, nil
}

// isBlockedSystemPath reports whether absPath (already absolute) targets a
// protected OS directory such as /etc, /proc, or /dev. On macOS these live under
// /private (/etc is a symlink to /private/etc), so a /private-prefixed resolved
// target is normalized before comparison. Allowed Unix pseudo-files are not
// treated as blocked.
func isBlockedSystemPath(absPath string) bool {
	if isAllowedPseudoFile(absPath) {
		return false
	}
	candidates := []string{absPath}
	if stripped, ok := strings.CutPrefix(absPath, "/private"); ok && strings.HasPrefix(stripped, "/") {
		candidates = append(candidates, stripped)
	}
	for _, p := range candidates {
		for _, sysDir := range []string{"/etc", "/proc", "/sys", "/dev", "/boot"} {
			if p == sysDir || strings.HasPrefix(p, sysDir+"/") {
				return true
			}
		}
	}
	return false
}

// stagePseudoFileAsCSV stages an extensionless Unix pseudo-file (e.g. /dev/stdin,
// /dev/fd/0, /proc/self/fd/0) into a temporary CSV file so it imports end-to-end,
// returning the temp path, a cleanup, and whether staging applied. filesql types a
// file by its name, and these pseudo-files carry no format extension, so their
// content is copied to "<table>.csv" and read as CSV (sqly's default text format);
// use --stdin-format FORMAT for a non-CSV stream. Only the allowed pseudo-files are
// staged, so an ordinary unsupported path still fails as before.
func (s *Shell) stagePseudoFileAsCSV(path string) (stagedPath string, cleanup func(), ok bool) {
	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}
	if !isAllowedPseudoFile(abs) {
		return "", nil, false
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is a validated pseudo-file input
	if err != nil {
		return "", nil, false
	}
	dir, err := os.MkdirTemp("", "sqly-pseudo-")
	if err != nil {
		return "", nil, false
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	// The staged path is a freshly created temp dir joined with the sanitized table
	// name, so it cannot escape the temp dir.
	stagedPath = filepath.Join(dir, s.usecases.importer.GetTableNameFromFilePath(path)+model.ExtCSV)
	if err := os.WriteFile(stagedPath, data, 0o600); err != nil { //nolint:gosec // stagedPath is under a sqly-created temp dir with a sanitized name
		cleanup()
		return "", nil, false
	}
	return stagedPath, cleanup, true
}

// isAllowedPseudoFile reports whether an absolute path is a standard Unix
// pseudo-file that holds legitimate, user-controlled input even though it lives
// under a system directory. These are exempt from the system-directory guard:
//   - /dev/shm/*            tmpfs for user data ()
//   - /dev/fd/*             open file descriptors, process substitution ()
//   - /dev/stdin, /dev/stdout, /dev/stderr  standard stream pseudo-files ()
//   - /proc/<pid|self>/fd/* the Linux fd aliases behind many fd-based workflows ()
func isAllowedPseudoFile(absPath string) bool {
	switch absPath {
	case devStdinPath, "/dev/stdout", "/dev/stderr":
		return true
	}
	for _, prefix := range []string{"/dev/shm/", "/dev/fd/"} {
		if strings.HasPrefix(absPath, prefix) {
			return true
		}
	}
	for _, base := range []string{"/dev/shm", "/dev/fd"} {
		if absPath == base {
			return true
		}
	}
	// /proc/self/fd/* and /proc/<pid>/fd/* are the Linux aliases for open file
	// descriptors that shells use for process substitution and fd redirection.
	if rest, ok := strings.CutPrefix(absPath, "/proc/"); ok {
		if slash := strings.IndexByte(rest, '/'); slash > 0 {
			owner, tail := rest[:slash], rest[slash+1:]
			if (owner == "self" || isAllDigits(owner)) && strings.HasPrefix(tail, "fd/") {
				return true
			}
		}
	}
	return false
}

// isAllDigits reports whether s is non-empty and contains only ASCII digits, used
// to match a numeric /proc/<pid> component.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// importUsageText returns the .import command usage block. It is attached to the
// error returned for a missing path argument so a batch script fails fast
// instead of skipping the import and exiting 0.
func importUsageText() string {
	return "[Usage]\n" +
		"  .import FILE_PATH(S)|DIRECTORY_PATH(S)\n" +
		"\n" +
		"  - Quote arguments that contain spaces: .import \"my data.csv\"\n" +
		"\n" +
		"  - Supported file format: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed\n" +
		"  - Compression (csv/tsv/ltsv/json/jsonl/parquet/xlsx only): .gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4\n" +
		"  - Files and directories can be mixed in arguments\n" +
		"  - Directories are automatically detected and all supported files are imported\n" +
		"  - If import multiple files/directories, separate them with spaces\n" +
		"  - For Excel files, each sheet the workbook shows becomes its own table (enables cross-sheet JOINs);\n" +
		"    start sqly with --include-hidden-sheets to import the hidden ones too\n" +
		"  - JSON/JSONL data is stored in a 'data' column; use json_extract() to query fields"
}
