# CHANGELOG

## [Unreleased]

### Bug Fixes
* Multi-line Interactive SQL: the shell now buffers a SQL statement across lines and submits on Enter only when it ends with `;`, so a typed or pasted multi-line statement (for example `SELECT ... UNION ALL SELECT ...;`) runs once instead of executing each line separately. Dot-commands stay single-line, and pressing Enter on a blank line force-runs a query typed without `;`.
* Idempotent SQLite Driver Registration: `config.InitSQLite3()` now guards driver registration with a package-level `sync.Once` instead of a function-local one, so calling it more than once no longer panics with `sql: Register called twice for driver sqlite3`.
* Prefix-Scoped Import Completion: `.import` tab completion now reads only the directory named by the typed path prefix instead of walking the whole working tree on every keystroke. Directories are offered with a trailing slash so the path can be completed one level at a time, keeping latency proportional to the targeted subtree rather than repository size.
* Space-Safe Import Completion: `.import` tab completion now backslash-escapes spaces and shell-special characters, so accepting a path like `my data.csv` inserts `my\ data.csv` and reaches `.import` as a single argument. Escaping (not quoting) is used so the suggestion still prefix-matches the typed word.
* Completion Into Space-Containing Directories: `.import` tab completion now descends into a directory whose name contains a space. The escaped prefix (for example `my\ dir/`) is decoded to read the real directory while the escaped form is kept on each suggestion, so nested files complete and still round-trip through the command parser.
* Compare Input Order: `--compare` without `--compare-tables` now keeps the left/right direction in the order the inputs were given on the command line, instead of sorting the table names alphabetically.
* Typed JSON Mode Shell UX: switching to `.mode json-typed`/`.mode ndjson-typed` now shows the typed mode name in the prompt label and the `.mode` current-mode banner instead of the plain `json`/`ndjson`, and `.mode` lists both typed variants.
* Content-Aware Import Cache Key: `--cache` now keys invalidation on each input file's path, size, and a SHA-256 content hash instead of path, size, and modification time. A source rewritten in place with different but same-length content and its original mtime restored is now detected and the cache rebuilt, so a warm run can no longer return stale rows for a modified file.
* Clean Ctrl-D Exit: pressing Ctrl-D (EOF) in the interactive shell now exits cleanly like `.exit` instead of printing a raw `EOF` line. Both EOF spellings the prompt library reports (Ctrl-D on an empty line and a closed input stream) are treated as a normal termination.
* Symlink-Resolved System-Path Guard: import path validation now rejects a symlink whose canonical target is a blocked system location (such as a link to `/etc/hosts`), not only a directly typed system path. It also normalizes the macOS `/private` prefix, while standard Unix pseudo-files (`/dev/stdin`, `/proc/self/fd/*`) keep importing.

### Dependencies
* Prompt: upgrade `github.com/nao1215/prompt` to v0.0.7 for the `WithIsComplete` multiline submit predicate and the `WithWordEscape` option that lets completion treat backslash-escaped whitespace as part of a word.

## [v0.24.0](https://github.com/nao1215/sqly/compare/v0.23.0...v0.24.0) (2026-06-06)

