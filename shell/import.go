package shell

import (
	"context"
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

const (
	// sheetFlag is the .import flag selecting a single Excel sheet. It accepts
	// both the separated form "--sheet NAME" and the joined form "--sheet=NAME".
	sheetFlag = "--sheet"
	// sheetFlagAssign is the joined form prefix of sheetFlag.
	sheetFlagAssign = sheetFlag + "="
)

// errPartialImport is returned when some explicitly requested inputs imported
// successfully and at least one failed. Callers use errors.Is to decide whether
// to continue (interactive shell) or fail the run (non-interactive modes).
var errPartialImport = errors.New("one or more inputs failed to import")

// errSheetNotFound marks a --sheet filter that matched no sheet in a particular
// workbook. In a multi-workbook import it is downgraded to a non-fatal skip so a
// single non-matching workbook cannot suppress the workbooks that do match.
// Ref #378.
var errSheetNotFound = errors.New("requested sheet not found in workbook")

// validateSheetFlag rejects the CLI --sheet option when no input can be an
// Excel file. --sheet selects a single Excel sheet and is silently ignored for
// other formats, so a typo (or pairing it with --stdin) would otherwise pass
// unnoticed. A directory input is allowed because it may contain Excel files,
// and a path that cannot be stat'd is left for the import step to report.
func (s *Shell) validateSheetFlag() error {
	if s.argument.SheetName == "" {
		return nil
	}
	if s.sheetAppliesTo(s.argument.FilePaths) {
		return nil
	}
	return errors.New("--sheet is only valid for Excel (.xlsx) inputs")
}

// sheetAppliesTo reports whether --sheet can affect any of the given input
// paths: a path is meaningful for --sheet when it is an Excel file or a
// directory that contains at least one Excel file. A path that cannot be stat'd
// is treated as applicable so the import step reports the real path error
// instead of this validation misattributing it to --sheet.
func (s *Shell) sheetAppliesTo(paths []string) bool {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return true
		}
		if info.IsDir() {
			contains, walkErr := s.dirContainsExcel(path)
			if walkErr != nil {
				// The directory exists but cannot be traversed (e.g. permission
				// denied), so whether it holds an Excel file is unknown. Defer to the
				// import step, which surfaces the real access error instead of this
				// validation misattributing it to --sheet. Ref #356.
				return true
			}
			if contains {
				return true
			}
			continue
		}
		if s.usecases.importer.IsExcelFile(path) {
			return true
		}
	}
	return false
}

// dirContainsExcel reports whether dir contains at least one Excel file,
// searched recursively. It returns a non-nil error when the directory cannot be
// traversed (e.g. permission denied), so callers can distinguish "no Excel
// match" from "could not determine" and defer to the import step. Ref #356.
func (s *Shell) dirContainsExcel(dir string) (bool, error) {
	found := false
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // a traversal error (unreadable dir) is reported, not swallowed
		}
		if found {
			return nil
		}
		if !d.IsDir() && s.usecases.importer.IsExcelFile(path) {
			found = true
		}
		return nil
	})
	return found, walkErr
}

// importPaths returns the file/directory arguments from a .import argv,
// excluding the --sheet flag and its value in both the separated and joined
// forms. It mirrors the flag handling in importCommand's main loop.
func importPaths(argv []string) []string {
	var paths []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == sheetFlag {
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
				i++ // skip the separated sheet value
			}
			continue
		}
		if strings.HasPrefix(a, sheetFlagAssign) {
			continue
		}
		paths = append(paths, a)
	}
	return paths
}

