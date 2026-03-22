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
// When sheetName is specified, --sheet filtering is applied per-Excel-file
// by walking the directory and filtering each Excel file individually.
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

	// Apply --sheet filtering per-Excel-file within the directory.
	// Walk the directory to find Excel files and filter each one individually,
	// so that non-Excel tables (CSV, JSON, etc.) are never affected, and
	// multiple Excel files each keep their own matching sheet.
	// Pass the newTableNames as candidates so only freshly-imported tables
	// are considered, preventing prefix collision with pre-existing tables.
	if sheetName != "" {
		newSet := make(map[string]struct{}, len(newTableNames))
		for _, n := range newTableNames {
			newSet[n] = struct{}{}
		}
		// Walk the directory recursively to match filesql's recursive import behavior.
		// filesql uses filepath.WalkDir when importing directories, so sheet filtering
		// must also traverse subdirectories to find all Excel files.
		err := filepath.WalkDir(cleanPath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if s.usecases.sqlite3.IsExcelFile(path) {
				if err := s.filterExcelSheets(ctx, path, sheetName, newSet); err != nil {
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
	tablesNow, err := s.usecases.sqlite3.GetTableNames(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get table names after sheet filtering for %s: %w", displayPath, err)
	}
	remainingNames := diffTableNames(tablesNow, existingTables)

	fmt.Fprintf(config.Stdout, "Successfully imported %d table(s) from directory %s: %v\n", len(remainingNames), displayPath, remainingNames)
	return true, nil
}

// importFile loads a single file into the database, applying --sheet filtering for Excel.
func (s *Shell) importFile(ctx context.Context, cleanPath, displayPath, sheetName string) error {
	if !s.usecases.sqlite3.IsSupportedFile(cleanPath) {
		return fmt.Errorf("unsupported file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed)", filepath.Base(cleanPath))
	}

	if err := s.usecases.sqlite3.LoadFiles(ctx, cleanPath); err != nil {
		return fmt.Errorf("failed to import file %s: %w", displayPath, err)
	}

	// Apply --sheet filtering only to Excel files.
	// Pass nil candidates so filterExcelSheets falls back to prefix matching
	// over all current tables. This handles both first-import and re-import:
	// for a single-file import there's no ambiguity about which file owns
	// the prefix, so prefix matching is safe here.
	if s.usecases.sqlite3.IsExcelFile(cleanPath) && sheetName != "" {
		if err := s.filterExcelSheets(ctx, cleanPath, sheetName, nil); err != nil {
			return err
		}
	}

	return nil
}

// filterExcelSheets keeps only the requested sheet from a specific Excel file,
// operating on the given candidate set of table names. If candidates is nil,
// we build the candidate set from all tables matching the file's prefix (used
// for re-import where the diff is empty).
//
// Candidates isolate the filtering to tables owned by the current import,
// preventing prefix collisions: e.g. "sales_" won't accidentally match
// tables from "sales_q1.xlsx" if those tables aren't in the candidate set.
func (s *Shell) filterExcelSheets(ctx context.Context, excelPath string, sheetName string, candidates map[string]struct{}) error {
	exactPrefix := s.usecases.sqlite3.GetTableNameFromFilePath(excelPath) + "_"

	// If no candidate set was provided (re-import case), fall back to
	// prefix matching over all current tables.
	if candidates == nil {
		tables, err := s.usecases.sqlite3.GetTableNames(ctx)
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

	sanitized := s.usecases.sqlite3.SanitizeForSQL(sheetName)
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
			dropSQL := "DROP TABLE IF EXISTS " + s.usecases.sqlite3.QuoteIdentifier(name)
			if _, err := s.usecases.sqlite3.Exec(ctx, dropSQL); err != nil {
				return fmt.Errorf("failed to drop sheet table %s: %w", name, err)
			}
		}
		return fmt.Errorf("sheet %q not found in Excel file %s", sheetName, excelPath)
	}

	for name := range candidates {
		if name == keepTable || !strings.HasPrefix(name, exactPrefix) {
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
	fmt.Fprintln(config.Stdout, "  - Supported file format: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed")
	fmt.Fprintln(config.Stdout, "  - Compression (csv/tsv/ltsv/json/jsonl/parquet/xlsx only): .gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4")
	fmt.Fprintln(config.Stdout, "  - Files and directories can be mixed in arguments")
	fmt.Fprintln(config.Stdout, "  - Directories are automatically detected and all supported files are imported")
	fmt.Fprintln(config.Stdout, "  - If import multiple files/directories, separate them with spaces")
	fmt.Fprintln(config.Stdout, "  - For Excel files, all sheets are imported as separate tables (enables cross-sheet JOINs)")
	fmt.Fprintln(config.Stdout, "  - Use --sheet to import only a specific sheet from Excel files (works with files and directories)")
	fmt.Fprintln(config.Stdout, "  - JSON/JSONL data is stored in a 'data' column; use json_extract() to query fields")
}