### Features
* Opt-In Import Cache: `--cache PATH` snapshots the imported tables to a standalone SQLite file so a repeated run against unchanged inputs reloads from it instead of re-parsing large source files. The cache key is each input file's path, size, and modification time (expanded recursively for directories), so it invalidates automatically when a source changes. `--cache-clear` forces a cold rebuild, and a cache that is unavailable or unwritable falls back cleanly to a cold import with a warning instead of failing the query. Caching is skipped for `--stdin` datasets and for ACH/Fedwire inputs (whose write-back needs the live import registry).
* CLI-First Profile Workflow: a top-level `--profile` mode prints a machine-readable data-quality report for every imported table, so users who received unfamiliar data can understand it before writing SQL. Each report covers per-table row and column counts and, per column, null and blank counts, distinct and numeric counts, and safe warnings for mixed numeric/non-numeric values, null-like placeholder text (`NULL`, `N/A`, ...), and leading or trailing whitespace. JSON is the default automation contract; `--profile-format text` prints a human-readable summary. It works for files, directories, stdin datasets, and multi-table imports.
* CLI-First Compare Workflow: a top-level `--compare` mode diffs two imported tables without entering the interactive shell. It reports schema differences (columns unique to each side and type changes), a row-count delta, and—when `--compare-key COL` is given—keyed row differences (added, removed, and modified rows). JSON is the default automation contract; `--compare-format text` prints a human-readable summary. The two tables are the pair of imported tables, or an explicit `--compare-tables "left,right"`. Clear errors are returned for a missing key column, a non-unique key, a missing named table, or an ambiguous import that did not produce exactly two tables.
* Native ACH and Fedwire Write-Back: `--save`/`--save-dir` (and interactive `.save`) now reconstruct a complete `.ach`/`.fed` file from its imported table set after in-session `UPDATE`s, using filesql's native ACH/Fedwire writers. The whole related table set is rewritten together into one valid file, and write-back validates that the required companion tables (for ACH, the file-header, batches, and entries tables) are present, failing with an explicit error when the set is incomplete. The single-table `--output`/`.dump` path still rejects `.ach`/`.fed`, since those formats require a coordinated record set. Adding or removing records is not supported by the native reconstruction; only updates to existing rows are persisted.
* Typed JSON Output Contract: `--json-typed` and `--ndjson-typed` (and the matching `.mode json-typed`/`.mode ndjson-typed`) opt query output into a typed contract that emits native JSON scalars instead of strings. A canonical JSON number becomes a number, `true`/`false` become booleans, and a SQL NULL becomes `null`; a large integer is preserved verbatim so it never regresses into scientific notation, while a value with a leading zero such as `007` stays a string. The default `--json`/`--ndjson` keep the legacy string contract for compatibility. `--inspect --json-typed` applies the same contract to the report's sample rows so the schema metadata and sample payloads agree.

### Bug Fixes
* Directory-Imported Financial Files: an ACH/Fedwire file picked up by a directory import is no longer reconstructed as a whole-set write-back target; like every other directory-imported table it is rejected for write-back with a clear error. `--cache` now also detects ACH/Fedwire files nested inside a directory argument and skips caching, so a warm reload cannot leave their write-back registry unpopulated.
* Compare Distinguishes NULL From Empty: `--compare --compare-key` now reports a change between a SQL NULL and an empty string and emits a NULL cell as JSON `null` rather than `""`, so keyed row differences are accurate for nullable columns.
* Profile Numeric Counting: `--profile` no longer counts Go-specific float spellings (hexadecimal floats like `0x1p4`, underscore-separated digits like `1_000`) as numeric values, keeping the numeric count and mixed-type warning aligned with ordinary data.
* Clearer Output-Mode Conflicts: an `--inspect`/`--compare`/`--profile` conflict with a typed JSON flag now names the flag the user actually passed (`--json-typed`/`--ndjson-typed`) instead of the base mode.

## [v0.23.0](https://github.com/nao1215/sqly/compare/v0.22.0...v0.23.0) (2026-06-02)

### Bug Fixes
* Literal Dotted Object Names With A Schema Prefix: `.schema`, `.describe`, `.header`, and `.dump` now reach a table or view whose quoted literal name begins with `main.` or `temp.` (for example `CREATE TABLE "main.x"` or `CREATE VIEW "temp.v"`). Because the shell strips the quotes the user typed, a name is read as a schema qualifier only when no object literally carries it. `.tables` prints such a name quoted so it pastes back into these commands.
* Long-Form Compression Aliases In Stacked Suffixes: `--output` and `.dump` now reject a destination that stacks a long-form compression alias on another codec (for example `out.parquet.gzip.zst` or `out.tsv.gzip.zst`). Previously sqly applied only the outermost codec and wrote CSV bytes under a name that advertised a different format. `.gzip`, `.zstd`, and `.bzip2` are now recognized as compression suffixes when detecting stacked suffixes and when seeing through compression to an input-only `.ach`/`.fed` format.
* Leading Empty Statement In Direct --sql: direct `--sql` now drops a leading empty statement (a bare `;`) before classifying the input, so `;SELECT ...` returns its rows, `;UPDATE ...` reports its affected count and triggers write-back, `;PRAGMA`/`;CREATE` apply their effect, and `;ATTACH ...` is still rejected as unsupported. Previously the leading `;` caused the real statement to be classified as a no-rowset statement, discarding a query, dropping a data change, or bypassing unsupported-statement validation.
* Write-Back Skips Unchanged Imported Tables: `.save`, `.save DIR`, and the non-interactive `--save`/`--save-dir` runs now persist only a file-backed imported table whose content changed. A session that touched only a TEMP or SQL-created scratch table, or that made net-zero edits that cancel out, no longer rewrites an untouched source, fails on an unwritable JSONL import, or aborts on a scratch table that has no source file. Each import records a content fingerprint, and write-back compares against it instead of relying on a coarse session-wide changed flag.

