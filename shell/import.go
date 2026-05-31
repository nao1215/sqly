package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

const (
	// maxDirectoryDepth is the maximum directory depth to prevent deep traversal.
	maxDirectoryDepth = 10
	// sheetFlag is the .import flag selecting a single Excel sheet. It accepts
	// both the separated form "--sheet NAME" and the joined form "--sheet=NAME".
	sheetFlag = "--sheet"
	// sheetFlagAssign is the joined form prefix of sheetFlag.
	sheetFlagAssign = sheetFlag + "="
)

// importCommand imports files into the in-memory database.
// Each file/directory is loaded individually so that same-name tables from
// different directories are overwritten (last-wins) rather than failing,
// and --sheet filtering is scoped to the correct Excel file.
func (c CommandList) importCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		printImportUsage()
		return nil
	}

	sheetName := s.argument.SheetName
	if sheetName == "" {
		sheetName = extractSheetNameFromArgs(argv)
	}

	var errorMessages []string
	var successCount int

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
			imported, err := s.importDirectory(ctx, cleanPath, path, sheetName)
			if err != nil {
				errorMessages = append(errorMessages, err.Error())
				continue
			}
			if imported {
				successCount++
			}
		} else {
			if err := s.importFile(ctx, cleanPath, path, sheetName); err != nil {
				errorMessages = append(errorMessages, err.Error())
				continue
			}
			successCount++
		}
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
	}

	return nil
}

// importDirectory loads all supported files from a directory into the database.
// Returns true if at least one new table was actually imported, false if nothing
// was imported (empty directory or no supported files).
// When sheetName is specified, --sheet filtering is applied per-Excel-file
// by walking the directory and filtering each Excel file individually.
func (s *Shell) importDirectory(ctx context.Context, cleanPath, displayPath, sheetName string) (bool, error) {
	tablesBefore, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get table names before importing directory %s: %w", displayPath, err)
	}
	existingTables := tableNameSet(tablesBefore)

	if err := s.usecases.importer.LoadFiles(ctx, cleanPath); err != nil {
		return false, fmt.Errorf("failed to import files from directory %s: %w", displayPath, err)
	}

	tablesAfter, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get table names after importing directory %s: %w", displayPath, err)
	}
	newTableNames := diffTableNames(tablesAfter, existingTables)

	if len(newTableNames) == 0 {
		fmt.Fprintf(s.importStatusWriter(), "No supported files found in directory %s\n", displayPath)
		return false, nil
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
					return err
				}
			}
			return nil
		})
		if err != nil {
			return false, fmt.Errorf("failed to walk directory %s for sheet filtering: %w", displayPath, err)
		}
	}

	// Recompute remaining tables after potential sheet filtering may have dropped some.
	tablesNow, err := s.usecases.importer.GetTableNames(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get table names after sheet filtering for %s: %w", displayPath, err)
	}
	remainingNames := diffTableNames(tablesNow, existingTables)

	if s.argument.InspectFlag {
		s.recordInspectSources(remainingNames, displayPath)
	}

	fmt.Fprintf(s.importStatusWriter(), "Successfully imported %d table(s) from directory %s: %v\n", len(remainingNames), displayPath, remainingNames)
	return true, nil
}

// importFile loads a single file into the database, applying --sheet filtering for Excel.
func (s *Shell) importFile(ctx context.Context, cleanPath, displayPath, sheetName string) error {
	if !s.usecases.importer.IsSupportedFile(cleanPath) {
		return fmt.Errorf("unsupported file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed)", filepath.Base(cleanPath))
	}

	// Capture which tables this file creates so --inspect can map them to their
	// source. The before/after diff is only computed when inspecting, so normal
	// imports keep their single-load cost.
	var existingTables map[string]struct{}
	if s.argument.InspectFlag {
		before, err := s.usecases.importer.GetTableNames(ctx)
		if err != nil {
			return fmt.Errorf("failed to get table names before importing %s: %w", displayPath, err)
		}
		existingTables = tableNameSet(before)
	}

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

	if s.argument.InspectFlag {
		after, err := s.usecases.importer.GetTableNames(ctx)
		if err != nil {
			return fmt.Errorf("failed to get table names after importing %s: %w", displayPath, err)
		}
		s.recordInspectSources(diffTableNames(after, existingTables), displayPath)
	}

	return nil
}

// recordInspectSources remembers which source path produced each table name so
// the --inspect report can show the source-to-table mapping.
func (s *Shell) recordInspectSources(tableNames []string, source string) {
	if s.inspectSources == nil {
		s.inspectSources = make(map[string]string)
	}
	for _, name := range tableNames {
		s.inspectSources[name] = source
	}
}

// importStatusWriter selects where import progress messages go. Under --inspect,
// stdout must carry only the JSON report, so progress is sent to stderr instead.
func (s *Shell) importStatusWriter() io.Writer {
	if s.argument.InspectFlag {
		return config.Stderr
	}
	return config.Stdout
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
		return fmt.Errorf("sheet %q not found in Excel file %s", sheetName, excelPath)
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

	// Check directory depth to prevent deep traversal
	pathParts := strings.Split(cleanPath, string(filepath.Separator))
	if len(pathParts) > maxDirectoryDepth {
		return "", fmt.Errorf("path exceeds maximum directory depth of %d: %s", maxDirectoryDepth, path)
	}

	// Check for dangerous patterns that could indicate path traversal attacks
	// These are the most common patterns used in path traversal attacks
	dangerousPatterns := []string{
		"../../../",    // Multiple levels up
		"..\\..\\..\\", // Windows path traversal
		"....//",       // Double encoding attempts
		"..%2f",        // URL encoded path traversal
		"..%5c",        // URL encoded backslash
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
