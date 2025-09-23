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
	// Security limits for directory traversal protection
	maxDirectoryDepth    = 10   // Maximum directory depth to prevent deep traversal
	maxFilesPerDirectory = 1000 // Maximum files per directory to prevent resource exhaustion
)

// importCommand import csv into DB

func (c CommandList) importCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		printImportUsage()
		return nil
	}

	var errorMessages []string
	var successCount int

	for _, path := range argv {
		// Skip non-path arguments (like --sheet=...)
		if strings.HasPrefix(path, "--sheet=") {
			continue
		}

		// Validate and clean the path to prevent directory traversal
		cleanPath, err := validatePath(path)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("invalid path %s: %v", path, err))
			continue
		}

		// Check if path is a directory
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
			// Get existing table names before import to determine what was newly imported
			tablesBefore, err := s.usecases.filesql.GetTableNames(ctx)
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to get existing table names before importing from directory %s: %v", path, err))
				continue
			}

			// Create a set of existing table names for efficient lookup
			existingTables := make(map[string]struct{}, len(tablesBefore))
			for _, table := range tablesBefore {
				existingTables[table.Name()] = struct{}{}
			}

			// Use filesql to import all files from the directory
			if err := s.usecases.filesql.LoadFiles(ctx, cleanPath); err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to import files from directory %s: %v", path, err))
				continue
			}

			// Get and display newly imported tables
			tablesAfter, err := s.usecases.filesql.GetTableNames(ctx)
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to get table names after importing from directory %s: %v", path, err))
				continue
			}

			// Find newly imported tables by comparing with existing set
			var newTableNames []string
			for _, table := range tablesAfter {
				if _, exists := existingTables[table.Name()]; !exists {
					newTableNames = append(newTableNames, table.Name())
				}
			}

			if len(newTableNames) == 0 {
				fmt.Fprintf(config.Stdout, "No supported files found in directory %s (supported: csv, tsv, ltsv, xlsx with .gz/.bz2/.xz/.zst compression)\n", path)
			} else {
				fmt.Fprintf(config.Stdout, "Successfully imported %d tables from directory %s: %v\n", len(newTableNames), path, newTableNames)
				successCount++
			}
		} else {
			// Handle individual file import (existing logic)
			var table *model.Table
			var sheetName string

			switch {
			case isCSV(cleanPath):
				table, err = s.usecases.csv.List(cleanPath)
				if err != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("failed to import CSV file %s: %v", path, err))
					continue
				}
			case isTSV(cleanPath):
				table, err = s.usecases.tsv.List(cleanPath)
				if err != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("failed to import TSV file %s: %v", path, err))
					continue
				}
			case isLTSV(cleanPath):
				table, err = s.usecases.ltsv.List(cleanPath)
				if err != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("failed to import LTSV file %s: %v", path, err))
					continue
				}
			case isXLAM(cleanPath) || isXLSM(cleanPath) || isXLSX(cleanPath) || isXLTM(cleanPath) || isXLTX(cleanPath):
				sheetName = s.argument.SheetName
				if sheetName == "" {
					sheetName = extractSheetNameFromArgs(argv)
					if sheetName == "" {
						errorMessages = append(errorMessages, fmt.Sprintf("sheet name is required for Excel file %s (use --sheet=SHEET_NAME)", path))
						continue
					}
				}
				table, err = s.usecases.excel.List(cleanPath, sheetName)
				if err != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("failed to import Excel file %s (sheet: %s): %v", path, sheetName, err))
					continue
				}
			default:
				errorMessages = append(errorMessages, fmt.Sprintf("unsupported file format: %s (supported: csv, tsv, ltsv, xlsx)", getFileTypeFromPath(cleanPath)))
				continue
			}

			if err := s.usecases.sqlite3.CreateTable(ctx, table); err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to create table for %s: %v", path, err))
				continue
			}
			if err := s.usecases.sqlite3.Insert(ctx, table); err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to insert data from %s: %v", path, err))
				continue
			}
			successCount++
		}
	}

	// Report results
	if len(errorMessages) > 0 {
		if successCount > 0 {
			fmt.Fprintf(config.Stdout, "\nImport completed with %d successful imports and %d errors:\n", successCount, len(errorMessages))
		} else {
			fmt.Fprintf(config.Stdout, "\nImport failed with %d errors:\n", len(errorMessages))
		}
		for _, errMsg := range errorMessages {
			fmt.Fprintf(config.Stdout, "  - %s\n", errMsg)
		}
		// Return error only if nothing was successfully imported
		if successCount == 0 {
			return errors.New("all import attempts failed")
		}
	}

	return nil
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
	fmt.Fprintln(config.Stdout, "  - Supported file format: csv, tsv, ltsv, xlam, xlsm, xlsx, xltm, xltx")
	fmt.Fprintln(config.Stdout, "  - Compression: .gz, .bz2, .xz, .zst (automatically detected)")
	fmt.Fprintln(config.Stdout, "  - Files and directories can be mixed in arguments")
	fmt.Fprintln(config.Stdout, "  - Directories are automatically detected and all supported files are imported")
	fmt.Fprintln(config.Stdout, "  - If import multiple files/directories, separate them with spaces")
	fmt.Fprintln(config.Stdout, "  - Does not support importing multiple excel sheets at once")
	fmt.Fprintln(config.Stdout, "  - If import an Excel file, specify the sheet name with --sheet")
}