// importCommand imports files into the in-memory database.
// Each file/directory is loaded individually so that same-name tables from
// different directories are overwritten (last-wins) rather than failing,
// and --sheet filtering is scoped to the correct Excel file.
func (c CommandList) importCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		printImportUsage()
		return nil
	}

	// Reject an explicit empty helper --sheet value (separated `--sheet ""` or
	// joined `--sheet=`) before any file/Excel checks, so it is not silently
	// treated as "no sheet filter". Ref #354, #355.
	if helperSheetExplicitlyEmpty(argv) {
		return errors.New("--sheet requires a non-empty sheet name")
	}

	sheetName := s.argument.SheetName
	if sheetName == "" {
		sheetName = extractSheetNameFromArgs(argv)
	}

	// Reject --sheet when none of the inputs is an Excel file (or a directory
	// containing one), so the flag is not silently ignored. Ref #312.
	if sheetName != "" && !s.sheetAppliesTo(importPaths(argv)) {
		return errors.New("--sheet is only valid for Excel (.xlsx) inputs")
	}

	// A --sheet filter that misses one workbook must not fail a multi-workbook
	// import: count the Excel workbooks targeted so a miss can be downgraded to a
	// non-fatal skip when more than one workbook is involved. Ref #378.
	multiWorkbook := sheetName != "" && s.countExcelWorkbooks(importPaths(argv)) > 1

	var errorMessages []string
	var successCount int
	var skippedSheet int

	for i := 0; i < len(argv); i++ {
		path := argv[i]
		if path == sheetFlag {
			// Require a value (e.g. --sheet "Q1 Sales"). A trailing --sheet or
			// one followed by another flag is rejected instead of silently
			// importing nothing, which would hide the mistake in scripts.
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
				i++ // consume the separated value
				continue
			}
			return fmt.Errorf("%s requires a value", sheetFlag)
		}
		if strings.HasPrefix(path, sheetFlagAssign) {
			continue
		}

		// Reject an empty path so `.import ""` does not silently import the
		// current working directory. Ref #325.
		if strings.TrimSpace(path) == "" {
			errorMessages = append(errorMessages, "empty import path")
			continue
		}

		cleanPath, err := validatePath(path)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("invalid path %s: %v", path, err))
			continue
		}

		info, err := os.Stat(cleanPath)
		if err != nil {
			switch {
			case os.IsNotExist(err):
				errorMessages = append(errorMessages, "path does not exist: "+path)
			case os.IsPermission(err):
				errorMessages = append(errorMessages, "permission denied accessing path: "+path)
			default:
				errorMessages = append(errorMessages, fmt.Sprintf("failed to access path %s: %v", path, err))
			}
			continue
		}

		if info.IsDir() {
			imported, skipped, err := s.importDirectory(ctx, cleanPath, path, sheetName, multiWorkbook)
			skippedSheet += skipped
			if err != nil {
				errorMessages = append(errorMessages, err.Error())
				continue
			}
			if imported {
				successCount++
			}
		} else {
			if err := s.importFile(ctx, cleanPath, path, sheetName); err != nil {
				// In a multi-workbook import, a workbook that lacks the requested
				// sheet is skipped rather than failing the whole run. Ref #378.
				if multiWorkbook && errors.Is(err, errSheetNotFound) {
					fmt.Fprintf(s.importStatusWriter(), "Skipped %s: %v\n", path, err)
					skippedSheet++
					continue
				}
				errorMessages = append(errorMessages, err.Error())
				continue
			}
			successCount++
		}
	}

	// Every workbook was skipped because none contained the requested sheet, and
	// nothing else imported. Fail loudly instead of succeeding with no tables so
	// a wrong --sheet value is visible. Ref #378.
	if successCount == 0 && skippedSheet > 0 && len(errorMessages) == 0 {
		return fmt.Errorf("sheet %q not found in any of the imported workbooks", sheetName)
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
		if successCount == 0 {
			return errors.New("all import attempts failed")
		}
		// An explicitly requested input failed even though others succeeded.
		// Return a partial-failure error so non-interactive runs exit non-zero
		// (Ref #297, #300, #302); the interactive shell tolerates it and starts
		// with the tables that did load.
		return errPartialImport
	}

	return nil
}