### Documentation
* Add README demos for cross-format JOIN (Parquet and CSV), --output format conversion (JSON, Parquet, Excel), and directory import across formats, recorded with VHS.

## [v0.22.0](https://github.com/nao1215/sqly/compare/v0.21.0...v0.22.0) (2026-06-01)

### Breaking Changes
* Direct --sql Runs One Statement: direct `--sql` (and `--sql --output`) now rejects multi-statement input instead of silently running every statement and keeping only the last result set.
* Save Mode Rejects PRAGMA: a non-interactive `--save`/`--save-dir` run now rejects a setter, command, or rowset PRAGMA, since a PRAGMA side effect lives only in the in-memory session and has no file write-back representation.
* Nested Compression Suffixes Rejected: `--output` and `.dump` reject a destination that stacks more than one compression suffix (for example `out.csv.gz.zst` or `fake.parquet.gz.zst`), instead of applying only the outermost codec and leaving a file whose name lies about its bytes.
* END Rejected As Transaction Control: `END` and `END TRANSACTION` are rejected as unsupported transaction control across direct `--sql`, batch stdin, and `--sql-file`, matching `BEGIN`/`COMMIT`/`ROLLBACK`/`SAVEPOINT`.

### Bug Fixes
* Helper Commands Resolve TEMP Before Main: `.schema` resolves an unqualified name against temp objects before main, so a TEMP table or view that shadows an imported table reports the live definition; `.tables` keeps both a main object and a same-named temp object instead of collapsing them.
* Literal Dotted Table Names: `.schema`, `.describe`, `.header`, and `.dump` target a SQL-created table whose quoted literal name contains a dot (for example `"a.b"`); only `main` and `temp` are treated as schema qualifiers, since ATTACH is rejected and no other schema can exist.
* TEMP Keyword Preserved: `.schema temp.NAME` emits `CREATE TEMP TABLE`/`CREATE TEMP VIEW`, re-inserting the TEMP keyword SQLite strips from the SQL it stores for a temp object.
* Paste-Safe .tables Output: `.tables` quotes identifiers that need quoting and qualifies a temp object as `temp.NAME`, so its output pastes back into SQL and helper commands; `.header` keeps the full table name when it contains spaces.
* Structured Output For .tables And .header: `.tables` and `.header` honor `.mode json` and `.mode ndjson`, emitting machine-readable rows instead of always printing an ASCII table.
* Read-Only Interactive Save: interactive `.save --force` and `.save DIR` write nothing when the session changed no table data, so a read-only session no longer rewrites sources or emits fresh exports, matching the non-interactive `--save` contract.

### Documentation
* README Version Refresh: Refresh the shell snippet and benchmark caption to the current release, correct the "not supported" list for v0.21.0 (DDL runs in-memory; transaction control, VACUUM, ATTACH/DETACH, and DCL are rejected), and add a Go test that fails when a README `sqly vX.Y.Z` string drifts from the latest CHANGELOG version.
* README Demos For Non-Interactive Flows: Add VHS demos and examples for `--inspect` (including `--inspect-sample 0` for a schema-only report), `--stdin` combined with `--sql-file`, and the write-back safety boundaries (`--save` requires `--force`; a schema change is rejected up front). The new example commands are exercised end-to-end by the shellspec suite.

