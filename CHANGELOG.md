# CHANGELOG

## [Unreleased]

### Bug Fixes
* Table-Name Collision Detection: When two inputs sanitize to the same table name (for example `a-b.csv` and `a_b.csv`, both becoming `a_b`), sqly now fails with a clear collision error instead of letting the later import silently overwrite the earlier one while keeping the first file's source metadata.
* Input Path Validation False Positives: Input path validation no longer rejects legitimate paths. The arbitrary 10-level directory-depth limit is removed, so deeply nested workspace paths import (#316), and the URL-encoded traversal patterns (`..%2f`, `..%5c`) are no longer matched, so a real filename that merely contains those bytes is accepted (#317). sqly runs locally with the user's own permissions, so these web-style traversal checks only produced false rejections.
* Helper Commands Reject Extra Arguments: `.schema`, `.describe`, `.header`, `.mode`, `.tables`, and `.help` now reject unexpected trailing arguments with a clear error instead of silently ignoring them, so typos no longer pass unnoticed.
* Output Requires SQL: `--output` is now rejected with a clear error when no `--sql` query is supplied (including batch stdin, `--sql-file`, and interactive runs), instead of being silently ignored while the command still exits successfully. `--output` is only honored by the single-result `--sql` path.
* Empty Command Arguments Rejected: `.save ""`, `.dump TABLE ""`, and `.import ""` now fail with a clear error instead of being reinterpreted. `.save ""` no longer behaves like an in-place save (which bypassed `--force`), `.dump TABLE ""` no longer writes a file named `.csv`, and `.import ""` no longer imports the current working directory.
* Stdin Dataset Source And Name Safety: `--inspect` now reports a stable `stdin` source for a piped `--stdin` dataset instead of leaking the ephemeral staging temp path. `--save`/`--save-dir` reject a stdin-backed table up front instead of failing late while trying to write to a deleted temp file. `--stdin-name` is validated and rejects empty or path-like values, so it can no longer stage files outside the temp directory.
* Import Failure Handling: When an explicitly requested input fails to import, non-interactive runs now exit non-zero instead of continuing on the partially imported subset. This covers query mode (`--sql`/`--sql-file`), `--inspect`, and the batch `.import` command (which also stops later commands). Import diagnostics now always go to stderr, so stdout stays reserved for query results and the `--inspect` JSON report. The interactive shell still starts after a partial import, with a warning, since the loaded tables remain usable.
* Batch Fail-Fast: Batch mode (piped stdin and `--sql-file`) now stops at the first failed statement or helper command instead of continuing. Later statements no longer run, so their output cannot leak into a pipeline the process then reports as failed, and side-effecting commands such as `.save` and `.dump` placed after a failure no longer execute. The run still exits non-zero.
* Empty Batch No Write-Back: An empty batch (for example empty piped stdin) no longer triggers `--save`/`--save-dir` write-back. With nothing executed, source files are left untouched and the run is a no-op.
* Sheet Flag Validation For Directories And Empty Values: `--sheet` is now rejected when a directory input contains no Excel files, and when it is given an explicit empty value (`--sheet ""`). Both previously slipped past validation and were silently ignored. This applies to the CLI flag and the `.import` command.
* Batch Identifier Quoting: Batch statement splitting now recognizes SQLite bracket-quoted (`[ ... ]`) and backtick-quoted (`` `...` ``) identifiers, so a semicolon inside them no longer splits a statement. This matches the existing handling of single-quoted strings, double-quoted identifiers, and comments.
* File-Output Status On Stderr: Status lines for file-output operations (`--output`, `.dump`, and `.save`/`--save`/`--save-dir`) now go to stderr instead of stdout. When all data is written to files, stdout stays empty, matching `--inspect` and letting scripts rely on an empty stdout for success.
* Mode-Change Banner On Stderr: The `.mode` change banner now goes to stderr instead of stdout. In batch mode, switching to `.mode json` or `.mode ndjson` no longer prints a human-readable banner ahead of the machine-readable payload, so stdout stays parseable.
* Directory Output Targets: `--output` and `.dump` now reject a destination that already exists as a directory with a clear error, instead of silently writing to a sibling file such as `dir.csv`.
* Output Path Preservation: `--output` and `.dump` no longer rewrite a destination with an unknown extension to a sibling `.csv` file. The CSV fallback now writes to the exact path given (for example `--output out.unknown` writes to `out.unknown`), instead of silently creating `out.csv`.
* Inspect Flag Conflicts: `--inspect` now rejects conflicting action and side-effecting flags (`--sql`, `--sql-file`, `--output`, `--save`, `--save-dir`) with a clear error instead of silently discarding them.
* Excel Export Permissions: Exported `.xlsx` files are now created without executable bits (mode 0600), matching CSV, TSV, LTSV, and Parquet outputs. excelize's `SaveAs` created them as 0777, so they were left executable.
* Sheet Flag Validation: `--sheet` is now rejected with a clear error when no input can be an Excel file (for example a single CSV input or a `--stdin` dataset), instead of being silently ignored. Directory inputs are still accepted because they may contain Excel files.

### New Features
* Inspect Sample Control: `--inspect-sample N` sets how many sample rows `--inspect` includes per table (default 5). `--inspect-sample 0` produces a schema-only report, which keeps the output small for wide or multi-table sources such as Fedwire.
* SQL File Input: `--sql-file PATH` runs SQL loaded from a file for non-interactive runs. Because the query no longer comes from stdin, `--stdin <format>` can pipe a dataset while the query comes from the file (`cat data.csv | sqly --stdin csv --sql-file query.sql`). The file supports multiline statements and multiple statements separated by `;`, using the same splitting rules as batch stdin mode, and a leading header comment is allowed. It cannot be combined with `--sql`, and missing, unreadable, or empty files fail with a clear error.

## [v0.17.0](https://github.com/nao1215/sqly/compare/v0.16.0...v0.17.0) (2026-05-31)

### Performance
* Faster Imports: Files are streamed directly into the session database with filesql's `LoadInto` instead of being loaded into a temporary database and copied table by table. A 100k-row CSV import is about 2.5x faster and uses roughly half the peak memory. Behavior is unchanged (last-wins overwrite, cross-file JOINs, `.schema`/`.describe`/`--inspect`, and export all work as before).

### Dependencies
* filesql: 0.12.2 to 0.13.0 (adds `LoadInto` for loading files into an existing database).

### Bug Fixes
* Runtime History Tolerance: A history database that becomes read-only after startup no longer aborts `--sql`, `--inspect`, or batch runs. The first runtime read or write failure disables history for the rest of the session and warns once, instead of failing the command or retrying on every command. This extends the startup tolerance to the post-initialization path.
* Flags After Input Paths: Flags placed after file or directory arguments (e.g. `sqly --sql ... data.csv --output out.json`) are now parsed as flags instead of being silently treated as import paths that fail with "path does not exist". An unknown flag in any position fails fast with a clear parse error.
* History Storage Tolerance: Non-interactive runs (`--sql` and batch mode) no longer fail when the history database cannot be created or written (for example, a read-only config directory in CI or containers). History is disabled for the session with a warning, and the requested command still runs. Point `SQLY_HISTORY_DB_PATH` at a writable path to re-enable it.

### New Features
* Write-Back: Persist DML changes to files with explicit, opt-in flags and the `.save` command, so edits no longer vanish with the in-memory session. `--save-dir DIR` writes each table into DIR, preserving each source's format and compression and leaving the originals untouched. `--save` overwrites the source files in place and requires `--force`. In the interactive shell, `.save DIR` and `.save --force` do the same. Only single-source csv/tsv/ltsv/parquet tables are written; tables from a directory import, multi-table sources (Excel, ACH, Fedwire), and SQL-created tables are rejected with a clear error before anything is written. The save flags apply after `--sql` and batch runs; without them a session stays in-memory only.
* Inspect Workflow: `sqly --inspect FILE(S)|DIR(S)` imports the inputs and prints a machine-readable JSON report of every table (name, source path, column schema, row count, and a small sample of rows), then exits without starting the shell. It gives scripts and LLMs a non-interactive equivalent of `.tables`, `.schema`, and `.describe`. Progress messages go to stderr so stdout carries only the JSON.
* Export Format and Compression Inference: `--output` and `.dump` infer the export format and compression from the destination path, so `--output result.parquet` or `--output result.ndjson.gz` works without coordinating format flags. An explicit output mode that disagrees with the path extension is rejected instead of writing a surprising format. Text and JSON formats support `.gz`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, and `.lz4`; `.bz2` and compression on Parquet or Excel are rejected with a clear error.
* Multiline SQL in Batch Mode: Piped stdin is now parsed into statements instead of one statement per line, so SQL (including CTEs) can span multiple lines. A statement ends at a top-level `;`; separate multiple statements with `;`. Helper commands stay single-line, and a single trailing statement without `;` still runs. Errors report the statement index.
* Stdin Dataset Input: `--stdin <format>` (csv|tsv|ltsv|json|jsonl) imports piped stdin as a dataset instead of reading it as SQL/helper commands, so sqly works in Unix pipelines (e.g. `cat users.csv | sqly --stdin csv --sql "SELECT * FROM stdin"`). The table defaults to `stdin` and is overridable with `--stdin-name`; piped data can be joined with file and directory arguments. Without `--stdin`, non-TTY batch mode is unchanged.

## [v0.16.0](https://github.com/nao1215/sqly/compare/v0.15.0...v0.16.0) (2026-05-30)

### New Features
* Parquet Export: Export query results to Apache Parquet via `--parquet`, `.mode parquet`, `.dump`, and `--output`. Like Excel, it is export-only: on-screen it renders as CSV, and writes the file through filesql. Exporting an empty result errors because Parquet needs at least one row to infer its schema.
* Schema Inspection Commands: `.schema TABLE_NAME` prints the `CREATE TABLE` statement and `.describe TABLE_NAME` lists each column's position, name, type, nullability, default, and primary-key flag. Both work for CSV/TSV/LTSV/JSON, Excel, ACH, and Fedwire tables, and emit structured output in `.mode json`/`.mode ndjson`.
* JSON and NDJSON Output: Render query results as JSON or newline-delimited JSON via `--json`/`--ndjson`, `.mode json`/`.mode ndjson` in the shell, and `.dump`/`--output` for files. Values are emitted as strings like the other text formats; an empty result is `[]` for JSON and an empty stream for NDJSON.
* Non-TTY Batch Mode: When stdin is piped or redirected, sqly reads SQL and helper commands from stdin line by line. A failed command exits non-zero, so batch runs are scriptable (e.g. `echo 'SELECT * FROM sample' | sqly sample.csv`).
* Quoted Helper-Command Arguments: Helper commands honor single quotes, double quotes, and backslash-escaped whitespace, so file paths and `--sheet` values can contain spaces (e.g. `.import "my data.csv"`, `.import --sheet "Q1 Sales" report.xlsx`). The separated `--sheet NAME` form is now accepted alongside `--sheet=NAME`.

### Bug Fixes
* Shell Prompt Session: Reuse a single `sqly-shell` prompt across interactive commands so multiline SQL, history preload, and completion state no longer depend on per-command prompt teardown workarounds.
* `.cd` Prompt Path: Store the normalized absolute path after a directory change so the prompt stays correct after relative moves such as `.cd ..`. Argument-less `.cd` now resolves the home directory via `os.UserHomeDir`, fixing it on Windows where `$HOME` is usually unset.

### Refactoring
* Session Usecase Boundaries: Split the monolithic database usecase into focused `QueryUsecase`, `ImportUsecase`, and `MetadataUsecase` interfaces so each shell command depends only on the capability it uses. Behavior is unchanged.
* In-Process Shell Helpers: `.ls` and `.clear` no longer shell out to `ls`/`dir`/`clear`/`cls`. `.ls` lists entries sorted with a trailing `/` on directories for output stable across operating systems; `.clear` uses ANSI escapes. This avoids stalls in headless environments.

### Documentation
* filesql Session Integration: Documented the sqly/filesql integration model in the architecture page: a single shared in-memory SQLite session, importing by copying filesql's verbatim `CREATE TABLE` to preserve schema fidelity, and the deterministic ACH/Fedwire registry cleanup.

### Testing
* shellspec Binary E2E: Added shellspec end-to-end tests that drive the built binary (flags, piped stdin, exit codes) on Linux and macOS, run in CI via `make test-e2e`.
* Property-Based and Metamorphic Tests: Added `testing/quick` properties for JSON/NDJSON round-trips, `splitArgs` quoting, `trimGaps`/`normalizeDumpExt`/`SanitizeForSQL` invariants, and shell-level metamorphic relations (COUNT vs rows, ORDER BY permutation, format invariance, dump/reimport round-trip).
* filesql Integration Regressions: Added import regressions across CSV, JSONL, Parquet, Excel, ACH, and Fedwire, a Go test that locks filesql schema-type fidelity in the shared session, an ACH cleanup-determinism check, and a Parquet fixture.

## [v0.15.0](https://github.com/nao1215/sqly/compare/v0.14.2...v0.15.0) (2026-03-22)

### New Features
* **ACH/Fedwire Support**: Import and query ACH (`.ach`) and Fedwire (`.fed`) files
  - ACH files are loaded as multiple tables (`_file_header`, `_batches`, `_entries`, `_addenda`, and IAT variants)
  - Fedwire files are loaded as a single `_message` table
  - Full SQL query support on imported ACH/Fedwire data
  - `.dump` exports ACH/Fedwire tables to CSV/TSV/XLSX (round-trip to `.ach`/`.fed` format is not supported)

### Bug Fixes
* **Table Name Sanitization**: Align with filesql's `sanitizeTableName` rules ([eb78009](https://github.com/nao1215/sqly/commit/eb78009))
  - Names starting with a digit now get a `sheet_` prefix (e.g., `2023-data.csv` → table `sheet_2023_data`)
  - Special characters like `@`, `#`, `$` are removed (not replaced with `_`)
  - Empty names fall back to `"sheet"`
* **`--sheet` Filtering**: Fix recursive directory walk for sheet filtering ([7fd6230](https://github.com/nao1215/sqly/commit/7fd6230))
  - Previously only top-level directory entries were checked; now matches filesql's recursive import
  - Simplified to use prefix-based candidate matching for both directory and single-file imports
* **ACH/Fedwire Registry Cleanup**: Prevent memory leaks in long-running shells ([cee5e8b](https://github.com/nao1215/sqly/commit/cee5e8b), [f05449a](https://github.com/nao1215/sqly/commit/f05449a))
  - Clean up filesql global ACH/Fedwire registries via `defer` after import
  - Scope cleanup to actual `.ach`/`.fed` input paths, not table name suffixes
* **Windows CI**: Fix test timeout caused by PowerShell argument parsing ([5cab2c3](https://github.com/nao1215/sqly/commit/5cab2c3))
  - Use `shell: bash` in CI workflow to prevent `-coverprofile=coverage.out` misinterpretation
  - Remove `-coverpkg=./...` that caused shell test binary compilation to exceed 10-minute timeout

### Breaking Changes
* **Table Name Sanitization**: Files with digit-leading names now produce different table names
  - `2023-data.csv` → `sheet_2023_data` (was `2023_data`)
  - `data@file.csv` → `datafile` (was `data_file`)
  - This aligns sqly with filesql's naming rules and fixes `--sheet` filtering on numeric filenames

### Documentation
* Add ACH and Fedwire to supported formats table, usage, help, and all localized READMEs (EN, JA, KO, RU, ZH-CN, ES, FR)
* Update `.import` and `.dump` documentation in `sqly_helper_command.md`
* Clarify that compression extensions apply to tabular formats only, not ACH/Fedwire
* Fix French README diacritics

### Dependencies
* Bump github.com/nao1215/filesql from 0.8.0 to 0.12.0
* Bump github.com/olekukonko/tablewriter from 1.1.3 to 1.1.4
* Bump modernc.org/sqlite from 1.39.0 to 1.47.0

### Technical Improvements
* **Performance**: Use in-memory history DB in tests, reducing shell test time by ~75%
* **Testing**: Add ACH/Fedwire import smoke tests, naming consistency regression tests, and shell command coverage tests
* **Architecture**: Remove unused `IsACHTable`/`IsWireTable` from `DatabaseUsecase` interface
* **Code Quality**: Deduplicate compression extension list in `GetTableNameFromFilePath`

## [v0.14.2](https://github.com/nao1215/sqly/compare/v0.14.1...v0.14.2) (2025-12-06)

### New Features
* **Shell Command**: Add `.clear` command to clear terminal screen ([c26ddaf](https://github.com/nao1215/sqly/commit/c26ddaf))
  - Clear the terminal display with a simple `.clear` command
  - Uses `CommandContext` for proper context cancellation support
  - Cross-platform support for terminal clearing

### Documentation
* **README Updates**: Updated shell functions documentation to include `.clear` command ([6a48777](https://github.com/nao1215/sqly/commit/6a48777))

### Dependencies
* Bump github.com/nao1215/filesql from 0.4.5 to 0.5.0 ([3065465](https://github.com/nao1215/sqly/commit/3065465))
* Bump github.com/olekukonko/tablewriter from 1.1.0 to 1.1.2 ([afebb9c](https://github.com/nao1215/sqly/commit/afebb9c), [70c04c3](https://github.com/nao1215/sqly/commit/70c04c3))
* Bump github.com/xuri/excelize/v2 from 2.9.1 to 2.10.0 ([d27bf05](https://github.com/nao1215/sqly/commit/d27bf05))

### Technical Improvements
* **Code Quality**: Fix linter issues and update libraries ([be66492](https://github.com/nao1215/sqly/commit/be66492))
* **Testing**: Improved test coverage for clear command ([ce1b226](https://github.com/nao1215/sqly/commit/ce1b226), [d6f24e4](https://github.com/nao1215/sqly/commit/d6f24e4))

## [v0.14.1](https://github.com/nao1215/sqly/compare/v0.14.0...v0.14.1) (2025-09-23)

### New Features
* **Directory Import**: Add support for importing entire directories containing supported files ([021feb8](https://github.com/nao1215/sqly/commit/021feb8))
  - Automatically detect and import all CSV, TSV, LTSV, and Excel files (including compressed versions) from directories
  - Support for mixing files and directories in the same command (e.g., `sqly file1.csv ./data_dir file2.tsv`)
  - Enhanced `.import` command in interactive shell to accept both files and directories
  - Batch import functionality for efficient processing of multiple files

### Enhancements
* **CLI Interface**: Expanded command-line argument parsing to accept directory paths
  - Updated usage examples and help text to demonstrate directory import functionality
  - Improved file discovery and processing for directory-based imports
* **Interactive Shell**: Enhanced `.import` command with directory support
  - Displays summary of successfully imported tables from directories
  - Maintains backward compatibility with single file imports
* **File Processing**: Improved bulk import operations
  - Enhanced error handling for directory traversal and file processing
  - Better feedback for batch import operations

### Documentation
* **README Updates**: Comprehensive documentation updates across all languages
  - Added directory import examples and usage patterns
  - Updated help command descriptions and CLI usage information
  - Enhanced documentation in 7 languages (EN, JA, ES, FR, KO, RU, ZH-CN)

### Technical Improvements
* **Architecture**: Enhanced filesql adapter and interactor layers
  - New `DirectoryImporter` functionality in `interactor/filesql.go`
  - Comprehensive test coverage for directory import features
  - Updated dependency injection configuration for new functionality
* **Testing**: Added extensive test suite for directory import functionality
  - New test cases in `interactor/filesql_test.go` covering various directory scenarios
  - Enhanced shell extension tests for mixed file/directory imports
  - Updated golden file tests to reflect new functionality

### Migration Notes
* **For Users**: No breaking changes - all existing functionality remains identical
  - Directory import is purely additive functionality
  - All existing file-based commands continue to work as before
  - Enhanced functionality available immediately without configuration
* **For Developers**: New directory import APIs available
  - Extended `FileSQLAdapter` interface with directory import methods
  - New use case layer functionality for batch file processing

## [v0.14.0](https://github.com/nao1215/sqly/compare/v0.13.0...v0.14.0) (2025-09-23)

### New Features
* **CTE Support**: Add support for Common Table Expressions (WITH clauses)
  - Enable complex queries and recursive operations using CTE syntax
  - Full SQLite CTE functionality available for all supported file formats
  - Enhanced SQL capabilities for advanced data analysis workflows

### Breaking Changes
* **Dependencies**: Upgrade `github.com/olekukonko/tablewriter` from v0.0.5 to v1.1.0
  - Migrate to new functional options API pattern
  - Update all table rendering components to use new API
  - Maintain exact backward compatibility in output formatting

### Enhancements
* **Table Rendering**: Improved table output quality and performance
  - Enhanced numeric column detection for better right-alignment
  - Improved ASCII table formatting with consistent borders
  - Fixed markdown table cell escaping for proper rendering of `|` characters
  - Better error handling with proper error propagation instead of silent failures
* **Code Quality**: Comprehensive error handling improvements
  - All table operations now return proper errors instead of logging silently
  - Enhanced error messages with context using `fmt.Errorf` wrapping
  - Removed unnecessary logging dependencies in favor of error propagation

### Technical Improvements
* **Architecture**: Updated dependency constraints and module management
  - Added support for new tablewriter sub-packages in `.go-arch-lint.yml`
  - Updated `go.mod` with new tablewriter v1.1.0 and dependencies
  - Maintained clean architecture boundaries with proper error handling
* **Testing**: Enhanced test coverage for new functionality
  - Added comprehensive unit tests for `getColumnData()` and `isAllNumeric()` helper functions
  - Updated existing tests to handle new error return patterns
  - All tests passing with new tablewriter API
* **Documentation**: Updated README files across all languages
  - Added CTE support information to feature lists
  - Replaced "Powered by filesql" section with concise "Libraries Used" section
  - Updated documentation in 7 languages (EN, JA, ES, FR, KO, RU, ZH-CN)

### Bug Fixes
* **Numeric Detection**: Improved column type detection accuracy
  - Removed redundant pattern matching that caused false positives
  - Enhanced `isAllNumeric()` function using `strconv.ParseFloat()` for robust validation
  - Fixed over-broad string matching that misclassified columns like "paid_at" as numeric

### Migration Notes
* **For Users**: No changes to command-line interface or functionality
  - All existing commands, features, and workflows remain identical
  - CTE support is automatically available - no configuration required
  - Table output formatting maintains exact compatibility
* **For Developers**: Updated tablewriter dependency and error handling
  - New dependency: `github.com/olekukonko/tablewriter v1.1.0`
  - Table printing methods now return errors that should be handled
  - Enhanced error propagation patterns throughout codebase

## [v0.13.0](https://github.com/nao1215/sqly/compare/v0.12.2...v0.13.0) (2025-09-19)

### Breaking Changes
* **Dependencies**: Migrate from `c-bata/go-prompt` to `github.com/nao1215/prompt`
  - Replace unmaintained `c-bata/go-prompt` library with modern `nao1215/prompt`
  - Addresses critical stability issues including divide-by-zero panics and memory leaks
  - Improved cross-platform compatibility and better terminal handling

### Enhancements
* **Interactive Shell**: Enhanced prompt functionality and user experience
  - Maintained full compatibility with existing shell features (completion, history, commands)
  - Improved terminal input handling with better cursor control
  - Support for multiline input with enhanced editing capabilities
  - Fixed display issues with extra newlines after user input
  - Updated color themes and visual consistency

### Technical Improvements
* **Architecture**: Updated dependency management and architecture constraints
  - Updated `.go-arch-lint.yml` to reflect new prompt library dependency
  - Maintained clean architecture boundaries and dependency injection patterns
  - All existing tests pass with new prompt implementation
* **Code Quality**: Improved error handling and input processing
  - Enhanced input sanitization with `strings.TrimSpace()` for reliable parsing
  - Added terminal control sequences for optimal display behavior
  - Removed legacy workarounds for `c-bata/go-prompt` bugs
* **Testing**: Comprehensive test coverage maintained
  - All shell functionality tests updated and passing
  - Completion system tests adapted to new prompt library API
  - Cross-platform compatibility verified

### Bug Fixes
* **Shell Display**: Fix unwanted newlines appearing after user input
  - Resolved extra blank lines that appeared between input and output
  - Improved terminal cursor positioning with ANSI escape sequences
  - Maintains clean, professional shell appearance

### Migration Notes
* **For Users**: No changes to command-line interface or functionality
  - All existing commands, features, and workflows remain identical
  - No configuration changes required
* **For Developers**: Updated prompt library dependency
  - New dependency: `github.com/nao1215/prompt v0.0.1`
  - Removed dependency: `github.com/c-bata/go-prompt`
  - Internal API changes are fully abstracted from public interfaces

## [v0.12.2](https://github.com/nao1215/sqly/compare/v0.12.1...v0.12.2) (2025-09-17)

### Bug Fixes
* **Table Names**: Fix SQL syntax errors caused by special characters in filenames ([#153](https://github.com/nao1215/sqly/pull/153))
  - Automatically sanitize table names by replacing problematic characters (hyphens, dots, special chars) with underscores
  - Example: `bug-syntax-error.csv` now creates table `bug_syntax_error` instead of failing with syntax error
  - Added comprehensive test coverage for filename sanitization edge cases

### Documentation
* **README**: Update all localized README files with table name sanitization information
  - Added explanations in English, Japanese, Korean, Russian, Chinese, Spanish, and French
  - Clarified that special characters in filenames are automatically replaced with underscores
  - Provided clear examples of filename → table name conversion

### Technical Improvements
* **Testing**: Enhanced test suite for filename edge cases
  - Added tests for files with hyphens, dots, and special characters
  - Verified cross-platform compatibility of table name generation
  - Ensured deterministic table naming behavior

## [v0.12.1](https://github.com/nao1215/sqly/compare/v0.12.0...v0.12.1) (2025-09-06)

### Bug Fixes
* **Completion**: Fix shell completion functionality that was preventing file discovery ([066ea6a](https://github.com/nao1215/sqly/commit/066ea6a))
  - Fixed hidden directory skipping issue in file path completion
  - Completion now properly discovers all importable files recursively
  - Improved completion performance with efficient directory traversal
* **Windows**: Fix Windows compatibility issues in tests ([cc11ab6](https://github.com/nao1215/sqly/commit/cc11ab6))
  - Fixed directory cleanup issues in Windows test environments
  - Added proper directory restoration patterns for cross-platform compatibility
* **Testing**: Add ORDER BY clauses to SQL queries for deterministic test results ([e0fe515](https://github.com/nao1215/sqly/commit/e0fe515))
  - Ensures consistent test results across different platforms and SQLite versions

### Enhancements
* **Shell**: Add Windows path separator support in completion system ([066ea6a](https://github.com/nao1215/sqly/commit/066ea6a))
  - Support for backslash (`\`) path separators on Windows
  - Enhanced path pattern recognition for Windows-style paths (`.\`, `..\`, `C:\`)
* **Code Quality**: Improve error handling and remove unused parameters ([066ea6a](https://github.com/nao1215/sqly/commit/066ea6a))
  - All lint issues resolved
  - Better error propagation in file system operations

### Technical Improvements
* **Completion System**: Optimize file completion algorithm
  - Recursive directory walking with proper hidden file handling  
  - Cross-platform path normalization with `filepath.ToSlash()`
  - Efficient filtering of importable file types
* **Test Coverage**: Maintain high test coverage (36.2% for shell package)
  - All existing tests pass on both Unix and Windows platforms
  - Enhanced test stability with deterministic SQL query ordering

## [v0.12.0](https://github.com/nao1215/sqly/compare/v0.9.0...v0.12.0) (2025-01-09)

### Major Changes
* **BREAKING**: Remove JSON file format support in favor of filesql integration ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))
* **Integration**: Migrate to filesql library for enhanced performance and compressed file support ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))
* **Performance**: Implement bulk insert operations with transaction batching for faster file processing
* **Compression**: Add native support for compressed files (.gz, .bz2, .xz, .zst) ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))
* **Dependencies**: Remove mattn/go-sqlite3 (CGO) in favor of pure Go modernc.org/sqlite ([d5649f9](https://github.com/nao1215/sqly/commit/d5649f9))

### New Features
* **Shell Commands**: Add .cd helper command for directory navigation ([d49e5a7](https://github.com/nao1215/sqly/commit/d49e5a7))
* **Shell Commands**: Add .ls helper command to list directory contents ([d49e5a7](https://github.com/nao1215/sqly/commit/d49e5a7))
* **Shell Commands**: Add .pwd helper command to show current working directory ([8812122](https://github.com/nao1215/sqly/commit/8812122))
* **Interactive**: Display current output mode in shell prompt ([a0f7047](https://github.com/nao1215/sqly/commit/a0f7047))
* **Type Detection**: Automatic column data type detection ensures proper numeric sorting
* **Go Version**: Add support for Go 1.24 ([a4c7512](https://github.com/nao1215/sqly/commit/a4c7512))

### Architecture Improvements
* **Clean Architecture**: Refactor codebase to follow Clean Architecture principles more strictly ([5a4bb96](https://github.com/nao1215/sqly/commit/5a4bb96))
* **Architecture Linting**: Add go-arch-lint for architectural boundary enforcement ([35c7e8f](https://github.com/nao1215/sqly/commit/35c7e8f))
* **Domain Model**: Convert parts of domain model to Value Objects for better encapsulation ([5c8ec2d](https://github.com/nao1215/sqly/commit/5c8ec2d))
* **Dependency Injection**: Improve usecase interfaces and add mock code for testing ([ee92763](https://github.com/nao1215/sqly/commit/ee92763))
* **Package Structure**: Refactor shell package for better organization ([101163f](https://github.com/nao1215/sqly/commit/101163f))

### Documentation & Developer Experience
* **LLM Integration**: Add Claude Code, Cursor, and GitHub Copilot configuration files ([2ceefa0](https://github.com/nao1215/sqly/commit/2ceefa0))
* **Documentation**: Create comprehensive developer documentation ([c368778](https://github.com/nao1215/sqly/commit/c368778))
* **GitHub Pages**: Set up documentation site at https://nao1215.github.io/sqly/ ([a061c49](https://github.com/nao1215/sqly/commit/a061c49))
* **Internationalization**: Add README translations for multiple languages ([b676409](https://github.com/nao1215/sqly/commit/b676409)):
  - Spanish (es)
  - French (fr) 
  - Japanese (ja)
  - Korean (ko)
  - Russian (ru)
  - Chinese Simplified (zh-cn)

### GitHub Actions & Automation
* **AI Assistance**: Add Claude Code Review workflow ([0a86dd2](https://github.com/nao1215/sqly/commit/0a86dd2))
* **AI Assistance**: Add Claude PR Assistant workflow ([5b8be74](https://github.com/nao1215/sqly/commit/5b8be74))

### Dependencies
* Bump github.com/sergi/go-diff from 1.3.1 to 1.4.0 ([dd44965](https://github.com/nao1215/sqly/commit/dd44965))
* Bump github.com/spf13/pflag from 1.0.6 to 1.0.10 ([0763386](https://github.com/nao1215/sqly/commit/0763386))
* Bump github.com/stretchr/testify from 1.10.0 to 1.11.1 ([f9fe0e5](https://github.com/nao1215/sqly/commit/f9fe0e5))
* Bump github.com/xuri/excelize/v2 from 2.9.0 to 2.9.1 ([9cbb0ff](https://github.com/nao1215/sqly/commit/9cbb0ff))
* Bump go.uber.org/mock from 0.5.1 to 0.5.2 ([c50a81f](https://github.com/nao1215/sqly/commit/c50a81f))
* Bump golang.org/x/net from 0.33.0 to 0.36.0 ([3ff5306](https://github.com/nao1215/sqly/commit/3ff5306))
* Bump modernc.org/sqlite from 1.34.5 to 1.36.1 ([b03c0d2](https://github.com/nao1215/sqly/commit/b03c0d2))
* Bump github.com/google/go-cmp from 0.6.0 to 0.7.0 ([38d711c](https://github.com/nao1215/sqly/commit/38d711c))

### Re-added and New Input Formats
- **JSON/JSONL Support**: JSON and JSONL (JSON Lines) file format support has been re-added as input via the filesql library. Data is stored in a single `data` column; use SQLite's `json_extract()` to query individual fields
- **Parquet Support**: Parquet file format is now supported as input

### Breaking Changes
- **CLI Flag Removed**: The `--json` output flag has been removed (output formats: table, CSV, TSV, LTSV, Excel, Markdown)
- **Output Format**: Numeric formatting may differ slightly due to improved type detection
- **Dependencies**: Removed CGO dependency (mattn/go-sqlite3) in favor of pure Go implementation

### Migration Guide
- **For JSON users**: JSON/JSONL files are now supported again as input. Use `json_extract()` to query fields from the `data` column
- **For developers**: Update any code that relied on the `--json` output flag
- **Benefits**: Enjoy improved performance, compressed file support, JSON/JSONL/Parquet input, and better type handling

## [v0.9.0](https://github.com/nao1215/sqly/compare/v0.8.1...v0.9.0) (2025-02-03)

* Add architecture linter [#87](https://github.com/nao1215/sqly/pull/87) ([nao1215](https://github.com/nao1215))
* Reduce dependency and add unit tests for interactor [#86](https://github.com/nao1215/sqly/pull/86) ([nao1215](https://github.com/nao1215))
* Add usecase interface and mock code [#85](https://github.com/nao1215/sqly/pull/85) ([nao1215](https://github.com/nao1215))
* Bump github.com/spf13/pflag from 1.0.5 to 1.0.6 [#84](https://github.com/nao1215/sqly/pull/84) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/net from 0.30.0 to 0.33.0 [#83](https://github.com/nao1215/sqly/pull/83) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.4 to 1.34.5 [#82](https://github.com/nao1215/sqly/pull/82) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-colorable from 0.1.13 to 0.1.14 [#81](https://github.com/nao1215/sqly/pull/81) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.3 to 1.34.4 [#80](https://github.com/nao1215/sqly/pull/80) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/crypto from 0.28.0 to 0.31.0 [#79](https://github.com/nao1215/sqly/pull/79) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.1 to 1.34.3 [#78](https://github.com/nao1215/sqly/pull/78) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.33.1 to 1.34.1 [#76](https://github.com/nao1215/sqly/pull/76) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.17.0 to 1.18.0 [#75](https://github.com/nao1215/sqly/pull/75) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.23 to 1.14.24 [#73](https://github.com/nao1215/sqly/pull/73) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/xuri/excelize/v2 from 2.8.1 to 2.9.0 [#74](https://github.com/nao1215/sqly/pull/74) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.22 to 1.14.23 [#70](https://github.com/nao1215/sqly/pull/70) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.33.0 to 1.33.1 [#72](https://github.com/nao1215/sqly/pull/72) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.32.0 to 1.33.0 [#71](https://github.com/nao1215/sqly/pull/71) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add go 1.23 in unit test coverage [#69](https://github.com/nao1215/sqly/pull/69) ([nao1215](https://github.com/nao1215))
* Bump modernc.org/sqlite from 1.31.1 to 1.32.0 [#68](https://github.com/nao1215/sqly/pull/68) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.2 to 1.31.1 [#67](https://github.com/nao1215/sqly/pull/67) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.1 to 1.30.2 [#66](https://github.com/nao1215/sqly/pull/66) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.0 to 1.30.1 [#65](https://github.com/nao1215/sqly/pull/65) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump goreleaser/goreleaser-action from 5 to 6 [#64](https://github.com/nao1215/sqly/pull/64) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.10 to 1.30.0 [#63](https://github.com/nao1215/sqly/pull/63) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.9 to 1.29.10 [#62](https://github.com/nao1215/sqly/pull/62) ([dependabot[bot]](https://github.com/apps/dependabot))
* Update project config [#61](https://github.com/nao1215/sqly/pull/61) ([nao1215](https://github.com/nao1215))
* Bump github.com/fatih/color from 1.16.0 to 1.17.0 [#60](https://github.com/nao1215/sqly/pull/60) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.8 to 1.29.9 [#59](https://github.com/nao1215/sqly/pull/59) ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.8.1](https://github.com/nao1215/sqly/compare/v0.8.0...v0.8.1) (2024-05-01)

* Introduce homebrew [#58](https://github.com/nao1215/sqly/pull/58) ([nao1215](https://github.com/nao1215))

## [v0.8.0](https://github.com/nao1215/sqly/compare/v0.7.0...v0.8.0) (2024-05-01)

* Change SQLite3 driver from mattn/go-sqlite3 to modernc.org/sqlite [#57](https://github.com/nao1215/sqly/pull/57) ([nao1215](https://github.com/nao1215))
* Add benchmark [#56](https://github.com/nao1215/sqly/pull/56) ([nao1215](https://github.com/nao1215))
* Add unit test for excel [#55](https://github.com/nao1215/sqly/pull/55) ([nao1215](https://github.com/nao1215))

## [v0.7.0](https://github.com/nao1215/sqly/compare/v0.6.5...v0.7.0) (2024-04-30)

* Bump golang.org/x/net from 0.21.0 to 0.23.0 [#54](https://github.com/nao1215/sqly/pull/54) ([dependabot[bot]](https://github.com/apps/dependabot))
* Support Microsoft Excel™ (XLAM / XLSM / XLSX / XLTM / XLTX) [#53](https://github.com/nao1215/sqly/pull/53) ([nao1215](https://github.com/nao1215))

## [v0.6.5](https://github.com/nao1215/sqly/compare/v0.6.4...v0.6.5) (2024-04-29)

## [v0.6.4](https://github.com/nao1215/sqly/compare/v0.5.2...v0.6.4) (2024-04-29)

* Bump goreleaser/goreleaser-action from 2 to 5 [#50](https://github.com/nao1215/sqly/pull/50) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/checkout from 3 to 4 [#52](https://github.com/nao1215/sqly/pull/52) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/setup-go from 3 to 5 [#51](https://github.com/nao1215/sqly/pull/51) ([dependabot[bot]](https://github.com/apps/dependabot))
* Maintain dependencies for GitHub Actions [#49](https://github.com/nao1215/sqly/pull/49) ([nao1215](https://github.com/nao1215))
* Introduce numerical sorting [#48](https://github.com/nao1215/sqly/pull/48) ([nao1215](https://github.com/nao1215))
* Fix issue 43: Panic when importing json table with numeric field. [#47](https://github.com/nao1215/sqly/pull/47) ([nao1215](https://github.com/nao1215))
* Fix issue 42 (bug): Panic when json field is null [#46](https://github.com/nao1215/sqly/pull/46) ([nao1215](https://github.com/nao1215))
* Update project config [#45](https://github.com/nao1215/sqly/pull/45) ([nao1215](https://github.com/nao1215))
* Introduce octocov [#44](https://github.com/nao1215/sqly/pull/44) ([nao1215](https://github.com/nao1215))
* Bump github.com/google/wire from 0.5.0 to 0.6.0 [#41](https://github.com/nao1215/sqly/pull/41) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.19 to 1.14.22 [#40](https://github.com/nao1215/sqly/pull/40) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.18 to 1.14.19 [#37](https://github.com/nao1215/sqly/pull/37) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.15.0 to 1.16.0 [#36](https://github.com/nao1215/sqly/pull/36) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.17 to 1.14.18 [#35](https://github.com/nao1215/sqly/pull/35) ([dependabot[bot]](https://github.com/apps/dependabot))
* (auto merged) Bump github.com/google/go-cmp from 0.5.9 to 0.6.0 [#34](https://github.com/nao1215/sqly/pull/34) ([dependabot[bot]](https://github.com/apps/dependabot))
* Add automerged workflows [#33](https://github.com/nao1215/sqly/pull/33) ([nao1215](https://github.com/nao1215))
* Bump github.com/mattn/go-sqlite3 from 1.14.16 to 1.14.17 [#32](https://github.com/nao1215/sqly/pull/32) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/nao1215/gorky from 0.2.0 to 0.2.1 [#31](https://github.com/nao1215/sqly/pull/31) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.14.1 to 1.15.0 [#30](https://github.com/nao1215/sqly/pull/30) ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.13.0 to 1.14.1 [#29](https://github.com/nao1215/sqly/pull/29) ([dependabot[bot]](https://github.com/apps/dependabot))
* Change golden package import path [#28](https://github.com/nao1215/sqly/pull/28) ([nao1215](https://github.com/nao1215))

## [v0.5.2](https://github.com/nao1215/sqly/compare/v0.5.1...v0.5.2) (2022-11-27)

* add unit test for infra package [#27](https://github.com/nao1215/sqly/pull/27) ([nao1215](https://github.com/nao1215))
* Add basic unit test for shell [#26](https://github.com/nao1215/sqly/pull/26) ([nao1215](https://github.com/nao1215))
* Add unit test for model package [#24](https://github.com/nao1215/sqly/pull/24) ([nao1215](https://github.com/nao1215))
* Bump github.com/google/go-cmp from 0.2.0 to 0.5.9 [#25](https://github.com/nao1215/sqly/pull/25) ([dependabot[bot]](https://github.com/apps/dependabot))
* Change golden test package from goldie to golden and more [#23](https://github.com/nao1215/sqly/pull/23) ([nao1215](https://github.com/nao1215))
* Add unit test for argument paser [#21](https://github.com/nao1215/sqly/pull/21) ([nao1215](https://github.com/nao1215))

## [v0.5.1](https://github.com/nao1215/sqly/compare/v0.5.0...v0.5.1) (2022-11-19)

* Add sqlite3 syntax completion [#22](https://github.com/nao1215/sqly/pull/22) ([nao1215](https://github.com/nao1215))

## [v0.5.0](https://github.com/nao1215/sqly/compare/v0.4.0...v0.5.0) (2022-11-13)

* Feat dump tsv ltsv json [#20](https://github.com/nao1215/sqly/pull/20) ([nao1215](https://github.com/nao1215))
* Add featuer thar print date by markdown table format [#19](https://github.com/nao1215/sqly/pull/19) ([nao1215](https://github.com/nao1215))
* Feat import ltsv [#18](https://github.com/nao1215/sqly/pull/18) ([nao1215](https://github.com/nao1215))

## [v0.4.0](https://github.com/nao1215/sqly/compare/v0.3.1...v0.4.0) (2022-11-13)

* Feat import tsv [#17](https://github.com/nao1215/sqly/pull/17) ([nao1215](https://github.com/nao1215))

## [v0.3.1](https://github.com/nao1215/sqly/compare/v0.3.0...v0.3.1) (2022-11-11)

* Fix panic bug when import file that is without extension [#15](https://github.com/nao1215/sqly/pull/15) ([nao1215](https://github.com/nao1215))

## [v0.3.0](https://github.com/nao1215/sqly/compare/v0.2.1...v0.3.0) (2022-11-10)

* Feat import json [#14](https://github.com/nao1215/sqly/pull/14) ([nao1215](https://github.com/nao1215))
* Fix input delays when increasing records [#13](https://github.com/nao1215/sqly/pull/13) ([nao1215](https://github.com/nao1215))

## [v0.2.1](https://github.com/nao1215/sqly/compare/v0.2.0...v0.2.1) (2022-11-09)

* Add header command [#12](https://github.com/nao1215/sqly/pull/12) ([nao1215](https://github.com/nao1215))

## [v0.2.0](https://github.com/nao1215/sqly/compare/v0.1.1...v0.2.0) (2022-11-09)

* Fixed a display collapse problem when multiple lines are entered [#11](https://github.com/nao1215/sqly/pull/11) ([nao1215](https://github.com/nao1215))

## [v0.1.1](https://github.com/nao1215/sqly/compare/v0.1.0...v0.1.1) (2022-11-07)

* Fixed a bug that caused SQL to fail if there was a trailing semicolon [#10](https://github.com/nao1215/sqly/pull/10) ([nao1215](https://github.com/nao1215))

## [v0.1.0](https://github.com/nao1215/sqly/compare/v0.0.11...v0.1.0) (2022-11-07)

* Add move cursor function in intaractive shell [#9](https://github.com/nao1215/sqly/pull/9) ([nao1215](https://github.com/nao1215))

## [v0.0.11](https://github.com/nao1215/sqly/compare/v0.0.10...v0.0.11) (2022-11-06)

* Fixed a bug in which the wrong arguments were used [#8](https://github.com/nao1215/sqly/pull/8) ([nao1215](https://github.com/nao1215))

## [v0.0.10](https://github.com/nao1215/sqly/compare/v0.0.9...v0.0.10) (2022-11-06)

* Added CSV output mode [#7](https://github.com/nao1215/sqly/pull/7) ([nao1215](https://github.com/nao1215))

## [v0.0.9](https://github.com/nao1215/sqly/compare/v0.0.7...v0.0.9) (2022-11-06)

## [v0.0.7](https://github.com/nao1215/sqly/compare/v0.0.6...v0.0.7) (2022-11-06)

* Improve execute query [#6](https://github.com/nao1215/sqly/pull/6) ([nao1215](https://github.com/nao1215))

## [v0.0.6](https://github.com/nao1215/sqly/compare/v0.0.5...v0.0.6) (2022-11-05)

## [v0.0.5](https://github.com/nao1215/sqly/compare/v0.0.4...v0.0.5) (2022-11-05)

* Add history usecase, repository, infra. sqly manage history by sqlite3 [#5](https://github.com/nao1215/sqly/pull/5) ([nao1215](https://github.com/nao1215))
* Add function that execute select query [#4](https://github.com/nao1215/sqly/pull/4) ([nao1215](https://github.com/nao1215))

## [v0.0.4](https://github.com/nao1215/sqly/compare/v0.0.3...v0.0.4) (2022-11-05)

## [v0.0.3](https://github.com/nao1215/sqly/compare/v0.0.2...v0.0.3) (2022-11-05)

* Add import command [#3](https://github.com/nao1215/sqly/pull/3) ([nao1215](https://github.com/nao1215))

## [v0.0.2](https://github.com/nao1215/sqly/compare/v0.0.1...v0.0.2) (2022-11-05)

* Add .tables command [#2](https://github.com/nao1215/sqly/pull/2) ([nao1215](https://github.com/nao1215))
* Add .exit/.help command and history manager [#1](https://github.com/nao1215/sqly/pull/1) ([nao1215](https://github.com/nao1215))

## [v0.0.1](https://github.com/nao1215/sqly/compare/dbf99896449e...v0.0.1) (2022-11-03)
