// Package filesql provides adapters for integrating nao1215/filesql package with sqly.
package filesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nao1215/filesql"
	"github.com/nao1215/sqly/domain/model"
	infra "github.com/nao1215/sqly/infrastructure"
)

const (
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
	// includeHiddenSheets makes an Excel import load the sheets a workbook hides
	// as well as the ones it shows. It defaults to false — sqly's default is the
	// opposite of filesql's, because sqly presents workbooks to someone who did
	// not build them — and is set by the --include-hidden-sheets flag for the
	// whole session, so a later .import applies it too.
	includeHiddenSheets bool
	// skipped holds what --row-mismatch skip discarded during the imports of
	// this session, keyed by table. A dropped row is what the user asked for,
	// but an import that says nothing leaves one dropped row and most of the
	// file dropped looking the same — and .save --in-place then makes either
	// permanent.
	skipped map[string]model.SkippedRows
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

// SetIncludeHiddenSheets decides whether subsequent Excel imports load the
// sheets a workbook hides as well as the ones it shows.
func (f *FileSQLAdapter) SetIncludeHiddenSheets(include bool) {
	f.includeHiddenSheets = include
}

// IncludeHiddenSheets reports whether Excel imports load hidden sheets.
func (f *FileSQLAdapter) IncludeHiddenSheets() bool {
	return f.includeHiddenSheets
}

// ExcelSheets reports every sheet of the workbook at path, in workbook order,
// with whether the workbook shows it.
//
// It asks filesql rather than opening the workbook here. The rule that decides
// which sheets an import loads lives there, and a second reader in sqly would
// be free to disagree with it — which is exactly the thing --inspect exists to
// rule out.
func (f *FileSQLAdapter) ExcelSheets(path string) ([]model.ExcelSheet, error) {
	sheets, err := filesql.ExcelSheetsInFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the sheets of %q: %w", path, err)
	}
	result := make([]model.ExcelSheet, 0, len(sheets))
	for _, sheet := range sheets {
		result = append(result, model.ExcelSheet{Name: sheet.Name, Visible: sheet.Visible})
	}
	return result, nil
}

// ExcelSheetTableNames maps each named sheet of a workbook to the table it is
// loaded as. It defers to filesql for the same reason ExcelSheets does: the
// sheet-to-table rule is filesql's, and a copy here would be free to drift from
// the names an import actually creates.
func (f *FileSQLAdapter) ExcelSheetTableNames(path string, sheetNames []string) ([]string, error) {
	tables, err := filesql.ExcelSheetTableNames(path, sheetNames)
	if err != nil {
		return nil, fmt.Errorf("map the sheets of %q to tables: %w", path, err)
	}
	return tables, nil
}