// importDirectory loads every supported file in a directory into the database,
// one file at a time, so each table can be mapped back to the exact file that
// produced it. Returns imported=true when at least one table was loaded or
// overwritten, plus the number of workbooks skipped because they lacked the
// requested --sheet (Ref #378).
//
// Importing per file (rather than handing the whole directory to filesql) lets
// importDirectory:
//   - record each table's real source file even when the basename is sanitized
//     or the file yields several tables (Excel/ACH/Fedwire), so --inspect reports
//     per-file provenance (Ref #357, #358);
//   - reject two files in the tree that map to the same table name instead of
//     silently overwriting one with the other (Ref #359, #360);
//   - treat re-importing over an existing table as a reported overwrite, not "no
//     supported files", and re-point that table's source so write-back targets
//     the directory file rather than the original (Ref #361, #362).
//
// Every imported table is marked as a directory import so write-back still
// rejects it: a directory is not a single editable source the session owns.
func (s *Shell) importDirectory(ctx context.Context, cleanPath, displayPath, sheetName string, multiWorkbook bool) (imported bool, skipped int, err error) {
	files, err := s.supportedFilesInDir(cleanPath)
	if err != nil {
		return false, 0, fmt.Errorf("failed to scan directory %s: %w", displayPath, err)
	}
	if len(files) == 0 {
		return false, 0, fmt.Errorf("no supported files found in directory %s", displayPath)
	}

	// producedHere maps a table name to the file in this directory import that
	// produced it, so a later file mapping to the same name is a collision rather
	// than a silent overwrite. Ref #359, #360.
	producedHere := make(map[string]string)
	var importedTables []string

	for _, file := range files {
		before, err := s.usecases.importer.GetTableNames(ctx)
		if err != nil {
			return false, skipped, fmt.Errorf("failed to get table names before importing %s: %w", file, err)
		}
		beforeSet := tableNameSet(before)

		if err := s.usecases.importer.LoadFiles(ctx, file); err != nil {
			return false, skipped, fmt.Errorf("failed to import file %s from directory %s: %w", file, displayPath, err)
		}

		// Apply --sheet filtering to Excel workbooks. A workbook that lacks the
		// requested sheet is skipped in a multi-workbook import (Ref #378);
		// filterExcelSheets has already dropped its tables before returning.
		if sheetName != "" && s.usecases.importer.IsExcelFile(file) {
			if ferr := s.filterExcelSheets(ctx, file, sheetName, nil); ferr != nil {
				if multiWorkbook && errors.Is(ferr, errSheetNotFound) {
					fmt.Fprintf(s.importStatusWriter(), "Skipped %s: %v\n", file, ferr)
					skipped++
					continue
				}
				return false, skipped, ferr
			}
		}

		// The tables this file owns are the ones it newly created. When it only
		// overwrote tables that already existed (a re-import), fall back to the
		// existing tables whose name matches this file's signature.
		fileTables := diffTableNames(mustTables(ctx, s), beforeSet)
		if len(fileTables) == 0 {
			fileTables = s.tablesMatchingFile(file, beforeSet)
		}

		for _, name := range fileTables {
			if prev, ok := producedHere[name]; ok && prev != file {
				return false, skipped, fmt.Errorf("table-name collision: %s and %s both map to table %q in directory %s; rename a file to disambiguate", prev, file, name, displayPath)
			}
		}
		for _, name := range fileTables {
			producedHere[name] = file
			s.recordTableSources([]string{name}, file)
			s.markDirImported(name)
		}
		s.warnKeywordTableNames(fileTables)
		importedTables = append(importedTables, fileTables...)
	}

	if len(importedTables) == 0 {
		// Every supported file was skipped (e.g. all workbooks lacked the --sheet).
		// The caller turns an all-skipped run into a clear error via skipped.
		return false, skipped, nil
	}

	sort.Strings(importedTables)
	fmt.Fprintf(s.importStatusWriter(), "Successfully imported %d table(s) from directory %s: %v\n", len(importedTables), displayPath, importedTables)
	return true, skipped, nil
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

// tablesMatchingFile returns the table names in the given set that a file owns by
// name. A single-table format (CSV/TSV/LTSV/JSON/Parquet) owns only the exact
// table name derived from its path. The "<base>_" prefix is matched only for
// multi-table formats (Excel sheets, ACH, Fedwire), where one file produces
// "<base>_<sheet>" tables. Without that restriction a file like "a.csv" would
// spuriously claim unrelated tables such as "a_b". Ref #429. It is used to
// identify which existing tables a re-imported file overwrote.
func (s *Shell) tablesMatchingFile(file string, names map[string]struct{}) []string {
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
	return matched
}

// countExcelWorkbooks counts the Excel workbooks reachable from the given input
// paths: a path that is itself an Excel file counts as one, and a directory is
// walked to count the Excel files it contains. It is used to decide whether a
// --sheet miss in one workbook should be a non-fatal skip (more than one
// workbook) or a hard error (a single workbook). Ref #378.
func (s *Shell) countExcelWorkbooks(paths []string) int {
	count := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return nil //nolint:nilerr // unreadable entries are surfaced by the import step
				}
				if s.usecases.importer.IsExcelFile(p) {
					count++
				}
				return nil
			})
			continue
		}
		if s.usecases.importer.IsExcelFile(path) {
			count++
		}
	}
	return count
}