## [v0.21.0](https://github.com/nao1215/sqly/compare/v0.20.0...v0.21.0) (2026-06-01)

### Breaking Changes
* Unsupported Statements Rejected Clearly: Explicit transaction control (`BEGIN`/`COMMIT`/`ROLLBACK`/`SAVEPOINT`/`RELEASE`), `VACUUM`/`VACUUM INTO`, and `ATTACH`/`DETACH DATABASE` are now rejected with a clear sqly error. sqly runs each statement in its own transaction on a single in-memory connection, so these cannot work across statements, and ATTACH would let a session read or write external SQLite files outside the import/save model.
* Write-Back Rejects Schema-Only Runs: A non-interactive `--save`/`--save-dir` run now fails up front when the SQL changes schema or runs a maintenance statement (ALTER, DROP, REINDEX, ANALYZE, CREATE/DROP of a table/view/index/trigger, including `CREATE TABLE AS SELECT`), since write-back can only persist `INSERT`/`UPDATE`/`DELETE` on imported tables. Previously such a run exited 0 and reported success while leaving the source unchanged.

### Bug Fixes
* Neutral Result Message For Non-DML: A DDL, PRAGMA, or maintenance statement now reports `statement executed successfully` instead of a misleading `affected is N row(s)` count.
* PRAGMA On The Exec Path: A setter PRAGMA (`PRAGMA user_version = 1`) and a no-row command PRAGMA (`PRAGMA incremental_vacuum`) now run successfully instead of failing with a "no records" error.
* Batch .import Under Save Flags: A batch or `--sql-file` script that imports its own input with `.import` and then modifies it is now allowed under `--save`/`--save-dir`; write-back is validated after the import runs.
* Schema-Qualified Helper Commands: `.schema`, `.describe`, `.header`, and `.dump` accept schema-qualified names such as `main.user`.
* TEMP Tables And Views In Helper Commands: `.tables` lists session-created views and TEMP tables; `.schema` prints the real `CREATE VIEW` for a view and reads the stored definition for a constrained TEMP table instead of a lossy reconstruction.
* Empty Compressed JSON And JSONL: An empty compressed JSON array (`.json.gz`) and an empty compressed JSONL file now import as a zero-row table, matching the uncompressed inputs.
* Output Destination Safety: `--output` and `.dump` strip every trailing compression suffix before checking for an input-only ACH/Fedwire extension, so a path like `out.ach.gz.zst` is rejected instead of receiving CSV bytes.
* Pseudo-File Inputs: `/dev/stdin`, `/dev/stdout`, `/dev/stderr`, and the Linux `/proc/<pid|self>/fd/*` aliases pass input-path validation and import end-to-end. An extensionless pseudo-file is staged as CSV (use `--stdin FORMAT` for another format), matching the already-allowed `/dev/fd/*`.
* LTSV Label Validation: LTSV output rejects a column name that is not a valid LTSV label (for example `foo:bar`) or that duplicates another, and LTSV import rejects a row that repeats a label, so LTSV stays round-trippable instead of silently losing values.
* Multiline CREATE TRIGGER: Batch and `--sql-file` parsing keeps a `CREATE TRIGGER ... BEGIN ... END` body as one statement instead of splitting it at the inner semicolons.

### Dependencies
* filesql: 0.13.0 → 0.14.0, which rejects a duplicate label within an LTSV record on import (the upstream root fix, replacing the temporary sqly-side check) and pulls in fileparser 0.5.2.

## [v0.20.0](https://github.com/nao1215/sqly/compare/v0.19.0...v0.20.0) (2026-06-01)

