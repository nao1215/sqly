// Package filesql provides adapters for integrating nao1215/filesql package with sqly.
package filesql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
)

var validTableName = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

const (
	opQuery      = "query"
	opRows       = "rows"
	opExec       = "exec"
	opGetTables  = "get_tables"
	opGetHeader  = "get_header"
	opScanTable  = "scan_table"
	opScanHeader = "scan_header"

	errDatabaseNotInit = "database not initialized"
	defaultSheetName   = "sheet"
)

// FileSQLAdapter wraps filesql functionality to integrate with sqly architecture
//
//nolint:revive // Name maintained for compatibility with existing wire dependencies and external usage
type FileSQLAdapter struct {
	sharedDB *sql.DB // The main sqly application database
}

// NewFileSQLAdapter creates a new adapter for filesql integration
func NewFileSQLAdapter(sharedDB *sql.DB) *FileSQLAdapter {
	return &FileSQLAdapter{
		sharedDB: sharedDB,
	}
}

// LoadFiles loads multiple files into the shared database using filesql
func (f *FileSQLAdapter) LoadFiles(ctx context.Context, filePaths ...string) error {
	if len(filePaths) == 0 {
		return nil
	}

	if f.sharedDB == nil {
		return errors.New("shared database is not initialized")
	}

	// Identify ACH/Fedwire base names from input paths (not table names) so we
	// can clean up the filesql global registry. We must do this before OpenContext
	// and defer the cleanup so it runs even if table copying fails.
	achBaseNames, wireBaseNames := collectRegistryBaseNames(filePaths)

	// Use filesql to load files into temporary database, then copy to shared database
	tmpDB, err := filesql.OpenContext(ctx, filePaths...)
	if err != nil {
		return err
	}
	defer func() { _ = tmpDB.Close() }()

	// Clean up filesql global registries for ACH/Fedwire TableSets.
	// filesql.OpenContext registers TableSets in global maps for DumpACH/DumpFedWire,
	// but sqly copies data to its own shared DB and does not use round-trip export.
	// Using defer ensures cleanup even if table copying fails partway through.
	defer func() {
		for _, baseName := range achBaseNames {
			filesql.UnregisterACHTableSet(baseName)
		}
		for _, baseName := range wireBaseNames {
			filesql.UnregisterWireTableSet(baseName)
		}
	}()

	// Get actual table names from the temporary database created by filesql
	rows, err := tmpDB.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("failed to get table names from filesql database: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating table names: %w", err)
	}

	// Copy tables from temporary filesql database to shared database
	for _, tableName := range tableNames {
		if err := f.copyTableToSharedDB(ctx, tmpDB, tableName); err != nil {
			return fmt.Errorf("failed to copy table %s: %w", tableName, err)
		}
	}

	return nil
}

// LoadFile loads a single file into the database
func (f *FileSQLAdapter) LoadFile(ctx context.Context, filePath string) error {
	return f.LoadFiles(ctx, filePath)
}

// collectRegistryBaseNames scans the input file paths and returns the sanitized
// base names for any ACH (.ach) or Fedwire (.fed) files. These base names
// correspond to the keys that filesql.OpenContext registers in its global
// ACH/Fedwire TableSet registries. Directory paths are walked recursively to
// find nested ACH/FED files.
func collectRegistryBaseNames(filePaths []string) (achBaseNames, wireBaseNames []string) {
	seen := make(map[string]bool)
	for _, p := range filePaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			addACHOrWireBaseName(p, seen, &achBaseNames, &wireBaseNames)
			continue
		}
		// Walk directory to find nested ACH/FED files.
		// Errors from unreadable entries are skipped.
		_ = filepath.WalkDir(p, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil //nolint:nilerr // skip unreadable entries
			}
			addACHOrWireBaseName(path, seen, &achBaseNames, &wireBaseNames)
			return nil
		})
	}
	return achBaseNames, wireBaseNames
}

// addACHOrWireBaseName checks if path is an ACH or Fedwire file and appends
// its sanitized base name to the appropriate slice, deduplicating per type.
// ACH and Fedwire are tracked independently so that payment.ach and payment.fed
// in the same import both get their respective cleanup entries.
func addACHOrWireBaseName(path string, seen map[string]bool, achOut, wireOut *[]string) {
	lower := strings.ToLower(path)
	baseName := GetTableNameFromFilePath(path)
	if strings.HasSuffix(lower, ".ach") {
		key := baseName + "|ach"
		if !seen[key] {
			seen[key] = true
			*achOut = append(*achOut, baseName)
		}
	} else if strings.HasSuffix(lower, ".fed") {
		key := baseName + "|fed"
		if !seen[key] {
			seen[key] = true
			*wireOut = append(*wireOut, baseName)
		}
	}
}

