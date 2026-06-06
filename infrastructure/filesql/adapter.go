// Package filesql provides adapters for integrating nao1215/filesql package with sqly.
package filesql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
)

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

	// Loading an ACH/Fedwire file registers its TableSet in a filesql global
	// registry keyed by base name. sqly keeps those registrations for the session
	// so the whole-set write-back path (DumpACHFile/DumpFedWireFile) can rebuild a
	// valid .ach/.fed file from the (possibly edited) tables. The registry is keyed
	// by base name, so re-importing the same source overwrites its entry, and the
	// process is short-lived, so the retained TableSets are released at exit.

	// An empty JSON array ("[]") or an empty JSONL file is valid JSON input but
	// has no rows. filesql rejects it as an empty data source, so handle those
	// files here as zero-row tables (a single "data" column, matching filesql's
	// JSON schema) before delegating the rest to filesql.
	var toLoad []string
	for _, path := range filePaths {
		name, isEmpty := emptyJSONLikeTable(path)
		if !isEmpty {
			toLoad = append(toLoad, path)
			continue
		}
		if err := f.createEmptyJSONTable(ctx, name); err != nil {
			return err
		}
	}

	// Stream the files directly into the shared session database. filesql's
	// LoadInto replaces a same-named table (last-wins), matching sqly's import
	// semantics, and avoids the previous temporary-database-plus-row-copy path.
	if len(toLoad) > 0 {
		if err := filesql.LoadInto(ctx, f.sharedDB, toLoad...); err != nil {
			return err
		}
	}

	return nil
}

// jsonDataColumn is the single column filesql uses to store raw JSON/JSONL
// values; sqly creates the same schema for an empty JSON input so queries with
// json_extract() behave the same on a zero-row table.
const jsonDataColumn = "data"

// emptyJSONLikeTable reports whether a .json or .jsonl file (uncompressed or
// compressed, e.g. .json.gz) holds no rows (an empty JSON array, whitespace-only
// JSON, or an empty/blank-only JSONL file), returning the table name to create for
// it. The format is decided by the base extension after any compression suffix is
// stripped, and the content is read through filesql's decompressor so a compressed
// empty input is detected the same as an uncompressed one.,
func emptyJSONLikeTable(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(stripCompressionExt(path))) {
	case model.ExtJSON:
		data, err := readDecompressed(path)
		if err != nil {
			return "", false
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return GetTableNameFromFilePath(path), true
		}
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil && len(arr) == 0 {
			return GetTableNameFromFilePath(path), true
		}
		return "", false
	case model.ExtJSONL:
		data, err := readDecompressed(path)
		if err != nil {
			return "", false
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				return "", false
			}
		}
		return GetTableNameFromFilePath(path), true
	default:
		return "", false
	}
}

// stripCompressionExt removes a single trailing compression extension from path
// (case-insensitive), so the base format of "data.json.gz" is read from ".json".
func stripCompressionExt(path string) string {
	lower := strings.ToLower(path)
	for _, ext := range compressionExts {
		if strings.HasSuffix(lower, ext) {
			return path[:len(path)-len(ext)]
		}
	}
	return path
}

// readDecompressed reads the full content of path, transparently decompressing it
// when its extension names a known codec. It backs the empty JSON/JSONL detection
// for both plain and compressed inputs.
func readDecompressed(path string) ([]byte, error) {
	r, cleanup, err := NewDecompressingReaderForFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cleanup() }()
	return io.ReadAll(r)
}

// createEmptyJSONTable creates (last-wins) a zero-row table with filesql's JSON
// "data" column, so an empty JSON/JSONL input imports as an empty table instead
// of failing.
func (f *FileSQLAdapter) createEmptyJSONTable(ctx context.Context, name string) error {
	quoted := QuoteIdentifier(name)
	if _, err := f.sharedDB.ExecContext(ctx, "DROP TABLE IF EXISTS "+quoted); err != nil {
		return fmt.Errorf("failed to reset empty JSON table %q: %w", name, err)
	}
	if _, err := f.sharedDB.ExecContext(ctx, fmt.Sprintf("CREATE TABLE %s (%s TEXT)", quoted, jsonDataColumn)); err != nil {
		return fmt.Errorf("failed to create empty JSON table %q: %w", name, err)
	}
	return nil
}

// LoadFile loads a single file into the database
func (f *FileSQLAdapter) LoadFile(ctx context.Context, filePath string) error {
	return f.LoadFiles(ctx, filePath)
}

// DumpACHFile reconstructs a complete ACH file at outputPath from the table set
// registered under baseName, reflecting any UPDATEs applied to those tables in
// the session. It reads the current rows from the shared session database that
// the queries ran against, so edits are included. It returns an error when no ACH
// table set is registered for baseName (for example after the source was never
// imported as ACH, or the registry entry was cleared).
func (f *FileSQLAdapter) DumpACHFile(ctx context.Context, baseName, outputPath string) error {
	if f.sharedDB == nil {
		return errors.New(errDatabaseNotInit)
	}
	return filesql.DumpACH(ctx, f.sharedDB, baseName, outputPath)
}

// DumpFedWireFile reconstructs a complete Fedwire file at outputPath from the
// message table registered under baseName, reflecting any UPDATEs applied in the
// session. It returns an error when no Fedwire table set is registered for
// baseName.
func (f *FileSQLAdapter) DumpFedWireFile(ctx context.Context, baseName, outputPath string) error {
	if f.sharedDB == nil {
		return errors.New(errDatabaseNotInit)
	}
	return filesql.DumpFedWire(ctx, f.sharedDB, baseName, outputPath)
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
