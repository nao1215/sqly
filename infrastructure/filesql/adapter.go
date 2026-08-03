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
	"strings"

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/cleanup"
	"github.com/nao1215/sqly/domain/model"
	infra "github.com/nao1215/sqly/infrastructure"
	"github.com/xuri/excelize/v2"
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
	// rowMismatchPolicy controls how a CSV/TSV row whose field count differs from
	// the header is handled on import.
	// It defaults to RowMismatchError and is updated by the --row-mismatch flag and
	// the .row-mismatch shell command.
	rowMismatchPolicy model.RowMismatchPolicy
}

// NewFileSQLAdapter creates a new adapter for filesql integration
func NewFileSQLAdapter(sharedDB *sql.DB) *FileSQLAdapter {
	return &FileSQLAdapter{
		sharedDB: sharedDB,
	}
}

// SetRowMismatchPolicy sets how a CSV/TSV row (one whose field count
// differs from the header) is handled by subsequent imports.
func (f *FileSQLAdapter) SetRowMismatchPolicy(policy model.RowMismatchPolicy) {
	f.rowMismatchPolicy = policy
}

// RowMismatchPolicy returns the policy applied to mismatched CSV/TSV rows on import.
func (f *FileSQLAdapter) RowMismatchPolicy() model.RowMismatchPolicy {
	return f.rowMismatchPolicy
}

// filesqlRowMismatchPolicy maps sqly's policy to the filesql policy that drives
// the actual import. Both enums share the same three cases; keeping them
// separate lets sqly's layers stay independent of the filesql type.
func filesqlRowMismatchPolicy(policy model.RowMismatchPolicy) filesql.MalformedRowPolicy {
	switch policy {
	case model.RowMismatchSkip:
		return filesql.MalformedRowSkip
	case model.RowMismatchPad:
		return filesql.MalformedRowFill
	default:
		return filesql.MalformedRowStop
	}
}

// registryPublisher is the deferred half of an import: metadata that names
// tables which do not exist until the transaction commits. filesql's
// *PendingRegistries satisfies it; a test double satisfies it to observe whether
// publication happened at all.
//
// PublishRegistries cannot fail — it moves already-built table sets into
// process maps — so there is no partially published state to reconcile. The
// integrity rule is therefore the simple one: nothing is published unless the
// commit succeeded, and once the commit succeeds every staged registry is
// published.
type registryPublisher interface {
	PublishRegistries()
}

// stageFunc loads one input path into an open transaction and returns the
// registry entries to publish after that transaction commits.
type stageFunc[T infra.Tx] func(ctx context.Context, tx T, path string) (registryPublisher, error)

// atomicImport is one ordered multi-file import: where the transaction comes
// from, and how a single path is staged into it. Splitting these two out of
// LoadFiles is what makes BeginTx, staging, commit, and rollback failures
// reproducible from a test without a real database that must be coaxed into
// failing at the right instant.
type atomicImport[T infra.Tx] struct {
	beginner infra.TxBeginner[T]
	stage    stageFunc[T]
}