// copyTableToSharedDB copies a table from source database to shared database using bulk insert optimization
func (f *FileSQLAdapter) copyTableToSharedDB(ctx context.Context, sourceDB *sql.DB, tableName string) error {
	if !validTableName.MatchString(tableName) {
		return fmt.Errorf("invalid table name: %q", tableName)
	}

	// Drop existing table if it exists to avoid conflicts
	// nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	dropSQL := "DROP TABLE IF EXISTS " + QuoteIdentifier(tableName) // #nosec G202
	if _, err := f.sharedDB.ExecContext(ctx, dropSQL); err != nil {
		return fmt.Errorf("failed to drop existing table %s: %w", tableName, err)
	}

	// Use manual approach to preserve filesql's automatic type detection
	return f.copyTableManually(ctx, sourceDB, tableName)
}

// copyTableManually performs manual table copy with proper type preservation
func (f *FileSQLAdapter) copyTableManually(ctx context.Context, sourceDB *sql.DB, tableName string) error {
	if !validTableName.MatchString(tableName) {
		return fmt.Errorf("invalid table name: %q", tableName)
	}
	// Get the original CREATE TABLE statement from filesql database
	var createTableSQL string
	err := sourceDB.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to get table schema for %s: %w", tableName, err)
	}

	// Create table with same schema in shared database (preserves filesql's type detection)
	if _, err := f.sharedDB.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	// Get column names for data copying
	// nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli
	rows, err := sourceDB.QueryContext(ctx, "PRAGMA table_info("+QuoteIdentifier(tableName)+")")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var columns []string
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		// Validate column name is not empty or whitespace-only
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("table %s contains empty or whitespace-only column name at position %d", tableName, cid)
		}
		columns = append(columns, name)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read columns for table %s: %w", tableName, err)
	}

	if len(columns) == 0 {
		return fmt.Errorf("table %s has no columns", tableName)
	}

	// Quote column names to handle reserved keywords and special characters
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = QuoteIdentifier(col)
	}
	quotedTableName := QuoteIdentifier(tableName)

	// Begin transaction for bulk insert optimization
	tx, err := f.sharedDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // Will be no-op if tx.Commit() succeeds

	// Copy data from source to shared database
	// nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	selectSQL := fmt.Sprintf("SELECT %s FROM %s", strings.Join(quotedColumns, ", "), quotedTableName) // #nosec G201
	// nosemgrep: go.lang.security.audit.sqli.gosql-sqli.gosql-sqli
	sourceRows, err := sourceDB.QueryContext(ctx, selectSQL)
	if err != nil {
		return err
	}
	defer func() { _ = sourceRows.Close() }()

	// Prepare insert statement with transaction
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", // #nosec G201
		quotedTableName, strings.Join(quotedColumns, ", "), strings.Join(placeholders, ", "))
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	// Bulk insert with batching for optimal performance
	const batchSize = 1000
	rowCount := 0

	for sourceRows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := sourceRows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row %d: %w", rowCount+1, err)
		}

		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return fmt.Errorf("failed to insert row %d: %w", rowCount+1, err)
		}

		rowCount++

		// Commit batch every batchSize rows for very large datasets
		if rowCount%batchSize == 0 {
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit batch at row %d: %w", rowCount, err)
			}

			// Begin new transaction for next batch
			if tx, err = f.sharedDB.BeginTx(ctx, nil); err != nil {
				return fmt.Errorf("failed to begin new transaction at row %d: %w", rowCount, err)
			}
			defer func() { _ = tx.Rollback() }()

			// Re-prepare statement for new transaction
			if stmt, err = tx.PrepareContext(ctx, insertSQL); err != nil {
				return fmt.Errorf("failed to re-prepare statement at row %d: %w", rowCount, err)
			}
			defer func() { _ = stmt.Close() }()
		}
	}

	// Check for errors during iteration
	if err := sourceRows.Err(); err != nil {
		return fmt.Errorf("error during row iteration: %w", err)
	}

	// Commit final transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit final transaction: %w", err)
	}

	return nil
}

