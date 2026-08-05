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

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// importFailedError is an import that read nothing it could use.
//
// It is a type rather than a message so the exit code is decided from what
// happened rather than from the shape of the words. An import is one operation:
// it either loaded every input it was given or none of them, so there is no
// "partly failed" for this to be a lesser version of.
type importFailedError struct {
	failed  int
	summary string
}

func (e *importFailedError) Error() string {
	return "import failed, and no table was created or changed: " + e.summary
}

// importCommand imports files into the in-memory database as one operation.
//
// Every path is resolved, every table name it will claim is checked, and then
// all of them are loaded in a single transaction. If any part fails, nothing is
// left behind: no table, no source record, no baseline, and no temporary file.
// See import_plan.go for why the phases are separated.
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
	return s.runImport(ctx, argv, argv)
}

// runImport is importCommand's body, with the labels to quote in messages kept
// separate from the paths being resolved. They differ only for an internal
// caller that resolves one place while the user named another.
func (s *Shell) runImport(ctx context.Context, argv, labels []string) error {
	plan, err := s.resolveImportPlan(ctx, argv, labels)
	if err != nil {
		return s.reportImportFailure(err)
	}
	defer plan.release()

	claims, err := s.preflightTableNames(ctx, plan)
	if err != nil {
		return s.reportImportFailure(err)
	}

	before, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return s.reportImportFailure(fmt.Errorf("failed to get table names before importing: %w", err))
	}
	beforeSet := tableNameSet(before)

	// One call, one transaction. A failure here rolls the whole thing back, so
	// the session is exactly as it was and the next line can say so plainly.
	if err := s.usecases.importer.LoadFiles(ctx, plan.loadPaths()...); err != nil {
		return s.reportImportFailure(s.describeLoadFailure(plan, err))
	}

	after, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return s.reportImportFailure(fmt.Errorf("failed to get table names after importing: %w", err))
	}
	afterSet := tableNameSet(after)

	for _, target := range plan.targets {
		s.warnSkippedExcelSheets(target.loadPath, target.displayPath)
	}

	imported := s.recordImportedTables(ctx, claims, afterSet)
	if len(imported) == 0 && len(diffTableNames(after, beforeSet)) == 0 {
		return s.reportImportFailure(importProducedNothing(plan))
	}

	// A successful import can change a table's columns without changing the
	// table-name set (re-import), so drop the cached completion suggestions.
	s.invalidateCompletionCache()

	// In inspect mode the structured report is the only intended output, so a
	// successful import stays quiet on stderr. Warnings (e.g. keyword table
	// names) and errors still print.
	if !s.reportOnly() && len(plan.directoryLabels) > 0 {
		fmt.Fprintf(s.importStatusWriter(), "Successfully imported %d table(s) from directory %s: %v\n",
			len(imported), strings.Join(plan.directoryLabels, ", "), imported)
	}
	return nil
}

// describeLoadFailure turns filesql's error into one that names the input the
// user typed. A staged download or a re-encoded copy has a temp path filesql
// quotes, and a user cannot act on a path they never wrote.
func (s *Shell) describeLoadFailure(plan *importPlan, err error) error {
	best := -1
	// The loader says which input it failed on, so the path is matched as a value
	// rather than looked for in the message.
	var importErr *model.ImportError
	if errors.As(err, &importErr) {
		for i, target := range plan.targets {
			if target.loadPath == importErr.Path {
				best = i
				break
			}
		}
		if best >= 0 {
			err = importErr.Err
		}
	}
	if best < 0 {
		// A failure from somewhere other than the per-file load names the path in
		// its text, if at all. The longest matching path wins: containment alone
		// is not exclusive, since one input's path can be a prefix of another's
		// ("/data/x" inside "/data/xy"), and the first match would then name a
		// file that imported perfectly well.
		for i, target := range plan.targets {
			if !strings.Contains(err.Error(), target.loadPath) {
				continue
			}
			if best < 0 || len(target.loadPath) > len(plan.targets[best].loadPath) {
				best = i
			}
		}
	}
	if best >= 0 {
		target := plan.targets[best]
		if msg, ok := s.stdinImportErrorMessage(target.displayPath, err, target.loadPath); ok {
			return errors.New(msg + s.rowMismatchAdvice(err))
		}
		if target.loadPath == target.displayPath {
			return fmt.Errorf("failed to import file %s: %w%s", target.displayPath, err, s.rowMismatchAdvice(err))
		}
		// Replace the staged path so the message quotes what the user named.
		return fmt.Errorf("failed to import file %s: %s%s", target.displayPath,
			strings.ReplaceAll(err.Error(), target.loadPath, target.displayPath), s.rowMismatchAdvice(err))
	}
	return err
}