// run stages every path inside one transaction, then publishes the staged
// registries — and only then.
//
// The two phases are deliberately separate. Everything that touches the
// database happens inside WithTransaction, which owns commit, rollback, and the
// joining of a cleanup error onto the cause. Everything that makes state visible
// to the rest of the process happens after it returns, gated on the transaction
// having actually committed. A failure anywhere in the first phase — a bad
// input, a failed commit, or a rollback that itself failed — therefore leaves no
// registry entry behind, so "the database rolled back but the registry kept the
// entry" is not a state this code can produce.
//
// When several inputs register the same base name, the later input wins: staging
// runs in the order the paths were given and publication replays that same
// order, so the last file to claim a name is the one write-back resolves to.
func (a atomicImport[T]) run(ctx context.Context, paths []string) error {
	var pending []registryPublisher
	committed, err := infra.WithTransaction(ctx, a.beginner, func(tx T) error {
		pending = pending[:0]
		for _, path := range paths {
			publisher, err := a.stage(ctx, tx, path)
			if err != nil {
				return err
			}
			pending = append(pending, publisher)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !committed {
		// Unreachable while WithTransaction reports success and commitment
		// together; kept so a future change to that contract fails loudly here
		// instead of publishing uncommitted registries.
		return errors.New("atomic import finished without committing")
	}
	for _, publisher := range pending {
		publisher.PublishRegistries()
	}
	return nil
}

// LoadFiles loads multiple files into the shared database using filesql. Either
// every input is applied or none is: a failure on the last of ten inputs rolls
// back the nine before it, leaving tables and views that existed beforehand
// untouched.
func (f *FileSQLAdapter) LoadFiles(ctx context.Context, filePaths ...string) error {
	if len(filePaths) == 0 {
		return nil
	}
	if f.sharedDB == nil {
		return errors.New("shared database is not initialized")
	}
	return atomicImport[*sql.Tx]{
		beginner: infra.SQLTxBeginner{DB: f.sharedDB},
		stage:    f.stageFile,
	}.run(ctx, filePaths)
}

// stageFile parses one input and applies it to the open import transaction.
func (f *FileSQLAdapter) stageFile(ctx context.Context, tx *sql.Tx, path string) (registryPublisher, error) {
	builder := filesql.NewBuilder().
		AddPath(path).
		WithMalformedRowPolicy(filesqlRowMismatchPolicy(f.rowMismatchPolicy))
	validated, err := builder.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("load file %q: %w", path, err)
	}
	registry, err := validated.LoadIntoTxWithPending(ctx, tx)
	if err != nil {
		if f.rowMismatchPolicy == model.RowMismatchPad && errors.Is(err, filesql.ErrColumnMismatch) {
			return nil, fmt.Errorf("load file %q: --row-mismatch pad refuses to truncate a long row: %w", path, err)
		}
		return nil, fmt.Errorf("load file %q: %w", path, err)
	}
	return registry, nil
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
	var cells [][]model.Cell

	// Scan all rows, keeping the driver's native value per cell. model.Cell
	// derives the display string from it, so this path preserves SQLite's
	// INTEGER/REAL/TEXT types and its NULLs the same way the memory repository
	// does instead of flattening every value to a string here.
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, &FileSQLError{Op: "scan", Err: err.Error()}
		}

		row := make([]model.Cell, len(columns))
		for i, val := range values {
			row[i] = model.NewCell(val)
		}
		cells = append(cells, row)
	}

	if err := rows.Err(); err != nil {
		return nil, &FileSQLError{Op: opRows, Err: err.Error()}
	}

	// Generate unique table name for query results to avoid conflicts
	tableName := "query_result_" + generateRandomName()

	table, err := model.NewTableFromCells(tableName, header, cells)
	if err != nil {
		return nil, &FileSQLError{Op: opQuery, Err: err.Error()}
	}
	return table, nil
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

// SheetNames returns the worksheet names of an Excel workbook in their
// in-workbook order. It is used for interactive --sheet completion. The workbook
// is read through the adapter's decompressing reader, so compressed variants
// (.xlsx.gz, .xlsx.zst, ...) are supported the same way as a plain .xlsx.
func SheetNames(filePath string) (names []string, err error) {
	r, closeReader, err := NewDecompressingReaderForFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file %s: %w", filePath, err)
	}
	defer func() {
		err = cleanup.Join(err, closeReader(), "close decompressing reader")
	}()

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel file %s: %w", filePath, err)
	}
	defer func() {
		err = cleanup.Join(err, f.Close(), "close Excel workbook")
	}()

	return f.GetSheetList(), nil
}

// generateRandomName generates a random 4-byte hex string.
func generateRandomName() string {
	const randomBytesLen = 4
	randomBytes := make([]byte, randomBytesLen)
	_, _ = rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}