### Bug Fixes
* Valid Machine-Readable Output: `--csv` and `--tsv` stdout now go through a CSV/TSV writer, so values containing the delimiter, quotes, or newlines are quoted and stay valid when redirected or piped. `--ltsv` rejects values with a tab or newline, which LTSV cannot represent losslessly, and the LTSV file export no longer quotes the whole `label:value` token, so it round-trips. `--json` and `--ndjson` reject duplicate output column names instead of emitting ambiguous duplicate keys. `--markdown` renders an embedded newline as `<br>` so a row stays on one line.
* Direct --sql Accepts More SQLite: The direct `--sql` path strips a leading SQL comment or UTF-8 BOM before classifying a statement, matching the batch and `--sql-file` paths. It now runs `PRAGMA`, `VALUES`, `REPLACE`, transaction control (`BEGIN`/`COMMIT`/...), DDL (`CREATE`/`DROP`/...), `ATTACH`, and `ANALYZE` instead of rejecting them, and rewrites the `TABLE name` shorthand to `SELECT * FROM name`. A non-returning `WITH ... INSERT/UPDATE/DELETE` runs as DML instead of failing on the query path.
* Empty JSON And JSONL Inputs: An empty JSON array (`[]`), whitespace-only JSON, and an empty or blank-only JSONL file now import as a zero-row table with the `data` column instead of failing as an empty data source.
* Inspect And Dependent-Flag Validation: `--inspect` rejects a conflicting output mode flag such as `--csv` or `--parquet`. `--stdin-name` requires `--stdin`, `--inspect-sample` requires `--inspect`, and `--force` requires `--save`/`--save-dir`, instead of being silently ignored. A `--stdin-name` that is a SQLite keyword is rejected since it is not queryable as a bare table name, and an imported file whose name sanitizes to a keyword now warns that the table must be quoted.
* Output Destination Safety: `--output` and `.dump` resolve symlinks before comparing a destination to an imported source, so a symlink alias can no longer overwrite a source file. `.dump` now rejects a destination that aliases an imported source, pointing at `.save --force`. A destination ending with a path separator is rejected instead of becoming a hidden `.csv` file, and ACH/Fedwire destination extensions (including compressed variants) are rejected instead of receiving CSV bytes.
* Write-Back Semantics: An `EXPLAIN` of a DML statement and a zero-row DML no longer trigger write-back, since neither changes table data. A `.csv.bz2` source is rejected during preflight, before any file is truncated, because bzip2 has no writer. A run that fails during write-back keeps stdout free of the DML success count.
* Directory Import And Collisions: Re-importing a directory-sourced file directly clears the directory marker so it becomes saveable, a standalone `.import` can replace a directory-imported table, a same-source symlink alias is treated as a harmless re-import rather than a collision, and a directory re-import no longer mis-detects basename-prefix tables (for example `a.csv` and `a_b.csv`) as collisions.
* Batch Line-By-Line Parsing: A helper command after a terminated SQL statement or after a leading SQL comment is parsed and executed on its own line instead of being absorbed into the following statement.
* Input Path Validation: User files under `/dev/shm` and process-substitution paths under `/dev/fd` are no longer rejected as system directories.
* History Lock Contention: The session databases set `busy_timeout`, so two sqly processes sharing one history DB wait for a transient lock instead of disabling history with a misleading SQLITE_BUSY warning.

## [v0.19.0](https://github.com/nao1215/sqly/compare/v0.18.0...v0.19.0) (2026-06-01)

### New Features
* DML RETURNING Support: `INSERT`, `UPDATE`, and `DELETE` statements with a `RETURNING` clause now print the returned rows instead of only an affected-row count, and those rows can be exported with `--output`.