// rowMismatchAdvice is the way out of an import that stopped on a short or long
// row, or "" for a failure that is about something else.
//
// The default policy is the one a user meets without choosing it, and it was the
// only one that did not name what changes it: `pad` says which flag it is
// refusing, while `error` — the one everybody hits first — said only that the
// counts differ. Which spelling to offer depends on where the import ran. A
// flag can only be passed when the process starts, so an .import typed into a
// running session is told about the helper command instead of a flag it has no
// way to reach.
func (s *Shell) rowMismatchAdvice(err error) string {
	if s.state.rowMismatch != model.RowMismatchError || !errors.Is(err, filesql.ErrColumnMismatch) {
		return ""
	}
	knob := rowMismatchCommand
	if s.importingStartupInputs {
		knob = rowMismatchFlag
	}
	return fmt.Sprintf("; use %s skip to drop such rows, or %s pad to fill short ones", knob, knob)
}

// reportImportFailure classifies an import failure.
//
// Everything here exits 3: the import read no input it could use. A bad command
// line is the exception and keeps its own class, because the thing to fix is
// what was typed rather than what was read.
//
// It returns the failure rather than printing it. Printing here and returning it
// as well meant every failing import said the same sentence twice, once from the
// import and once from whoever received the error — and a script's failure said
// it twice with the line number attached to only one of them.
func (s *Shell) reportImportFailure(err error) error {
	var invocationErr *invocationError
	if errors.As(err, &invocationErr) {
		return err
	}
	return &importFailedError{failed: 1, summary: err.Error()}
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

// clearDirImported removes the directory-import marker from the given tables, so
// a table first seen via a directory import becomes a normal file-backed table
// that write-back accepts once it is re-imported directly from a single file.
func (s *Shell) clearDirImported(names []string) {
	for _, name := range names {
		delete(s.dirImported, name)
	}
}

// absoluteSource is the one spelling --inspect gives an input: an absolute file
// path, or the URL a remote input was downloaded from, left exactly as it was.
//
// It is a single function because two fields of the report carry it —
// tables[].source and excel_sheets[].source — and the only way to join them is
// string equality. While one of them normalized and the other did not,
// `sqly --inspect book.xlsx` called the same file "/abs/path/book.xlsx" in one
// field and "book.xlsx" in the other, and a consumer holding both could not tell
// they were the same workbook.
//
// A path that cannot be made absolute is returned as it came. There is nothing
// better to say about it, and it is not worth failing a report over.
func absoluteSource(source string) string {
	if isRemoteURL(source) {
		return source
	}
	if abs, err := filepath.Abs(source); err == nil {
		return abs
	}
	return source
}

// recordTableSources maps each table to the file it was read from, which is what
// --inspect reports and what write-back writes to. The path is made absolute so
// it still names the right file after .cd. A table loaded as part of a directory
// import records that file too; whether it came from a directory is a separate
// mark (see markDirImported), because they answer different questions.
func (s *Shell) recordTableSources(ctx context.Context, tableNames []string, source string) {
	source = absoluteSource(source)
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
		// A file: URL is the one scheme "download it first" cannot be done for:
		// the file is already on this machine and the prefix is the whole problem,
		// so the advice is to drop it. Every other scheme names something sqly
		// cannot reach, where fetching it separately is the way through.
		if local, ok := localPathFromFileURL(path); ok {
			return fmt.Errorf("cannot import %s: sqly takes local paths directly; drop the %q prefix and pass %s",
				path, "file://", local)
		}
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
	// permissions, so legitimate deeply nested workspace paths must import.

	// Check for dangerous patterns that could indicate path traversal attacks.
	// URL-encoded sequences (..%2f, ..%5c) are intentionally NOT matched: the
	// filesystem never URL-decodes a path, so those bytes only ever appear in a
	// legitimate literal filename, and matching them rejected real files.
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