// warnKeywordTableNames warns when an imported table's name is a SQLite keyword.
// Such a name is created from the file name but is not queryable as a bare
// identifier ("SELECT * FROM select" is a syntax error); it must be quoted
// ("SELECT * FROM \"select\""). Warning at import time documents the gotcha
// instead of leaving the user with a table that silently fails in bare SQL. The
// table is still imported and is fully usable when quoted. Ref #424.
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

// importFile loads a single file into the database, applying --sheet filtering for Excel.
func (s *Shell) importFile(ctx context.Context, cleanPath, displayPath, sheetName string) error {
	if !s.usecases.importer.IsSupportedFile(cleanPath) {
		return fmt.Errorf("unsupported file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed)", filepath.Base(cleanPath))
	}

	// Capture which tables this file creates so --inspect and write-back (.save)
	// can map them back to their source path.
	before, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to get table names before importing %s: %w", displayPath, err)
	}
	existingTables := tableNameSet(before)

	if err := s.usecases.importer.LoadFiles(ctx, cleanPath); err != nil {
		return fmt.Errorf("failed to import file %s: %w", displayPath, err)
	}

	// Apply --sheet filtering only to Excel files.
	// Use prefix matching over current tables. LoadFiles just created or
	// overwrote these tables, so all prefix-matching tables belong to this file.
	// nil candidates tells filterExcelSheets to build the set from prefix match.
	if s.usecases.importer.IsExcelFile(cleanPath) && sheetName != "" {
		if err := s.filterExcelSheets(ctx, cleanPath, sheetName, nil); err != nil {
			return err
		}
	}

	after, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to get table names after importing %s: %w", displayPath, err)
	}

	// A successful import that produced no new table means this file overwrote one
	// or more tables that already existed in the session.
	newNames := diffTableNames(after, existingTables)
	if len(newNames) == 0 {
		owned := s.tablesMatchingFile(cleanPath, existingTables)
		switch {
		case s.isRecordedSource(displayPath):
			// Re-import of the same source path (including a symlink alias) is a
			// harmless last-wins overwrite. Take clean ownership so a table first
			// seen via a directory import becomes a normal file-backed table that
			// write-back accepts. Ref #415, #417.
			s.clearDirImported(owned)
			return nil
		case s.anyDirImported(owned):
			// A deliberate single-file .import replaces directory-sourced data of the
			// same name: re-point the table to this standalone file and drop the
			// directory marker so write-back targets the file the user named. Ref
			// #416.
			s.recordTableSources(owned, displayPath)
			s.clearDirImported(owned)
			s.warnKeywordTableNames(owned)
			return nil
		default:
			// Two distinct plain-file inputs sanitized to the same table name (for
			// example "a-b.csv" and "a_b.csv" both becoming "a_b"). filesql overwrote
			// the earlier table, which would leave the first file's source mapped to
			// the second file's rows, so fail instead of silently overwriting. Ref
			// #286.
			return fmt.Errorf("table-name collision: %s sanitizes to a table name already imported from another input; rename the file to disambiguate", displayPath)
		}
	}
	s.recordTableSources(newNames, displayPath)
	s.warnKeywordTableNames(newNames)

	return nil
}

// isRecordedSource reports whether path is already the source of an imported
// table. Paths are compared with symlink resolution, so a symlink alias of an
// imported source is recognized as the same source and re-importing through it is
// a harmless last-wins overwrite rather than a table-name collision. Ref #417.
func (s *Shell) isRecordedSource(path string) bool {
	for _, src := range s.tableSources {
		if src == stdinTableSource {
			continue
		}
		if sameFilePath(path, src) {
			return true
		}
	}
	return false
}

// clearDirImported removes the directory-import marker from the given tables, so
// a table first seen via a directory import becomes a normal file-backed table
// that write-back accepts once it is re-imported directly from a single file.
// Ref #415, #416.
func (s *Shell) clearDirImported(names []string) {
	for _, name := range names {
		delete(s.dirImported, name)
	}
}