### Bug Fixes
* Explicit Empty Flag Values Rejected: `--output`, `--sql-file`, `--save-dir`, and `--stdin` now reject an explicit empty value instead of treating the flag as absent. `.import` likewise rejects an empty `--sheet`, in both the `--sheet ""` and `--sheet=` forms.
* Comment-Only SQL Files Rejected: A `--sql-file` that contains only comments now fails like an empty file, since it has no executable SQL.
* Conflicting Output Mode Flags Rejected: Passing more than one output mode flag (for example `--csv --json`) now fails instead of applying an undocumented precedence.
* Output For Non-Rowset DML: `--output` is now rejected for a DML statement that produces no rows (an `INSERT`/`UPDATE`/`DELETE` without `RETURNING`), instead of being silently ignored.
* Save Flags With sql-file On A Terminal: `--save` and `--save-dir` now work with `--sql-file` even when stdin is a terminal.
* Stdin Routing: `--sql-file` now rejects non-empty piped stdin instead of silently dropping it, pointing at `--stdin` for dataset input. A `--stdin` dataset run with no query now fails instead of importing and discarding the data.
* UTF-8 BOM In Scripts: A leading UTF-8 BOM is now stripped from `--sql-file` scripts and batch stdin, so BOM-prefixed files from Windows editors and export tools parse like plain UTF-8.
* Sheet Flag On Unreadable Directories: `--sheet` validation now surfaces the real directory access error instead of misclassifying an unreadable directory as a non-Excel input.
* Multi-Workbook Sheet Filter: In a multi-workbook or directory import, a workbook that lacks the requested `--sheet` is now skipped instead of failing the whole import, so matching workbooks still load. The run fails only when no workbook contains the sheet.
* Directory Import Provenance: Directory imports now record each table's source file even when the basename is sanitized or the file yields several tables (Excel, ACH, Fedwire), so `--inspect` reports the file rather than the directory path.
* Directory Import Collisions: Two files in a directory tree that map to the same table name (duplicate basenames from different subdirectories, or sanitized-name collisions) are now rejected instead of one silently overwriting the other.
* Directory Re-Import: Re-importing a directory that overwrites an existing table is now reported as a successful overwrite instead of `No supported files found`, and the table's source is re-pointed to the directory file so `.save --force` can no longer write the directory rows back into the original source file.
* Write-Back Safety: `--save-dir` now rejects a destination that resolves to the source file or already exists in the destination directory, and validates all targets before writing any, so a failure leaves no partial output. `--output` now rejects a destination that aliases an imported source file. A read-only query no longer triggers write-back under `--save`/`--save-dir`, and a run that fails during write-back no longer prints a DML success count to stdout.

## [v0.18.0](https://github.com/nao1215/sqly/compare/v0.17.0...v0.18.0) (2026-05-31)

### Bug Fixes
* Inspect Per-File Source For Directories: `--inspect` now reports each table's real source file for directory imports, instead of the directory path for every table, restoring file-level provenance. Tables whose names cannot be matched to a single file fall back to the directory path, and directory-imported tables are still rejected by write-back.
* JSON/NDJSON Preserve NULL: `--json` and `--ndjson` now emit a SQL `NULL` as JSON `null` instead of collapsing it to an empty string, so `NULL` and `''` are distinguishable in machine-readable output. Query results carry per-cell NULL information (a NULL scans as a nil byte slice, an empty string as a non-nil empty one); text formats are unchanged.
* Stdin Name Must Be Queryable: `--stdin-name` now requires a valid table identifier (letters, digits, and underscores, not starting with a digit) and rejects values such as `my data` or `2023-data` up front. Previously such names were silently sanitized (`my data` became `my_data`), leaving the advertised name unusable in SQL.
* Table-Name Collision Detection: When two inputs sanitize to the same table name (for example `a-b.csv` and `a_b.csv`, both becoming `a_b`), sqly now fails with a clear collision error instead of letting the later import silently overwrite the earlier one while keeping the first file's source metadata.
* Input Path Validation False Positives: Input path validation no longer rejects legitimate paths. The arbitrary 10-level directory-depth limit is removed, so deeply nested workspace paths import, and the URL-encoded traversal patterns (`..%2f`, `..%5c`) are no longer matched, so a real filename that merely contains those bytes is accepted. sqly runs locally with the user's own permissions, so these web-style traversal checks only produced false rejections.
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
* **Table Names**: Fix SQL syntax errors caused by special characters in filenames
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

