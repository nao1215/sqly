package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/config"
)

const (
	// maxDirectoryDepth is the maximum directory depth to prevent deep traversal.
	maxDirectoryDepth = 10
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
				fmt.Fprintf(config.Stdout, "No supported files found in directory %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx)\n", path)
			} else {
				fmt.Fprintf(config.Stdout, "Successfully imported %d tables from directory %s: %v\n", len(newTableNames), path, newTableNames)
				successCount++
			}
		} else {
			// Use filesql.LoadFiles for all file types (CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel).
			// This avoids double processing and automatically supports all formats that filesql handles.
			if !isSupportedFile(cleanPath) {
				errorMessages = append(errorMessages, fmt.Sprintf("unsupported file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx and compressed variants)", filepath.Base(cleanPath)))
				continue
			}

			if err := s.usecases.filesql.LoadFiles(ctx, cleanPath); err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("failed to import file %s: %v", path, err))
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
	fmt.Fprintln(config.Stdout, "  - Supported file format: csv, tsv, ltsv, json, jsonl, parquet, xlsx")
	fmt.Fprintln(config.Stdout, "  - Compression: .gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4 (automatically detected)")
	fmt.Fprintln(config.Stdout, "  - Files and directories can be mixed in arguments")
	fmt.Fprintln(config.Stdout, "  - Directories are automatically detected and all supported files are imported")
	fmt.Fprintln(config.Stdout, "  - If import multiple files/directories, separate them with spaces")
	fmt.Fprintln(config.Stdout, "  - For Excel files, --sheet selects a specific sheet (default: first sheet)")
	fmt.Fprintln(config.Stdout, "  - JSON/JSONL data is stored in a 'data' column; use json_extract() to query fields")
}