// anyDirImported reports whether any of the given tables came from a directory
// import. It lets a deliberate single-file .import replace directory-sourced data
// of the same name, while two distinct plain-file inputs that collide are still
// rejected. Ref #416, #286.
func (s *Shell) anyDirImported(names []string) bool {
	for _, name := range names {
		if s.dirImported[name] {
			return true
		}
	}
	return false
}

// recordTableSources remembers which source path produced each table name, so
// the --inspect report and write-back (.save) can map a table back to its
// source. The source is resolved to an absolute path so write-back still targets
// the right file after the shell changes directory with .cd. For directory
// imports the source is the directory; write-back rejects those because it
// cannot tell which file in the directory owns the table.
func (s *Shell) recordTableSources(tableNames []string, source string) {
	if abs, err := filepath.Abs(source); err == nil {
		source = abs
	}
	if s.tableSources == nil {
		s.tableSources = make(map[string]string)
	}
	for _, name := range tableNames {
		s.tableSources[name] = source
	}
}

// importStatusWriter returns where import progress and error messages go.
// Import diagnostics are control-plane output, so they always go to stderr.
// This keeps stdout reserved for query results and the --inspect JSON report,
// so machine-readable output is never mixed with import banners. Ref #306.
func (s *Shell) importStatusWriter() io.Writer {
	return config.Stderr
}

// filterExcelSheets keeps only the requested sheet from a specific Excel file,
// operating on the given candidate set of table names. Callers should provide
// a candidates set scoped to tables owned by the current import to prevent
// prefix collisions between files with the same sanitized name.
// If candidates is nil, falls back to prefix matching over all current tables.
func (s *Shell) filterExcelSheets(ctx context.Context, excelPath, sheetName string, candidates map[string]struct{}) error {
	exactPrefix := s.usecases.importer.GetTableNameFromFilePath(excelPath) + "_"

	// If no candidate set was provided (re-import case), fall back to
	// prefix matching over all current tables.
	if candidates == nil {
		tables, err := s.usecases.importer.GetTableNames(ctx)
		if err != nil {
			return fmt.Errorf("failed to get table names for %s: %w", excelPath, err)
		}
		candidates = make(map[string]struct{})
		for _, t := range tables {
			if strings.HasPrefix(t.Name(), exactPrefix) {
				candidates[t.Name()] = struct{}{}
			}
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("no sheets found in Excel file %s", excelPath)
	}

	sanitized := s.usecases.importer.SanitizeForSQL(sheetName)
	var keepTable string
	for name := range candidates {
		// Extract sheet part by stripping the known prefix.
		// Only consider tables that actually start with this file's prefix.
		if !strings.HasPrefix(name, exactPrefix) {
			continue
		}
		sheetPart := strings.TrimPrefix(name, exactPrefix)
		if sheetPart == sanitized {
			keepTable = name
			break
		}
	}

	if keepTable == "" {
		for name := range candidates {
			if !strings.HasPrefix(name, exactPrefix) {
				continue
			}
			dropSQL := "DROP TABLE IF EXISTS " + s.usecases.importer.QuoteIdentifier(name)
			if _, err := s.usecases.query.Exec(ctx, dropSQL); err != nil {
				return fmt.Errorf("failed to drop sheet table %s: %w", name, err)
			}
		}
		return fmt.Errorf("sheet %q not found in Excel file %s: %w", sheetName, excelPath, errSheetNotFound)
	}

	for name := range candidates {
		if name == keepTable || !strings.HasPrefix(name, exactPrefix) {
			continue
		}
		dropSQL := "DROP TABLE IF EXISTS " + s.usecases.importer.QuoteIdentifier(name)
		if _, err := s.usecases.query.Exec(ctx, dropSQL); err != nil {
			return fmt.Errorf("failed to drop sheet table %s: %w", name, err)
		}
	}
	return nil
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
	// #316.

	// Check for dangerous patterns that could indicate path traversal attacks.
	// URL-encoded sequences (..%2f, ..%5c) are intentionally NOT matched: the
	// filesystem never URL-decodes a path, so those bytes only ever appear in a
	// legitimate literal filename, and matching them rejected real files. Ref
	// #317.
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

	// Prevent access to system directories on Unix-like systems
	absPath, err := filepath.Abs(cleanPath)
	if err == nil && !isAllowedPseudoFile(absPath) { // Only check if we can resolve the absolute path
		systemDirs := []string{"/etc", "/proc", "/sys", "/dev", "/boot"}
		for _, sysDir := range systemDirs {
			if absPath == sysDir || strings.HasPrefix(absPath, sysDir+"/") {
				return "", fmt.Errorf("access to system directory not allowed: %s", path)
			}
		}
	}

	return cleanPath, nil
}

// isAllowedPseudoFile reports whether an absolute path is a standard Unix
// pseudo-file that holds legitimate, user-controlled input even though it lives
// under a system directory. These are exempt from the system-directory guard:
//   - /dev/shm/*            tmpfs for user data (Ref #427)
//   - /dev/fd/*             open file descriptors, process substitution (Ref #428)
//   - /dev/stdin, /dev/stdout, /dev/stderr  standard stream pseudo-files (Ref #461)
//   - /proc/<pid|self>/fd/* the Linux fd aliases behind many fd-based workflows (Ref #462)
func isAllowedPseudoFile(absPath string) bool {
	switch absPath {
	case "/dev/stdin", "/dev/stdout", "/dev/stderr":
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

// extractSheetNameFromArgs extracts the sheet name from command line arguments.
// It accepts both the joined form "--sheet=SHEET_NAME" and the separated form
// "--sheet SHEET_NAME", returning the first match. The separated form lets the
// value carry spaces once splitArgs has tokenized the quoted input. If no sheet
// name is found, it returns an empty string.
func extractSheetNameFromArgs(argv []string) string {
	for i := range argv {
		arg := argv[i]
		if value, found := strings.CutPrefix(arg, sheetFlagAssign); found {
			return value
		}
		if arg == sheetFlag && i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
			return argv[i+1]
		}
	}
	return ""
}

// helperSheetExplicitlyEmpty reports whether a .import argv contains a --sheet
// flag given an explicit empty value, in either the separated form (`--sheet ""`)
// or the joined form (`--sheet=`). An empty value is the "no sheet" sentinel, so
// accepting it would silently import every sheet; the caller rejects it instead.
// A bare trailing --sheet with no value at all is left to the main loop, which
// reports it as a missing value. Ref #354, #355.
func helperSheetExplicitlyEmpty(argv []string) bool {
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == sheetFlag {
			// Separated form: a following token that is not another flag is the
			// value. An empty value here is the explicit-empty mistake.
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
				if strings.TrimSpace(argv[i+1]) == "" {
					return true
				}
				i++
			}
			continue
		}
		if value, found := strings.CutPrefix(a, sheetFlagAssign); found {
			if strings.TrimSpace(value) == "" {
				return true
			}
		}
	}
	return false
}