// Query executes SQL query and returns Table model
func (f *FileSQLAdapter) Query(ctx context.Context, query string) (*model.Table, error) {
	if f.sharedDB == nil {
		return nil, &FileSQLError{Op: opQuery, Err: "shared database not initialized"}
	}

	rows, err := f.sharedDB.QueryContext(ctx, query)
	if err != nil {
		return nil, &FileSQLError{Op: opQuery, Err: err.Error()}
	}
	defer func() { _ = rows.Close() }()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, &FileSQLError{Op: "columns", Err: err.Error()}
	}

	header := model.NewHeader(columns)
	var records []model.Record

	// Scan all rows
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, &FileSQLError{Op: "scan", Err: err.Error()}
		}

		// Convert to string slice
		record := make([]string, len(columns))
		for i, val := range values {
			if val == nil {
				record[i] = ""
			} else {
				// Handle different types that can be returned from SQL
				switch v := val.(type) {
				case []byte:
					record[i] = string(v)
				case string:
					record[i] = v
				case int64:
					record[i] = strconv.FormatInt(v, 10)
				case float64:
					record[i] = fmt.Sprintf("%g", v)
				default:
					record[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		records = append(records, model.NewRecord(record))
	}

	if err := rows.Err(); err != nil {
		return nil, &FileSQLError{Op: opRows, Err: err.Error()}
	}

	// Generate unique table name for query results to avoid conflicts
	tableName := "query_result_" + generateRandomName()

	return model.NewTable(tableName, header, records), nil
}

// Exec executes SQL statement (INSERT, UPDATE, DELETE)
func (f *FileSQLAdapter) Exec(ctx context.Context, statement string) (int64, error) {
	if f.sharedDB == nil {
		return 0, &FileSQLError{Op: opExec, Err: errDatabaseNotInit}
	}

	result, err := f.sharedDB.ExecContext(ctx, statement)
	if err != nil {
		return 0, &FileSQLError{Op: opExec, Err: err.Error()}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &FileSQLError{Op: "rows_affected", Err: err.Error()}
	}

	return rowsAffected, nil
}

// GetTableNames returns all table names in the database
func (f *FileSQLAdapter) GetTableNames(ctx context.Context) ([]*model.Table, error) {
	if f.sharedDB == nil {
		return nil, &FileSQLError{Op: opGetTables, Err: errDatabaseNotInit}
	}

	// Query sqlite_master for table names, excluding system tables and temporary query result tables
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%'"
	rows, err := f.sharedDB.QueryContext(ctx, query)
	if err != nil {
		return nil, &FileSQLError{Op: opGetTables, Err: err.Error()}
	}
	defer func() { _ = rows.Close() }()

	var tables []*model.Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, &FileSQLError{Op: opScanTable, Err: err.Error()}
		}

		// Create table model with just the name
		tables = append(tables, model.NewTable(tableName, nil, nil))
	}

	if err := rows.Err(); err != nil {
		return nil, &FileSQLError{Op: opRows, Err: err.Error()}
	}

	return tables, nil
}

// GetTableHeader returns header information for a specific table.
// The tableName is safely quoted via QuoteIdentifier, so any non-empty
// SQLite identifier (including names with spaces, hyphens, or starting
// with digits) is accepted.
func (f *FileSQLAdapter) GetTableHeader(ctx context.Context, tableName string) (*model.Table, error) {
	if f.sharedDB == nil {
		return nil, &FileSQLError{Op: opGetHeader, Err: errDatabaseNotInit}
	}
	if strings.TrimSpace(tableName) == "" {
		return nil, &FileSQLError{Op: opGetHeader, Err: "table name is empty"}
	}

	// Get column info using PRAGMA
	// nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli
	query := "PRAGMA table_info(" + QuoteIdentifier(tableName) + ")" // #nosec G202
	rows, err := f.sharedDB.QueryContext(ctx, query)
	if err != nil {
		return nil, &FileSQLError{Op: opGetHeader, Err: err.Error()}
	}
	defer func() { _ = rows.Close() }()

	var columns []string
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, &FileSQLError{Op: opScanHeader, Err: err.Error()}
		}
		columns = append(columns, name)
	}

	if err := rows.Err(); err != nil {
		return nil, &FileSQLError{Op: "rows", Err: err.Error()}
	}

	header := model.NewHeader(columns)
	return model.NewTable(tableName, header, nil), nil
}

// Close closes the database connection
func (f *FileSQLAdapter) Close() error {
	// The shared database is managed by the main application
	// We don't close it here
	return nil
}

