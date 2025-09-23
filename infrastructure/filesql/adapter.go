// Package filesql provides adapters for integrating nao1215/filesql package with sqly.
package filesql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
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

	// Use filesql to load files into temporary database, then copy to shared database
	tmpDB, err := filesql.OpenContext(ctx, filePaths...)
	if err != nil {
		return err
	}
	defer tmpDB.Close()

	// Get actual table names from the temporary database created by filesql
	// This handles cases where filesql creates different table names than expected (e.g., Excel sheets)
	rows, err := tmpDB.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("failed to get table names from filesql database: %w", err)
	}
	defer rows.Close()

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

// copyTableToSharedDB copies a table from source database to shared database using bulk insert optimization
func (f *FileSQLAdapter) copyTableToSharedDB(ctx context.Context, sourceDB *sql.DB, tableName string) error {
	// Drop existing table if it exists to avoid conflicts
	dropSQL := "DROP TABLE IF EXISTS " + quoteIdentifier(tableName)
	if _, err := f.sharedDB.ExecContext(ctx, dropSQL); err != nil {
		return fmt.Errorf("failed to drop existing table %s: %w", tableName, err)
	}

	// Use manual approach to preserve filesql's automatic type detection
	return f.copyTableManually(ctx, sourceDB, tableName)
}

// copyTableManually performs manual table copy with proper type preservation
func (f *FileSQLAdapter) copyTableManually(ctx context.Context, sourceDB *sql.DB, tableName string) error {
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
	rows, err := sourceDB.QueryContext(ctx, "PRAGMA table_info("+quoteIdentifier(tableName)+")")
	if err != nil {
		return err
	}
	defer rows.Close()

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
		quotedColumns[i] = quoteIdentifier(col)
	}
	quotedTableName := quoteIdentifier(tableName)

	// Begin transaction for bulk insert optimization
	tx, err := f.sharedDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be no-op if tx.Commit() succeeds

	// Copy data from source to shared database
	selectSQL := fmt.Sprintf("SELECT %s FROM %s", strings.Join(quotedColumns, ", "), quotedTableName) //nolint:gosec // Table name is controlled by filesql, columns are validated
	sourceRows, err := sourceDB.QueryContext(ctx, selectSQL)
	if err != nil {
		return err
	}
	defer sourceRows.Close()

	// Prepare insert statement with transaction
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", //nolint:gosec // Table and column names are controlled by filesql, placeholders are safe
		quotedTableName, strings.Join(quotedColumns, ", "), strings.Join(placeholders, ", "))
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

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
			defer tx.Rollback()

			// Re-prepare statement for new transaction
			if stmt, err = tx.PrepareContext(ctx, insertSQL); err != nil {
				return fmt.Errorf("failed to re-prepare statement at row %d: %w", rowCount, err)
			}
			defer stmt.Close()
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
		return nil, &FileSQLError{Op: "query", Err: "shared database not initialized"}
	}

	rows, err := f.sharedDB.QueryContext(ctx, query)
	if err != nil {
		return nil, &FileSQLError{Op: "query", Err: err.Error()}
	}
	defer rows.Close()

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
		return nil, &FileSQLError{Op: "rows", Err: err.Error()}
	}

	// Generate unique table name for query results to avoid conflicts
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, &FileSQLError{Op: "generate_table_name", Err: err.Error()}
	}
	tableName := "query_result_" + hex.EncodeToString(randomBytes)

	return model.NewTable(tableName, header, records), nil
}

// Exec executes SQL statement (INSERT, UPDATE, DELETE)
func (f *FileSQLAdapter) Exec(ctx context.Context, statement string) (int64, error) {
	if f.sharedDB == nil {
		return 0, &FileSQLError{Op: "exec", Err: "database not initialized"}
	}

	result, err := f.sharedDB.ExecContext(ctx, statement)
	if err != nil {
		return 0, &FileSQLError{Op: "exec", Err: err.Error()}
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
		return nil, &FileSQLError{Op: "get_tables", Err: "database not initialized"}
	}

	// Query sqlite_master for table names, excluding system tables and temporary query result tables
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%'"
	rows, err := f.sharedDB.QueryContext(ctx, query)
	if err != nil {
		return nil, &FileSQLError{Op: "get_tables", Err: err.Error()}
	}
	defer rows.Close()

	var tables []*model.Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, &FileSQLError{Op: "scan_table", Err: err.Error()}
		}

		// Create table model with just the name
		tables = append(tables, model.NewTable(tableName, nil, nil))
	}

	if err := rows.Err(); err != nil {
		return nil, &FileSQLError{Op: "rows", Err: err.Error()}
	}

	return tables, nil
}

// GetTableHeader returns header information for a specific table
func (f *FileSQLAdapter) GetTableHeader(ctx context.Context, tableName string) (*model.Table, error) {
	if f.sharedDB == nil {
		return nil, &FileSQLError{Op: "get_header", Err: "database not initialized"}
	}

	// Get column info using PRAGMA
	query := "PRAGMA table_info(" + quoteIdentifier(tableName) + ")"
	rows, err := f.sharedDB.QueryContext(ctx, query)
	if err != nil {
		return nil, &FileSQLError{Op: "get_header", Err: err.Error()}
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, &FileSQLError{Op: "scan_header", Err: err.Error()}
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

// GetTableNameFromFilePath extracts table name from file path (compatible with sqly logic)
func GetTableNameFromFilePath(filePath string) string {
	// Get base filename without directory
	filename := filepath.Base(filePath)

	// Remove compression extensions first (.gz, .bz2, .xz, .zst)
	for {
		ext := filepath.Ext(filename)
		if ext == ".gz" || ext == ".bz2" || ext == ".xz" || ext == ".zst" {
			filename = strings.TrimSuffix(filename, ext)
		} else {
			break
		}
	}

	// Remove file extension (.csv, .tsv, .ltsv, .xlsx, .parquet)
	ext := filepath.Ext(filename)
	if ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Sanitize filename to be SQL-safe
	// Replace characters that can cause SQL syntax errors with underscores
	// This includes: hyphen (-), dot (.), and other non-alphanumeric characters except underscore
	result := make([]rune, 0, len(filename))
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result = append(result, r)
		} else {
			result = append(result, '_')
		}
	}

	return string(result)
}

// quoteIdentifier safely quotes SQL identifiers by escaping embedded double quotes
func quoteIdentifier(identifier string) string {
	// Escape any existing double quotes by doubling them
	escaped := strings.ReplaceAll(identifier, `"`, `""`)
	// Wrap with double quotes
	return `"` + escaped + `"`
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
