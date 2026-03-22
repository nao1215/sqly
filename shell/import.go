package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

const (
	// maxDirectoryDepth is the maximum directory depth to prevent deep traversal.
	maxDirectoryDepth = 10
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

	for _, path := range argv {
		if strings.HasPrefix(path, "--sheet=") {
			continue
		}

		cleanPath, err := validatePath(path)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("invalid path %s: %v", path, err))
			continue
		}

		info, err := os.Stat(cleanPath)
		if err != nil {
			if os.IsNotExist(err) {
				errorMessages = append(errorMessages, "path does not exist: "+path)
			} else if os.IsPermission(err) {
				errorMessages = append(errorMessages, "permission denied accessing path: "+path)
			} else {
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
		if successCount > 0 {
			fmt.Fprintf(config.Stdout, "\nImport completed with %d successful import(s) and %d error(s):\n", successCount, len(errorMessages))
		} else {
			fmt.Fprintf(config.Stdout, "\nImport failed with %d error(s):\n", len(errorMessages))
		}
		for _, errMsg := range errorMessages {
			fmt.Fprintf(config.Stdout, "  - %s\n", errMsg)
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
// When sheetName is specified, Excel files within the directory are filtered
// to keep only the requested sheet.
func (s *Shell) importDirectory(ctx context.Context, cleanPath, displayPath, sheetName string) (bool, error) {
	tablesBefore, err := s.usecases.sqlite3.GetTableNames(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get table names before importing directory %s: %w", displayPath, err)
	}
	existingTables := tableNameSet(tablesBefore)

	if err := s.usecases.sqlite3.LoadFiles(ctx, cleanPath); err != nil {
		return false, fmt.Errorf("failed to import files from directory %s: %w", displayPath, err)
	}

	tablesAfter, err := s.usecases.sqlite3.GetTableNames(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get table names after importing directory %s: %w", displayPath, err)
	}
	newTableNames := diffTableNames(tablesAfter, existingTables)

	if len(newTableNames) == 0 {
		fmt.Fprintf(config.Stdout, "No supported files found in directory %s\n", displayPath)
		return false, nil
	}

	// Apply --sheet filtering to newly imported Excel tables in this directory
	if sheetName != "" {
		newSet := make(map[string]struct{}, len(newTableNames))
		for _, n := range newTableNames {
			newSet[n] = struct{}{}
		}
		if err := s.filterExcelSheetsInSet(ctx, newSet, sheetName); err != nil {
			return false, err
		}
	}

	fmt.Fprintf(config.Stdout, "Successfully imported %d table(s) from directory %s: %v\n", len(newTableNames), displayPath, newTableNames)
	return true, nil
}

// importFile loads a single file into the database, applying --sheet filtering for Excel.
func (s *Shell) importFile(ctx context.Context, cleanPath, displayPath, sheetName string) error {
	if !s.usecases.sqlite3.IsSupportedFile(cleanPath) {
		return fmt.Errorf("unsupported file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx, ach, fed and compressed variants)", filepath.Base(cleanPath))
	}

	// Record tables before import so we can identify exactly which tables were added
	tablesBefore, err := s.usecases.sqlite3.GetTableNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to get table names before importing %s: %w", displayPath, err)
	}
	existingTables := tableNameSet(tablesBefore)

	if err := s.usecases.sqlite3.LoadFiles(ctx, cleanPath); err != nil {
		return fmt.Errorf("failed to import file %s: %w", displayPath, err)
	}

	// Apply --sheet filtering only to Excel files, scoped to tables added by this import
	if s.usecases.sqlite3.IsExcelFile(cleanPath) && sheetName != "" {
		tablesAfter, err := s.usecases.sqlite3.GetTableNames(ctx)
		if err != nil {
			return fmt.Errorf("failed to get table names after importing %s: %w", displayPath, err)
		}
		newNames := diffTableNames(tablesAfter, existingTables)
		newSet := make(map[string]struct{}, len(newNames))
		for _, n := range newNames {
			newSet[n] = struct{}{}
		}
		if err := s.filterExcelSheetsInSet(ctx, newSet, sheetName); err != nil {
			return err
		}
	}

	return nil
}

// filterExcelSheetsInSet keeps only the table matching sheetName from the given
// set of newly-imported table names, dropping the rest. This avoids the prefix
// collision problem: instead of guessing which tables belong to a file via prefix
// matching, we operate only on the exact set of tables added by the most recent import.
func (s *Shell) filterExcelSheetsInSet(ctx context.Context, newTables map[string]struct{}, sheetName string) error {
	if len(newTables) == 0 {
		return nil
	}

	sanitized := s.usecases.sqlite3.SanitizeForSQL(sheetName)

	// Find the table whose sheet-part suffix matches the requested sheet name
	var keepTable string
	for name := range newTables {
		// Table names from Excel are formatted as "filename_sheetname".
		// Extract the sheet part by finding the last occurrence of the sanitized
		// sheet name as a suffix after an underscore.
		if idx := strings.Index(name, "_"); idx >= 0 {
			sheetPart := name[idx+1:]
			if sheetPart == sanitized {
				keepTable = name
				break
			}
		}
		// Also handle case where table name equals the sheet name directly
		if name == sanitized {
			keepTable = name
			break
		}
	}

	if keepTable == "" {
		// Drop all newly imported tables since the requested sheet was not found
		for name := range newTables {
			dropSQL := "DROP TABLE IF EXISTS " + s.usecases.sqlite3.QuoteIdentifier(name)
			if _, err := s.usecases.sqlite3.Exec(ctx, dropSQL); err != nil {
				return fmt.Errorf("failed to drop sheet table %s: %w", name, err)
			}
		}
		return fmt.Errorf("sheet %q not found in imported Excel tables", sheetName)
	}

	// Drop every newly imported table except the one we want to keep
	for name := range newTables {
		if name == keepTable {
			continue
		}
		dropSQL := "DROP TABLE IF EXISTS " + s.usecases.sqlite3.QuoteIdentifier(name)
		if _, err := s.usecases.sqlite3.Exec(ctx, dropSQL); err != nil {
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
// It looks for arguments in the format "--sheet=SHEET_NAME" and returns the sheet name.
// If no sheet name is found, it returns an empty string.
func extractSheetNameFromArgs(argv []string) string {
	for _, arg := range argv {
		if value, found := strings.CutPrefix(arg, "--sheet="); found {
			return value
		}
	}
	return ""
}

// printImportUsage print import command usage.
func printImportUsage() {
	fmt.Fprintln(config.Stdout, "[Usage]")
	fmt.Fprintln(config.Stdout, "  .import FILE_PATH(S)|DIRECTORY_PATH(S) [--sheet=SHEET_NAME]")
	fmt.Fprintln(config.Stdout, "")
	fmt.Fprintln(config.Stdout, "  - Supported file format: csv, tsv, ltsv, json, jsonl, parquet, xlsx, ach, fed")
	fmt.Fprintln(config.Stdout, "  - Compression: .gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4 (automatically detected)")
	fmt.Fprintln(config.Stdout, "  - Files and directories can be mixed in arguments")
	fmt.Fprintln(config.Stdout, "  - Directories are automatically detected and all supported files are imported")
	fmt.Fprintln(config.Stdout, "  - If import multiple files/directories, separate them with spaces")
	fmt.Fprintln(config.Stdout, "  - For Excel files, all sheets are imported as separate tables (enables cross-sheet JOINs)")
	fmt.Fprintln(config.Stdout, "  - Use --sheet to import only a specific sheet from Excel files (works with files and directories)")
	fmt.Fprintln(config.Stdout, "  - JSON/JSONL data is stored in a 'data' column; use json_extract() to query fields")
}