* Add architecture linter ([nao1215](https://github.com/nao1215))
* Reduce dependency and add unit tests for interactor ([nao1215](https://github.com/nao1215))
* Add usecase interface and mock code ([nao1215](https://github.com/nao1215))
* Bump github.com/spf13/pflag from 1.0.5 to 1.0.6 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/net from 0.30.0 to 0.33.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.4 to 1.34.5 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-colorable from 0.1.13 to 0.1.14 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.3 to 1.34.4 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump golang.org/x/crypto from 0.28.0 to 0.31.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.34.1 to 1.34.3 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.33.1 to 1.34.1 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.17.0 to 1.18.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.23 to 1.14.24 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/xuri/excelize/v2 from 2.8.1 to 2.9.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.22 to 1.14.23 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.33.0 to 1.33.1 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.32.0 to 1.33.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Add go 1.23 in unit test coverage ([nao1215](https://github.com/nao1215))
* Bump modernc.org/sqlite from 1.31.1 to 1.32.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.2 to 1.31.1 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.1 to 1.30.2 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.30.0 to 1.30.1 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump goreleaser/goreleaser-action from 5 to 6 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.10 to 1.30.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.9 to 1.29.10 ([dependabot[bot]](https://github.com/apps/dependabot))
* Update project config ([nao1215](https://github.com/nao1215))
* Bump github.com/fatih/color from 1.16.0 to 1.17.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump modernc.org/sqlite from 1.29.8 to 1.29.9 ([dependabot[bot]](https://github.com/apps/dependabot))

## [v0.8.1](https://github.com/nao1215/sqly/compare/v0.8.0...v0.8.1) (2024-05-01)

* Introduce homebrew ([nao1215](https://github.com/nao1215))

## [v0.8.0](https://github.com/nao1215/sqly/compare/v0.7.0...v0.8.0) (2024-05-01)

* Change SQLite3 driver from mattn/go-sqlite3 to modernc.org/sqlite ([nao1215](https://github.com/nao1215))
* Add benchmark ([nao1215](https://github.com/nao1215))
* Add unit test for excel ([nao1215](https://github.com/nao1215))

## [v0.7.0](https://github.com/nao1215/sqly/compare/v0.6.5...v0.7.0) (2024-04-30)

* Bump golang.org/x/net from 0.21.0 to 0.23.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Support Microsoft Excel™ (XLAM / XLSM / XLSX / XLTM / XLTX) ([nao1215](https://github.com/nao1215))

## [v0.6.5](https://github.com/nao1215/sqly/compare/v0.6.4...v0.6.5) (2024-04-29)

## [v0.6.4](https://github.com/nao1215/sqly/compare/v0.5.2...v0.6.4) (2024-04-29)

* Bump goreleaser/goreleaser-action from 2 to 5 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/checkout from 3 to 4 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump actions/setup-go from 3 to 5 ([dependabot[bot]](https://github.com/apps/dependabot))
* Maintain dependencies for GitHub Actions ([nao1215](https://github.com/nao1215))
* Introduce numerical sorting ([nao1215](https://github.com/nao1215))
* Fix issue 43: Panic when importing json table with numeric field. ([nao1215](https://github.com/nao1215))
* Fix issue 42 (bug): Panic when json field is null ([nao1215](https://github.com/nao1215))
* Update project config ([nao1215](https://github.com/nao1215))
* Introduce octocov ([nao1215](https://github.com/nao1215))
* Bump github.com/google/wire from 0.5.0 to 0.6.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.19 to 1.14.22 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.18 to 1.14.19 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.15.0 to 1.16.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/mattn/go-sqlite3 from 1.14.17 to 1.14.18 ([dependabot[bot]](https://github.com/apps/dependabot))
* (auto merged) Bump github.com/google/go-cmp from 0.5.9 to 0.6.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Add automerged workflows ([nao1215](https://github.com/nao1215))
* Bump github.com/mattn/go-sqlite3 from 1.14.16 to 1.14.17 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/nao1215/gorky from 0.2.0 to 0.2.1 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.14.1 to 1.15.0 ([dependabot[bot]](https://github.com/apps/dependabot))
* Bump github.com/fatih/color from 1.13.0 to 1.14.1 ([dependabot[bot]](https://github.com/apps/dependabot))
* Change golden package import path ([nao1215](https://github.com/nao1215))

## [v0.5.2](https://github.com/nao1215/sqly/compare/v0.5.1...v0.5.2) (2022-11-27)

* add unit test for infra package ([nao1215](https://github.com/nao1215))
* Add basic unit test for shell ([nao1215](https://github.com/nao1215))
* Add unit test for model package ([nao1215](https://github.com/nao1215))
* Bump github.com/google/go-cmp from 0.2.0 to 0.5.9 ([dependabot[bot]](https://github.com/apps/dependabot))
* Change golden test package from goldie to golden and more ([nao1215](https://github.com/nao1215))
* Add unit test for argument paser ([nao1215](https://github.com/nao1215))

## [v0.5.1](https://github.com/nao1215/sqly/compare/v0.5.0...v0.5.1) (2022-11-19)

* Add sqlite3 syntax completion ([nao1215](https://github.com/nao1215))

## [v0.5.0](https://github.com/nao1215/sqly/compare/v0.4.0...v0.5.0) (2022-11-13)

* Feat dump tsv ltsv json ([nao1215](https://github.com/nao1215))
* Add featuer thar print date by markdown table format ([nao1215](https://github.com/nao1215))
* Feat import ltsv ([nao1215](https://github.com/nao1215))

## [v0.4.0](https://github.com/nao1215/sqly/compare/v0.3.1...v0.4.0) (2022-11-13)

* Feat import tsv ([nao1215](https://github.com/nao1215))

## [v0.3.1](https://github.com/nao1215/sqly/compare/v0.3.0...v0.3.1) (2022-11-11)

* Fix panic bug when import file that is without extension ([nao1215](https://github.com/nao1215))

## [v0.3.0](https://github.com/nao1215/sqly/compare/v0.2.1...v0.3.0) (2022-11-10)

* Feat import json ([nao1215](https://github.com/nao1215))
* Fix input delays when increasing records ([nao1215](https://github.com/nao1215))

## [v0.2.1](https://github.com/nao1215/sqly/compare/v0.2.0...v0.2.1) (2022-11-09)

* Add header command ([nao1215](https://github.com/nao1215))

## [v0.2.0](https://github.com/nao1215/sqly/compare/v0.1.1...v0.2.0) (2022-11-09)

* Fixed a display collapse problem when multiple lines are entered ([nao1215](https://github.com/nao1215))

## [v0.1.1](https://github.com/nao1215/sqly/compare/v0.1.0...v0.1.1) (2022-11-07)

* Fixed a bug that caused SQL to fail if there was a trailing semicolon ([nao1215](https://github.com/nao1215))

## [v0.1.0](https://github.com/nao1215/sqly/compare/v0.0.11...v0.1.0) (2022-11-07)

* Add move cursor function in intaractive shell ([nao1215](https://github.com/nao1215))

## [v0.0.11](https://github.com/nao1215/sqly/compare/v0.0.10...v0.0.11) (2022-11-06)

* Fixed a bug in which the wrong arguments were used ([nao1215](https://github.com/nao1215))

## [v0.0.10](https://github.com/nao1215/sqly/compare/v0.0.9...v0.0.10) (2022-11-06)

* Added CSV output mode ([nao1215](https://github.com/nao1215))

## [v0.0.9](https://github.com/nao1215/sqly/compare/v0.0.7...v0.0.9) (2022-11-06)

## [v0.0.7](https://github.com/nao1215/sqly/compare/v0.0.6...v0.0.7) (2022-11-06)

* Improve execute query ([nao1215](https://github.com/nao1215))

## [v0.0.6](https://github.com/nao1215/sqly/compare/v0.0.5...v0.0.6) (2022-11-05)

## [v0.0.5](https://github.com/nao1215/sqly/compare/v0.0.4...v0.0.5) (2022-11-05)

* Add history usecase, repository, infra. sqly manage history by sqlite3 ([nao1215](https://github.com/nao1215))
* Add function that execute select query ([nao1215](https://github.com/nao1215))

## [v0.0.4](https://github.com/nao1215/sqly/compare/v0.0.3...v0.0.4) (2022-11-05)

## [v0.0.3](https://github.com/nao1215/sqly/compare/v0.0.2...v0.0.3) (2022-11-05)

* Add import command ([nao1215](https://github.com/nao1215))

## [v0.0.2](https://github.com/nao1215/sqly/compare/v0.0.1...v0.0.2) (2022-11-05)

* Add .tables command ([nao1215](https://github.com/nao1215))
* Add .exit/.help command and history manager ([nao1215](https://github.com/nao1215))

## [v0.0.1](https://github.com/nao1215/sqly/compare/dbf99896449e...v0.0.1) (2022-11-03)