// printImportUsage print import command usage.
func printImportUsage() {
	fmt.Fprintln(config.Stdout, "[Usage]")
	fmt.Fprintln(config.Stdout, "  .import FILE_PATH(S)|DIRECTORY_PATH(S) [--sheet NAME | --sheet=NAME]")
	fmt.Fprintln(config.Stdout, "")
	fmt.Fprintln(config.Stdout, "  - Quote arguments that contain spaces: .import \"my data.csv\" or --sheet \"Q1 Sales\"")
	fmt.Fprintln(config.Stdout, "")
	fmt.Fprintln(config.Stdout, "  - Supported file format: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed")
	fmt.Fprintln(config.Stdout, "  - Compression (csv/tsv/ltsv/json/jsonl/parquet/xlsx only): .gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4")
	fmt.Fprintln(config.Stdout, "  - Files and directories can be mixed in arguments")
	fmt.Fprintln(config.Stdout, "  - Directories are automatically detected and all supported files are imported")
	fmt.Fprintln(config.Stdout, "  - If import multiple files/directories, separate them with spaces")
	fmt.Fprintln(config.Stdout, "  - For Excel files, all sheets are imported as separate tables (enables cross-sheet JOINs)")
	fmt.Fprintln(config.Stdout, "  - Use --sheet to import only a specific sheet from Excel files (works with files and directories)")
	fmt.Fprintln(config.Stdout, "  - JSON/JSONL data is stored in a 'data' column; use json_extract() to query fields")
}
