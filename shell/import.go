package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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

// importDirectory loads all supported files from a directory into the database.
// Returns imported=true if at least one new table was actually imported, plus
// the number of workbooks skipped because they lacked the requested --sheet.
// When sheetName is specified, --sheet filtering is applied per-Excel-file by
// walking the directory and filtering each Excel file individually; in a
// multi-workbook import a workbook missing the sheet is skipped, not fatal. Ref
// #378.
func (s *Shell) importDirectory(ctx context.Context, cleanPath, displayPath, sheetName string, multiWorkbook bool) (imported bool, skipped int, err error) {
	tablesBefore, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get table names before importing directory %s: %w", displayPath, err)
	}
	existingTables := tableNameSet(tablesBefore)

	if err := s.usecases.importer.LoadFiles(ctx, cleanPath); err != nil {
		return false, 0, fmt.Errorf("failed to import files from directory %s: %w", displayPath, err)
	}

	tablesAfter, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get table names after importing directory %s: %w", displayPath, err)
	}
	newTableNames := diffTableNames(tablesAfter, existingTables)

	if len(newTableNames) == 0 {
		fmt.Fprintf(s.importStatusWriter(), "No supported files found in directory %s\n", displayPath)
		return false, 0, nil
	}

	// Apply --sheet filtering per-Excel-file within the directory.
	// For each Excel file, filterExcelSheets builds candidates from all
	// current tables matching the file's prefix (nil candidates mode).
	// This correctly handles both first-import and re-import (overwrite).
	if sheetName != "" {
		err := filepath.WalkDir(cleanPath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if s.usecases.importer.IsExcelFile(path) {
				if err := s.filterExcelSheets(ctx, path, sheetName, nil); err != nil {
					// A workbook in the tree that lacks the sheet is skipped rather
					// than aborting the whole directory import, so matching workbooks
					// still load. Ref #378. filterExcelSheets has already dropped this
					// workbook's tables before returning the not-found error.
					if multiWorkbook && errors.Is(err, errSheetNotFound) {
						fmt.Fprintf(s.importStatusWriter(), "Skipped %s: %v\n", path, err)
						skipped++
						return nil
					}
					return err
				}
			}
			return nil
		})
		if err != nil {
			return false, skipped, fmt.Errorf("failed to walk directory %s for sheet filtering: %w", displayPath, err)
		}
	}

	// Recompute remaining tables after potential sheet filtering may have dropped some.
	tablesNow, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return false, skipped, fmt.Errorf("failed to get table names after sheet filtering for %s: %w", displayPath, err)
	}
	remainingNames := diffTableNames(tablesNow, existingTables)

	// Record each table's real source file when it can be matched to a file in
	// the directory, so --inspect reports per-file provenance instead of the
	// directory path (#326). Tables that cannot be matched (filesql sanitized the
	// name, or the candidate is ambiguous) fall back to the directory path. Every
	// table is marked as a directory import so write-back still rejects it.
	fileSources := s.directoryTableFileSources(cleanPath)
	for _, name := range remainingNames {
		source := displayPath
		if file, ok := fileSources[name]; ok {
			source = file
		}
		s.recordTableSources([]string{name}, source)
		s.markDirImported(name)
	}

	fmt.Fprintf(s.importStatusWriter(), "Successfully imported %d table(s) from directory %s: %v\n", len(remainingNames), displayPath, remainingNames)
	return len(remainingNames) > 0, skipped, nil
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

// directoryTableFileSources walks dir and maps a candidate table name (the file
// base name with compression and format extensions stripped) to its file path.
// A name produced by more than one file is omitted as ambiguous, so the caller
// falls back to the directory path rather than guessing.
func (s *Shell) directoryTableFileSources(dir string) map[string]string {
	candidates := map[string][]string{}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // skip unreadable entries; import reports real errors
		}
		if !s.usecases.importer.IsSupportedFile(path) {
			return nil
		}
		if name := baseTableName(path); name != "" {
			candidates[name] = append(candidates[name], path)
		}
		return nil
	})
	result := make(map[string]string, len(candidates))
	for name, paths := range candidates {
		if len(paths) == 1 {
			result[name] = paths[0]
		}
	}
	return result
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

	// A successful import that produced no new table means this file's table
	// name collided with one already imported in this session (for example
	// "a-b.csv" and "a_b.csv" both sanitize to "a_b"). filesql overwrote the
	// earlier table in memory, which would leave the first file's source mapped
	// to the second file's rows. Fail instead of silently overwriting. Ref #286.
	//
	// A re-import of the same source path is harmless (last-wins), so only reject
	// when this file's path is not already a recorded source: that means a
	// different input produced the same sanitized name.
	newNames := diffTableNames(after, existingTables)
	if len(newNames) == 0 {
		if s.isRecordedSource(displayPath) {
			return nil
		}
		return fmt.Errorf("table-name collision: %s sanitizes to a table name already imported from another input; rename the file to disambiguate", displayPath)
	}
	s.recordTableSources(newNames, displayPath)

	return nil
}

// isRecordedSource reports whether path (resolved to an absolute path, matching
// recordTableSources) is already the source of an imported table. It lets a
// re-import of the same file be treated as a harmless last-wins overwrite rather
// than a table-name collision.
func (s *Shell) isRecordedSource(path string) bool {
	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}
	for _, src := range s.tableSources {
		if src == abs {
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
	if err == nil { // Only check if we can resolve the absolute path
		systemDirs := []string{"/etc", "/proc", "/sys", "/dev", "/boot"}
		for _, sysDir := range systemDirs {
			if strings.HasPrefix(absPath, sysDir) {
				return "", fmt.Errorf("access to system directory not allowed: %s", path)
			}
		}
	}

	return cleanPath, nil
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