// filesqlExcelSheetPolicy maps sqly's setting onto the filesql policy that
// drives the import. sqly's default is visible-only and filesql's is all, so
// the mapping is where the two defaults are reconciled — deliberately, in one
// place, rather than by changing filesql's default under its other callers.
func filesqlExcelSheetPolicy(includeHidden bool) filesql.ExcelSheetPolicy {
	if includeHidden {
		return filesql.ExcelSheetPolicyAll
	}
	return filesql.ExcelSheetPolicyVisibleOnly
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

// stageFunc loads one input path into an open transaction.
type stageFunc[T infra.Tx] func(ctx context.Context, tx T, path string) error

// atomicImport is one ordered multi-file import: where the transaction comes
// from, and how a single path is staged into it. Splitting these two out of
// LoadFiles is what makes BeginTx, staging, commit, and rollback failures
// reproducible from a test without a real database that must be coaxed into
// failing at the right instant.
type atomicImport[T infra.Tx] struct {
	beginner infra.TxBeginner[T]
	stage    stageFunc[T]
}

// run stages every path inside one transaction.
//
// Everything an import touches lives in that transaction, including the
// metadata filesql keeps for writing ACH and Fedwire files back. There is no
// second channel that can outlive a failure on its own: whatever the
// transaction's fate, the metadata shares it, so a bad input that rolls back
// leaves neither the tables nor a record pointing at them. A commit or rollback
// that itself fails leaves the database in whatever state that failure left it,
// which the returned error says; WithTransaction owns commit, rollback, and the
// joining of a cleanup error onto the cause.
//
// Staging runs in the order the paths were given, so when several inputs claim
// the same base name, write-back resolves to the last one.
func (a atomicImport[T]) run(ctx context.Context, paths []string) error {
	committed, err := infra.WithTransaction(ctx, a.beginner, func(tx T) error {
		for _, path := range paths {
			if err := a.stage(ctx, tx, path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !committed {
		// Unreachable while WithTransaction reports success and commitment
		// together; kept so a future change to that contract fails loudly here.
		return errors.New("atomic import finished without committing")
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
func (f *FileSQLAdapter) stageFile(ctx context.Context, tx *sql.Tx, path string) error {
	builder := filesql.NewBuilder().
		AddPath(path).
		WithMalformedRowPolicy(filesqlRowMismatchPolicy(f.rowMismatchPolicy)).
		WithExcelSheetPolicy(filesqlExcelSheetPolicy(f.includeHiddenSheets))
	validated, err := builder.Build(ctx)
	if err != nil {
		return importError(path, err)
	}
	err = validated.LoadIntoTx(ctx, tx)
	f.recordSkippedRows(validated.SkippedRows())
	if err != nil {
		if f.rowMismatchPolicy == model.RowMismatchPad && errors.Is(err, filesql.ErrColumnMismatch) {
			return importError(path, fmt.Errorf("--row-mismatch pad refuses to truncate a long row: %w", unnamedCause(err)))
		}
		return importError(path, err)
	}
	return nil
}

// recordSkippedRows keeps what one import discarded, so the shell can report it
// once the import has committed. A later import of the same table replaces the
// count, because it replaced the table.
func (f *FileSQLAdapter) recordSkippedRows(skipped []filesql.SkippedRows) {
	if len(skipped) == 0 {
		return
	}
	if f.skipped == nil {
		f.skipped = make(map[string]model.SkippedRows, len(skipped))
	}
	for _, s := range skipped {
		f.skipped[s.Table] = model.SkippedRows{Table: s.Table, Count: s.Count, Total: s.Total}
	}
}

// SkippedRows returns what the row-mismatch policy dropped for the given tables,
// and forgets it: a count is reported once, at the import that produced it.
func (f *FileSQLAdapter) SkippedRows(tables []string) []model.SkippedRows {
	if len(f.skipped) == 0 {
		return nil
	}
	out := make([]model.SkippedRows, 0, len(tables))
	for _, name := range tables {
		if s, ok := f.skipped[name]; ok {
			out = append(out, s)
			delete(f.skipped, name)
		}
	}
	return out
}

// importError names the input a load failed on, carrying the path as a value so
// the caller can report the path the user typed rather than the staged copy this
// layer was given.
func importError(path string, err error) error {
	return &model.ImportError{Path: path, Err: unnamedCause(err)}
}

// unnamedCause drops the file name filesql puts on a load failure, because the
// path travels beside the error instead.
//
// Both layers want to say which file failed. filesql has to, because a caller
// can hand it many at once; sqly loads one file per call and reports the failure
// against the path the user typed, so keeping filesql's copy made every message
// name the same file twice. The cause is reached through the type filesql
// provides for this, not by cutting the path back out of the text.
func unnamedCause(err error) error {
	var parseErr *filesql.ParseError
	if errors.As(err, &parseErr) {
		return parseErr.Err
	}
	return err
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

// SanitizeForSQL derives the table name filesql gives a source of this name.
//
// It has to agree with filesql exactly, because sqly works out which tables an
// import will create before running it — to refuse two inputs that would claim
// the same one. A rule that differs invents collisions that do not exist and
// misses ones that do. It used to keep ASCII letters and digits only, so every
// file named in a non-Latin script sanitized to the empty string and then to the
// fallback "sheet": two such files looked like a collision, while filesql was
// happily naming their tables after them.
//
// Letters, digits, and combining marks are judged by Unicode category, matching
// filesql. A combining mark is kept so a decomposed accent stays attached to its
// base letter.
//
// Transformations applied:
//   - Replaces spaces, hyphens (-), and dots (.) with underscores
//   - Drops every character that is not a letter, digit, mark, or underscore
//   - Adds "sheet_" prefix if the name starts with a digit
//   - Returns "sheet" as fallback for empty names
//
// Example:
//
//	SanitizeForSQL("A test") returns "A_test"
//	SanitizeForSQL("Café") returns "Café"
//	SanitizeForSQL("売上") returns "売上"
//	SanitizeForSQL("Sheet-1") returns "Sheet_1"
//	SanitizeForSQL("2023-data") returns "sheet_2023_data"
func SanitizeForSQL(name string) string {
	// First replace spaces, hyphens, and dots with underscores
	result := strings.ReplaceAll(name, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")

	// Then drop everything an identifier may not carry.
	var sanitized strings.Builder
	for _, r := range result {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_' {
			sanitized.WriteRune(r)
		}
	}

	finalResult := sanitized.String()

	// Add "sheet_" prefix if the name starts with a digit (matches filesql).
	if first, _ := utf8.DecodeRuneInString(finalResult); unicode.IsDigit(first) {
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