// GetTableNameFromFilePath extracts table name from file path.
// This function matches the naming logic used by filesql's sanitizeTableName(tableFromFilePath())
// to ensure consistent table name generation between sqly and filesql.
func GetTableNameFromFilePath(filePath string) string {
	// Get base filename without directory
	filename := filepath.Base(filePath)

	// Remove compression extensions first (case-insensitive, matching filesql behavior)
	lowerFilename := strings.ToLower(filename)
	for _, ext := range compressionExts {
		if strings.HasSuffix(lowerFilename, ext) {
			filename = filename[:len(filename)-len(ext)]
			break
		}
	}

	// Remove file extension
	ext := filepath.Ext(filename)
	if ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	return SanitizeForSQL(filename)
}

// QuoteIdentifier safely quotes SQL identifiers by escaping embedded double quotes.
// This handles reserved keywords, names starting with digits, and special characters.
//
// Example:
//
//	QuoteIdentifier("table_name") returns `"table_name"`
//	QuoteIdentifier("2023_data") returns `"2023_data"`
//	QuoteIdentifier(`foo"bar`) returns `"foo""bar"`
func QuoteIdentifier(identifier string) string {
	// Escape any existing double quotes by doubling them
	escaped := strings.ReplaceAll(identifier, `"`, `""`)
	// Wrap with double quotes
	return `"` + escaped + `"`
}

// SanitizeForSQL sanitizes a string to be SQL-safe. This function matches
// the sanitization logic used by filesql library's sanitizeTableName() to ensure
// consistent table name generation between sqly and filesql.
//
// Transformations applied:
//   - Replaces spaces, hyphens (-), and dots (.) with underscores
//   - Removes any non-alphanumeric characters except underscores
//   - Adds "sheet_" prefix if the name starts with a number
//   - Returns "sheet" as fallback for empty names
//
// Example:
//
//	SanitizeForSQL("A test") returns "A_test"
//	SanitizeForSQL("Café") returns "Caf"
//	SanitizeForSQL("Sheet-1") returns "Sheet_1"
//	SanitizeForSQL("2023-data") returns "sheet_2023_data"
func SanitizeForSQL(name string) string {
	// First replace spaces, hyphens, and dots with underscores
	result := strings.ReplaceAll(name, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")

	// Then remove any non-alphanumeric characters except underscore
	var sanitized strings.Builder
	for _, r := range result {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sanitized.WriteRune(r)
		}
	}

	finalResult := sanitized.String()

	// Add "sheet_" prefix if name starts with a number (matches filesql behavior)
	if len(finalResult) > 0 && finalResult[0] >= '0' && finalResult[0] <= '9' {
		finalResult = defaultSheetName + "_" + finalResult
	}

	// Return "sheet" as fallback for empty names (matches filesql behavior)
	if finalResult == "" {
		finalResult = defaultSheetName
	}

	return finalResult
}

// FileSQLError represents an error from filesql operations
//
//nolint:revive // Name maintained for consistency with FileSQLAdapter and clear context indication
type FileSQLError struct {
	Op  string
	Err string
}

func (e *FileSQLError) Error() string {
	return "filesql " + e.Op + ": " + e.Err
}

// supportedBaseExts lists the base file extensions that filesql can handle.
var supportedBaseExts = []string{".csv", ".tsv", ".ltsv", ".parquet", ".xlsx", ".json", ".jsonl"}

// compressionExts lists the compression extensions that filesql can decompress.
var compressionExts = []string{".gz", ".bz2", ".xz", ".zst", ".z", ".snappy", ".s2", ".lz4"}

// IsSupportedFile checks if the file has a format supported by filesql.
// This covers all formats that filesql can import: CSV, TSV, LTSV, JSON, JSONL,
// Parquet, XLSX (with compression variants), plus ACH and Fedwire.
func IsSupportedFile(filePath string) bool {
	lower := strings.ToLower(filePath)

	// Check ACH and Fedwire (no compression variants)
	if strings.HasSuffix(lower, ".ach") || strings.HasSuffix(lower, ".fed") {
		return true
	}

	// Strip compression extension if present
	for _, ext := range compressionExts {
		if before, ok := strings.CutSuffix(lower, ext); ok {
			lower = before
			break
		}
	}

	// Check base file extensions
	for _, ext := range supportedBaseExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// IsExcelFile checks if the file is an Excel format (.xlsx), including compressed variants.
func IsExcelFile(filePath string) bool {
	lower := strings.ToLower(filePath)

	// Strip compression extension if present
	for _, ext := range compressionExts {
		if before, ok := strings.CutSuffix(lower, ext); ok {
			lower = before
			break
		}
	}

	return strings.HasSuffix(lower, ".xlsx")
}

// generateRandomName generates a random 4-byte hex string.
func generateRandomName() string {
	const randomBytesLen = 4
	randomBytes := make([]byte, randomBytesLen)
	_, _ = rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}
