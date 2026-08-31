# CHANGELOG

## [Unreleased]

### Bug Fixes

* `.dialect` decides where a statement ends, the way `--dialect` already did. Translation and execution used the dialect `.dialect` had set, while everything that reads the text — where a statement ends, whether the interactive buffer holds a finished one — used the dialect the process started with, so the two halves of one setting disagreed for the rest of the session. `.dialect mysql` followed by `SELECT 1 # note; more` was cut at a semicolon MySQL has commented out, and `more` was run as a statement of its own; the same line under `--dialect mysql` ran correctly. At the prompt the same statement waited on a continuation for a rest that had already been written and never ran. A script is now read as it goes, so a `.dialect` inside one applies to the lines after it.

### New Features

* A mistyped name is answered with the one it is a typo of. An unknown helper command, a table this session does not have, and a column no table has each name the closest thing sqly knows, when there is one near enough: `.tabels` answers `.tables`, and `SELECT naem` answers `name`. Nearness is edit distance, where a letter dropped, doubled, mistyped, or two swapped each count as one edit; a name of five characters or more may be two edits away, a shorter one only one, and a name of two characters or fewer is never guessed at. Nothing is offered when no name is that close, and the rest of each message is unchanged.

* Completion reads the statement being typed. A table position — after `FROM`, `JOIN`, `INSERT INTO`, `UPDATE` — offers table names and no longer offers columns, which cannot go there. A `SELECT` list, a `WHERE`, `ON`, `GROUP BY`, `ORDER BY` or `SET` offers the columns of the tables the statement names before the rest of the session's. And `alias.` or `table.` offers that table's columns spelled with the qualifier, resolving the alias from the statement's own `FROM` and `JOIN` clauses. The statement is read with the dialect's own lexical rules, so a keyword inside a string literal or a comment is the text it is.

### Bug Fixes

* Tab after a lower-cased SQL keyword completes it. Keywords are offered upper-cased and matched case-insensitively, but the prompt library filtered the answer again with a case-sensitive test and discarded all of it, so `sel` followed by Tab produced no menu, no completion, and no change to the line. Completion now tells the prompt which span each suggestion replaces, which also fixes a suggestion whose text differs in case from what was typed being appended beside the word instead of completing it.

* Completion works on the continuation line of a multi-line statement. The word being completed was taken as everything after the last space on the line, which on a fresh continuation line still held the newline, so nothing matched it and the menu was empty exactly where a long statement is written.

* A column added by a schema-changing statement is completable in the same session. Completion caches each table's columns and keys the cache by the table-name set, which an `ALTER TABLE t ADD COLUMN c` leaves untouched, so the columns offered stayed as they were before the statement ran.

* A filename is no longer offered where a table goes. A bare word in a SQL statement was also completed against the working directory whenever the line held `SELECT` or `FROM`, but no SQL statement in sqly takes a path. The file a table was read from shares the table's prefix, so `FROM peo` offered both `people` and `people.csv` and neither completed.

### Dependencies

* prompt is updated to v0.0.20 for `Suggestion.Replace`, which lets a completer name the span of the input its suggestion overwrites.

## [v1.3.0](https://github.com/nao1215/sqly/compare/v1.2.2...v1.3.0) (2026-08-29)

### Bug Fixes

* A script is split into statements by the rules of the dialect it is written in, so `--dialect` reaches the splitter and not only the translator. sqly read every dialect by SQLite's lexical rules, and cut a statement where the dialect had no boundary or invented one that was never there. `sqly --dialect mysql --sql "SELECT 1 # note; more"` was refused as two statements, and a `#` line in a script became a statement of its own rather than a comment sqly skips. A backslash-escaped quote closed a string that MySQL and GoogleSQL leave open, so `SELECT 'a\';b'` was cut in half and the half reached the translator as an unterminated literal. A PostgreSQL dollar-quoted string ended at the first `;` inside it, and a PostgreSQL block comment did not nest, so a comment holding a comment was closed early. A GoogleSQL tripled quote is one string now rather than three. MySQL also asks for a blank after a double dash before it opens a comment, so `SELECT 1--1` is arithmetic there and a comment everywhere else, and GoogleSQL escapes inside a backtick-quoted name. What `#` means is the reason this could not be one rule for every dialect: it opens a comment in MySQL and GoogleSQL, and begins the `#>` and `#>>` operators in PostgreSQL.

* Importing a CSV whose header row holds two blank cells works. `a,,` -- a header written with two trailing commas, which a spreadsheet export produces -- was refused with `duplicate column name: ""`, naming a column name the file never wrote, and neither the file nor any file beside it could be read. A blank header now takes the name of its position, so that file imports with the columns `a`, `column_2` and `column_3`, and each can be named in a query. A name the file did write twice is still refused, with the column it is.

* An Excel export followed by an import keeps a row whose cells are all empty. `.save data.xlsx` wrote such a row as a row holding no cell, and reading that file back dropped it, so a table of ten rows came back with nine. A cell holding only spaces in a number column is stored as missing, so an ordinary sheet reached this without ever holding an empty string.

* A number around a million written into a CSV, TSV or LTSV export keeps the notation the source used. `.save` of a column holding `2500000` wrote `2.5e+06`, so a file exported from sqly and handed to another tool no longer read the way the file that was imported did.

* A CSV cell holding a decimal too large for a float64, such as `1e400`, keeps its digits instead of importing as an infinity. Such a column is imported as text now, which is how an integer too large to hold has always been imported.

* A CSV, TSV or LTSV file wider than 2000 columns is refused with a message saying how wide it is and how wide a table can be. It used to fail with SQLite's own `too many columns`, which named an internal staging table for a file read from stdin.

* An LTSV file read from standard input cannot ask for unbounded memory. A record with no terminator was read however long it was, where the same input in CSV or JSONL is refused at 64 MiB.

### Changed

* filesql is updated to v0.52.0, which rebuilt the dialect translation as a lexer, parser, syntax tree, lowering and renderer. Two things change for sqly. A query outside the supported subset is refused by name, with the line and column it was written at, instead of being handed to SQLite to fail on its own terms: `sqly --dialect postgresql --sql "SELECT a #> b"` now says the JSON path operators are not supported and what to write instead, where it used to reach SQLite as an unrecognized token. And a query holding only comments is refused as holding no statement rather than translating to nothing.

* The dialects answer more of what their own engines answer. MySQL's compound `INTERVAL` units work -- `DATE_ADD('2026-01-01', INTERVAL 1 DAY_HOUR)` answers what mysql:8.4 answers rather than being refused -- and so do `MAKE_SET`, `EXPORT_SET`, `JSON_LENGTH`, `TO_SECONDS`, `CONVERT_TZ`, `STR_TO_DATE` with `%j` and `%f`, PostgreSQL's `to_date` and `to_timestamp` over the templates `to_char` writes, its `age`, `scale`, `trim_scale` and `json_typeof`, and its `|/`, `||/`, `@` and JSON `-` and `#-` operators. A value a dialect refuses is refused here too rather than answered: `to_date('2024-02-30', 'YYYY-MM-DD')` is an error, `REGEXP_LIKE(x, '')` is an error, and `to_number` with an empty template is NULL.

* An error message from a rejected query carries the line and column of the construct and, where there is one, what to write instead. Scripts that match on the old wording need updating.

### Removed

* The winget release pipe, and with it the `winget install nao1215.sqly` instruction. GoReleaser generated the manifests and opened a pull request against microsoft/winget-pkgs on every stable tag, but none of those pull requests were ever merged — six were open, the oldest for twelve days, all past validation and waiting on a volunteer moderator — so the identifier resolves to nothing and the pipe published nothing. The open pull requests are withdrawn, and `winget:` is gone from `.goreleaser.yml` along with the `WINGET_GITHUB_TOKEN` wiring in the release workflow. Windows users install with `go install`, a prebuilt archive from the release page, aqua, or mise.

## [v1.2.2](https://github.com/nao1215/sqly/compare/v1.2.1...v1.2.2) (2026-08-27)

### Bug Fixes

* A query whose first `FROM` is the last word of a comment, of a string literal, or of a column aliased `from` answers instead of crashing. `sqly --sql "SELECT 1 -- from"` and ``sqly --sql "SELECT 1 AS `from`"`` ended the run with a Go panic, which exits 2 and so read as a rejected command line.

* A statement that is only a `#` comment is refused instead of crashing. `#` opens a line comment in MySQL and in GoogleSQL but not in SQLite, so `sqly --dialect mysql --sql "# hello"` survived the check for an empty statement, was translated into nothing, and panicked on the way back from running nothing.

* A blank cell written as a space rather than as nothing is a missing number, so `MAX` over a number column answers the largest value rather than the space, `SUM` and `COUNT` skip it and `IS NULL` finds it. It used to sit in the column as text, which SQLite orders above every number, and only an empty cell was read as missing. This holds for a file imported by path and for one read from a stream, which had disagreed.

* An Excel workbook whose sheet is named after the file, shortened to what Excel allows, imports as one table named after the file. A worksheet name is at most 31 characters, so a workbook named `quarterly_revenue_by_region_2026.xlsx` holding the sheet Excel had to shorten imported as the table `quarterly_revenue_by_region_2026_quarterly_revenue_by_region_202`.

* A damaged Parquet file costs no more than its own size to refuse. One 473-byte file allocated 98 MiB before failing, and did it again on every import of the same file.

* `--output` to Parquet no longer fails on a result with no table name. The export stages the result in a temporary database, and the staging table now carries a name of its own rather than the result's, which could be empty.

## [v1.2.1](https://github.com/nao1215/sqly/compare/v1.2.0...v1.2.1) (2026-08-27)

### Bug Fixes

* An import that is canceled -- Ctrl-C on a long `.import`, or a request that goes away under `sqly` used as a library -- stops reading the file it was reading, and reports the cancellation as one. A source arriving over a slow connection used to be read to the end whatever the cancellation said, and the error that came back was about a closed statement rather than about the context, so a caller could not tell a canceled import from a failed one.

* A JSON file whose array is followed by a stray `]` or `}` is refused instead of importing as the rows before it. `[{"a":1}]}[{"a":2}]` imported as a one-row table and reported success, with the second array's rows gone and nothing said about them.

* An LTSV line holding a field that names no label is handled by `--row-mismatch` rather than dropped in silence. A stray log line in the middle of an LTSV file used to vanish, leaving an import that reported success and a table missing a row.

* An import error names sqly's file layer once. Two of them said `filesql: database operation failed:` twice in one message.

## [v1.2.0](https://github.com/nao1215/sqly/compare/v1.1.0...v1.2.0) (2026-08-27)

### Added

* `--dialect googlesql` answers the scalar functions it used to refuse, and gains the DATETIME and TIME families it had no arithmetic for: `DATETIME_ADD`, `DATETIME_SUB`, `DATETIME_DIFF`, `TIME_ADD`, `TIME_SUB`, `TIME_DIFF`, `TIME_TRUNC`, `UNIX_DATE`, `DATE_FROM_UNIX_DATE`, `CURRENT_DATETIME`, `LAST_DAY`, alongside `CONTAINS_SUBSTR`, `NORMALIZE`, `EDIT_DISTANCE`, `FROM_HEX`, `TO_BASE32`, `TO_JSON_STRING`, `IEEE_DIVIDE`, `IS_INF` and the rest. `EXTRACT`, `DATE_TRUNC` and `DATE_DIFF` read `WEEK(<WEEKDAY>)`, so a query grouping by a week that starts on Monday works; `EXTRACT` answers `MILLISECOND` and `MICROSECOND` and `DATE_DIFF` answers `ISOWEEK`, `ISOYEAR` and `QUARTER`; the `SAFE.` prefix works in front of any function sqly computes rather than five of them; `APPROX_COUNT_DISTINCT`, `CORR`, `COVAR_POP` and `COVAR_SAMP` are translated; and `CAST('0x10' AS INT64)` answers a number.

* `--dialect postgresql` answers the scalar functions it used to refuse -- `quote_ident`, `regexp_count`, `cbrt`, `factorial`, `gcd`, `lcm`, the degree trigonometry, `to_number`, `to_timestamp`, `make_date`, `date_bin`, the `sha` family, `encode`, `decode`, `gen_random_uuid` and the clock functions among them. `BETWEEN SYMMETRIC` works, `DATE_PART` and `EXTRACT` answer `millennium`, `century`, `decade`, `isoyear` and the sub-second parts, `SUBSTRING(s FROM pattern)` returns what the pattern matched, and `TO_CHAR` implements the date/time and numeric templates rather than a handful of their patterns.

* `--dialect mysql` and `--dialect googlesql` read `x + INTERVAL n unit`, the operator spelling of date arithmetic that both dialects' own documentation writes first. Only the function form worked, so `SELECT d + INTERVAL 1 DAY` failed with a syntax error naming the number.

### Bug Fixes

* A blank cell in a number column is a missing number, so `MAX` answers the largest value rather than the blank, `AVG` divides by the values that are there, `COUNT(column)` counts them and `WHERE column IS NULL` finds the rows that have none. sqly stored the blank as an empty string, which SQLite orders above every number, so a column with one gap gave wrong answers to all four with nothing to say so.

* A TSV or LTSV file whose lines end with a lone carriage return loads as rows even when a value holds a double quote. One quote made every carriage return after it part of the data, so a three-row file loaded as one row holding the other two, and a wider one failed with a message about a column count that named neither the quote nor the line ending.

* `.save` on an Excel workbook leaves the cells it did not change as they were. A number in a column holding decimals was rewritten as text, so a sheet a spreadsheet was summing had text in it after a save that edited nothing, and a large number came back as `1e+15`; a `TRUE` under a date format became the string `TRUE`.

* `.save` to LTSV refuses a column name with a space at either end instead of writing a label that reads back as a different name, and a table name holding a quote or a backtick loads and saves instead of failing with SQLite's tokenizer error.

* Closing after `.save --in-place` with a transaction still open returns instead of hanging forever, and an in-place save replaces no file until every source is known to be writable, so a run that would fail on the third file no longer rewrites the first two. A save through a symbolic link writes the file the link points at and leaves the link a link.

* A record longer than 64 MiB is refused with a message naming the line and the limit, rather than read whole. A file that is not the format it claims -- one long line with no terminator -- used to cost the process the whole of it before anything noticed.

* `--dialect` answers correctly where it used to answer plausibly: a division by zero is reported as one rather than as NULL, `GCD` never answers a negative number, `TO_CHAR` reads its argument to tell a date from a number, `DATE_DIFF` in weeks counts the boundaries crossed, day and hour differences are exact for every date the calendar holds, BigQuery's `MD5` and `SHA1` answer bytes, its `DATE`, `DATETIME` and `TIME` constructors build a value instead of NULL, and `NOW()` and its siblings read UTC and answer once per statement rather than once per appearance.

## [v1.1.0](https://github.com/nao1215/sqly/compare/v1.0.4...v1.1.0) (2026-08-26)

### Added

* `--dialect mysql` answers thirty-eight MySQL functions it used to refuse as "no such function". The names that are a second spelling of something sqly already had: `LCASE`, `UCASE`, `MID`, `DAYOFMONTH`, `ISNULL`, `REGEXP_LIKE`. The strings and numbers: `STRCMP`, `BIT_LENGTH`, `INSERT`, `TO_BASE64`, `FROM_BASE64`, `CONV`, `BIN`, `OCT`, `BIT_COUNT`, `COT`, the two-argument `ATAN`, `CRC32`, `REGEXP_SUBSTR`, `REGEXP_INSTR`, `SHA1`, `SHA2`. The ones a log file wants: `INET_ATON`, `INET_NTOA`, `IS_IPV4`, `IS_IPV6`. And the times and dates: `SEC_TO_TIME`, `TIME_TO_SEC`, `MAKETIME`, `TIME_FORMAT`, `ADDTIME`, `SUBTIME`, `MICROSECOND`, `TO_DAYS`, `FROM_DAYS`, `MAKEDATE`, `PERIOD_ADD`, `PERIOD_DIFF`. A MySQL TIME is a signed span rather than a clock reading, so `SELECT SEC_TO_TIME(360000)` answers `100:00:00` and a negative one keeps its sign.

### Bug Fixes

* `--dialect mysql` matches `REGEXP` and `RLIKE` without regard to case, as MySQL does under its default collation. `WHERE name REGEXP 'a'` used to pass over a row holding `A`, so a filter written against MySQL quietly returned fewer rows, and `REGEXP_REPLACE` quietly left some occurrences alone. `LIKE` already worked this way, so the two halves of one dialect disagreed about the same letter.

* `--dialect mysql` computes `QUOTE`, `ASCII` and `HEX` the way MySQL does. `QUOTE` produced a literal MySQL cannot read back, escaping a quote by doubling it and leaving a number unquoted. `ASCII` answered the code point rather than the first byte, which made it the same function as `ORD`. `HEX` of a negative number answered a decimal with a minus sign in front of it, which holds no hexadecimal digits, so `UNHEX(HEX(n))` came back empty for every negative n.

* `--dialect mysql` shifts with `<<` and `>>` the way MySQL does, on an unsigned 64-bit value. `SELECT -1 >> 1` answered -1 rather than 9223372036854775807, and a shift by 64 or more left a negative value untouched. These were different bits, not a different way of printing the same bits.

* `--dialect mysql` reads a string in the condition of `IF` as the number its leading digits spell, which is what MySQL and SQLite both do. `IF(flag, a, b)` took the a branch for every value that was not a number, and a column sqly typed as text is exactly where that lands: a flag column holding `0` and `1` alongside a stray `no` took the true branch for every `no`.

* `.save` and `--output` of an ACH or Fedwire file no longer write a file with a field replaced by another field's value, and no longer apply an edit to a record other than the one it was made on. A Fedwire message is now read back and compared column by column before a byte reaches the destination, and an ACH row is held against the position it was read from, so a write that cannot be made faithfully fails and leaves the original file where it was.

### Changed

* The Go baseline moves to 1.25.13, which is what filesql v0.47.0 requires. The unit test matrix pins the patch rather than the minor: `setup-go` resolves a bare `1.25` from its own manifest and pins the toolchain to what it installs, so a runner that had 1.25.12 refused a `go.mod` asking for 1.25.13 and the job failed on a version this repository never chose.

* filesql v0.46.0 → v0.47.0, which is where everything above comes from. That release also removes exported symbols from filesql's `dialect`, `prep` and compression packages and changes `frame.DataFrame`'s `DistinctBy` and `DropNASubset` to return an error. sqly uses none of them, so a program that embeds sqly's own packages is unaffected; a program using filesql directly should read that release's notes.

## [v1.0.4](https://github.com/nao1215/sqly/compare/v1.0.3...v1.0.4) (2026-08-26)

### Bug Fixes

* `.import` of a directory loads every file in it that sqly can read, and follows a symbolic link to one. The directory walk carried a filter written for filesql's own repository, so a supported file whose name it did not recognize was skipped without a word, and a link to a directory was read as a file and refused.

* `.import` of a large workbook or Parquet file costs a fraction of what it did. A sheet's blank rows are no longer counted as records, a workbook's cells are read a row at a time rather than one call per cell, and a Parquet file named by path is read where it lies instead of being copied into memory first. An 18.5 MB workbook of 200,000 rows no longer holds gigabytes while it loads.

* `.import` of a compressed file reads all of it. A `.z` file holding more than one zlib stream read as the first stream alone and said nothing about the rest, and an xz file's second stream and later were not held to the memory limit the first was.

* `.save` and `--output` work on a destination whose name is close to the filesystem's length limit, and report a staged file they cannot remove instead of losing the report inside the operation's error.

* `.save` of a workbook keeps its sheets apart from those of another source whose name it prefixes. Two files named `report.xlsx` and `report_2026.xlsx` had their sheets attributed to whichever matched first.

* `--dialect mysql` computes `WEEK`, `WEEKOFYEAR`, `YEARWEEK`, `QUARTER`, `ADDDATE` and `SUBDATE`, all of which failed as "no such function" or as a syntax error naming a token the query did not contain. `STR_TO_DATE` reads a month, day or time written without a leading zero, which MySQL accepts and sqly refused.

* `--dialect postgresql` answers a timestamp for a date plus an interval, as PostgreSQL 17 does. `DATE '2026-01-31' + INTERVAL '1 month'` was `2026-02-28` and is now `2026-02-28 00:00:00`, which decides a comparison against a timestamp literal.

* A string with a second point in it keeps the number in front of it where a dialect casts it. `'1.2.3'` cast to a number was 0 rather than 1.2, so a version string sorted as if it had no number at all.

### Changed

* filesql v0.45.0 → v0.46.0, which is where the fixes above come from. Its release also removes five exported symbols sqly does not use, and changes filesql's `prep` package: the cross-field tags read every field and value pair they name, an empty cell passes a cross-field comparison, and `required_with_all` and `required_without_all` are accepted. A program that embeds sqly's own packages is unaffected; a program using filesql's `prep` directly should read that release's notes.

* Dependencies: `github.com/moov-io/wire` 0.16.0 → 0.16.1.

## [v1.0.3](https://github.com/nao1215/sqly/compare/v1.0.2...v1.0.3) (2026-08-24)

### Bug Fixes

* `.import` of a workbook or an ACH file leaves the database as it was when the file fails partway. A workbook whose later sheet could not be read left the earlier sheets' tables behind, and a table sqly already held under one of those names was replaced by them, so a failed import both reported an error and changed what was loaded.

* `--dialect postgresql` stops a query that divides by zero, and so does `--dialect googlesql` for `MOD`. Both answered NULL, which is SQLite's answer and neither engine's: a report dividing by a count that turned out to be zero came back with a blank cell rather than the error PostgreSQL gives, and the blank flowed into the next `SUM` unnoticed. `--dialect mysql` still answers NULL, which is what MySQL answers.

* `--dialect postgresql` computes `div`, `trunc(x, n)` and `width_bucket`, and `--dialect googlesql` computes `DIV` and `TRUNC(x, n)`. All five failed with "no such function" or a wrong-argument-count error although both engines define them, while the MySQL spelling of the same truncation already worked.

* A column holding both signs of zero is one value rather than two. `-0.0` and `0.0` grouped apart in the `frame` package, so a summary over a column of deltas reported the zero group twice with the rows split between them.

* `.import` with a `prep` struct compares two columns the way the field they land on says. A comparison between two columns answered on the text whatever the columns were, so `007` was neither greater than, nor equal to, nor less than `7` in the same file.

* A parquet export refuses a header sqly cannot read back. Two column names differing only in surrounding whitespace — `SELECT 1 AS "x", 2 AS " x"` — were written at exit 0 and the file then failed to import, because the reader takes those as one column while the export compared only case. The export now compares the way the import does, so the pair is refused before a file is written, as it already was for csv, tsv and excel.

### Changed

* filesql v0.44.0 → v0.45.0, which is where the fixes above come from. Its release changes the `prep` package's `numeric` and `number` validate tags to the go-playground meanings they are named after, unexports three interfaces sqly does not use, and makes `prep`'s cross-field comparisons follow the field they land on. A program that embeds sqly's own packages is unaffected; a program using filesql's `prep` directly should read that release's notes.

* Dependencies: `github.com/mattn/go-runewidth` 0.0.27 → 0.0.28, `github.com/pierrec/lz4/v4` 4.1.28 → 4.1.29.

## [v1.0.2](https://github.com/nao1215/sqly/compare/v1.0.1...v1.0.2) (2026-08-21)

### Bug Fixes

* `--dialect mysql` answers `SUBSTR` from position 0 the way MySQL does. MySQL reads position 0 as no position at all and returns an empty string, where SQLite reads it as the slot before the first character; `SUBSTR(s, LOCATE('x', s))` was the whole of `s` for a row without an `x`, which is the shape a lookup takes when it finds nothing. `--dialect postgresql` follows PostgreSQL's rule for the same call, checked against PostgreSQL 17. `--dialect googlesql` is left on SQLite's answer, because no BigQuery was available to check its rule against; the dialects page says so.

* `--dialect mysql` writes the value rather than the letter for twelve `DATE_FORMAT` specifiers. `%f`, `%k`, `%l`, `%r`, `%T`, `%D`, `%u`, `%U`, `%v`, `%V`, `%x` and `%X` came back as their own letter, which is also what MySQL does for a specifier it does not know, so a query asking for `'%Y-%m-%d %T'` returned `2024-02-29 T` and looked like it had worked. Each was checked against MySQL 8.4 for every date from 1995 to 2035.

* `.save` and `--output` to a workbook keep what the file already had. Saving an `.xlsx` back over itself rebuilt it from the values sqly holds, so a column computed by a formula came back empty and a date cell came back as text; the column widths, cell styles, merged ranges and comments went with it, and a sheet no table was loaded from was deleted outright. Only the cells whose value changed are written now.

* A file sqly cannot read fails instead of taking the shell with it. A damaged Parquet or Excel file could crash the process, and a Parquet file whose metadata places a column chunk outside it could allocate until the machine ran out of memory. Both are errors now. A Parquet file whose metadata is consistent and whose page headers are not can still exhaust memory; a program pointed at Parquet from an untrusted source should read it where the process can be lost.

* `.import` of a JSON file that is not JSON reports the same error whether or not it starts with a bracket. A malformed object and a malformed array are the same fault and were reported two ways.

* `.import` reads a column's type from the whole file rather than from its first chunk. A column that looked numeric until a later row proved it text was stored with SQLite's spelling of its numbers rather than the file's, so `2.50` came back as `2.5`.

### Changed

* filesql v0.43.1 → v0.44.0, which is where the fixes above come from. Its release removes exported names sqly does not use, so nothing about sqly's own API changes.

* Building sqly from source needs Go 1.25.8, up from 1.25.0, which is what filesql v0.44.0 declares. A released binary is unaffected.

* Dependencies: `golang.org/x/text` 0.40.0 → 0.41.0, `github.com/pierrec/lz4/v4` 4.1.27 → 4.1.28, `github.com/rickar/cal/v2` 2.1.27 → 2.1.29, `google.golang.org/grpc` 1.82.1 → 1.83.0.

## [v1.0.1](https://github.com/nao1215/sqly/compare/v1.0.0...v1.0.1) (2026-08-15)

### Features

* Windows installs through winget: `winget install nao1215.sqly`. A tagged release pushes the generated manifests to a fork of the community repository and opens the pull request against microsoft/winget-pkgs, so the package tracks releases without a separate manual submission. The Windows archives are zips, which winget installs as a portable package — `sqly.exe` lands on PATH with no installer to run. Release-candidate tags are skipped, since the community repository takes stable versions only.

## Release candidates and what v1.0.0 froze

`v1.0.0` is the stable release. Pin against it.

* Breaking changes landed during the release candidates, rc1 through rc8. Each
  is listed under Breaking Changes in the section for the candidate that made it.
* Between rc8 and v1.0.0 only bug fixes and documentation changes landed, as
  rc8 promised: no new flag, no new feature, no changed default. A bug fix
  moves a run toward what these notes already describe — most turned a silently
  wrong result into a named error — without changing what the notes promise.

## [v1.0.0](https://github.com/nao1215/sqly/compare/v1.0.0-rc8...v1.0.0) (2026-08-09)

### Bug Fixes

* `--dialect mysql` translates the operator family `||` belonged to. `&&`, `!`, and `^` reached SQLite's tokenizer as unrecognized tokens, so a query using the MySQL spelling of AND, NOT, or a bitwise exclusive or failed with a message about SQLite's parser. `&&` is now `AND`, `!a` is a parenthesized `NOT` — parenthesized because MySQL's `!` binds tighter than a comparison and SQLite's `NOT` binds looser, so `!a = b` would otherwise negate the comparison — and `^` is a helper call, since SQLite has no XOR operator. `XOR` is refused by name: its precedence sits between `OR` and `AND`, which SQLite has no operator for, and the message says what to write instead. Fixed in filesql v0.41.0.
* `--dialect googlesql` recognizes BigQuery's `SAFE.` call prefix, and refuses an array by name. `SAFE.DIVIDE(1, 0)` is now the same query as `SAFE_DIVIDE(1, 0)`; the prefixed form is what BigQuery's own documentation writes, and it used to reach SQLite as a qualified name and fail on the `(`. An array literal reached SQLite as identifier quoting and came back as `no such column: 1,2,3`, about a column the query never wrote; it now says arrays are not supported. A `SAFE.` prefix on a function with no safe form is refused rather than dropped.
* `--dialect postgresql` refuses a set-returning function by name. `generate_series` in a FROM clause reported `no such table: generate_series`, which reads as a missing input file rather than as a construct SQLite has no form for. The dialects page lists all three sets of rejections.

* An excel export refuses a value longer than an XLSX cell instead of cutting it to fit. Excel stops a cell at 32,767 characters, and the writer wrote the first 32,767 of a longer value at exit `0` — a 40,000-character value lost 7,233 characters with nothing said. The length is now checked beside the character check that already refuses a value XLSX cannot carry, in characters because that is the unit the limit is in, and the message names the column, the length, and the limit. A value of exactly the limit still writes and reads back whole.

* An infinity exported to parquet keeps its type. sqly wrote a genuine double into the file, but reading it back gave `+Inf` typed as text, because filesql rendered a non-finite double with `%g` and SQLite's REAL affinity cannot read `+Inf`: the value landed as text in a column declared REAL. Fixed in filesql v0.40.1. The text formats already spelled the three values `Infinity`, `-Infinity`, and `NaN` everywhere, and JSON already quoted the same words as PostgreSQL's `row_to_json` does; what each format does with a value it cannot hold is now written down in the reference under "What a format cannot hold".

* An ltsv export refuses a result with no rows. LTSV records its columns on each row, so a result with no rows has nowhere to put them and the writer produced a zero-byte file — which sqly's own import then refuses as an empty data source, at exit `3`. csv writes its header and json writes `[]`, and both load back as a zero-row table; LTSV has no such form. The export now stops at exit `4`, the way a parquet export of an empty result already does.

* A csv, tsv, or ltsv export refuses a BLOB rather than writing its raw bytes. A BLOB is usually not text at all, and the bytes went into the file as they were, so the export exited `0` and produced a file sqly's own reader then refuses as not UTF-8. XLSX already refused such a value and JSON base64-encodes it; csv, tsv, and ltsv did neither. They now refuse it at exit `4`, naming the column, before anything is written, and the message points at `--output-format json`. XLSX's message, which used to send the user to csv/tsv, points there too. The same refusal covers a text column read with the wrong `--encoding`, which arrives as the same bytes.

* `--row-mismatch skip` says how many rows it dropped. Skipping ragged rows is what the flag asks for, so it is not an error, but an import that reported nothing left one dropped row and most of the file dropped looking exactly alike — and the row count that came out only meant something to someone who already knew what went in. A `.save --in-place` afterwards writes the smaller table back over the source, which is the moment the loss stops being recoverable. Every import path now prints `skipped N of M data rows` on stderr when rows were dropped, and prints nothing when none were.

* A write refuses a destination the filesystem will not open for writing. `--output`, `.dump`, and `.save --in-place` staged a file beside the destination and renamed over it, which needs only a writable directory — so a file with `chmod a-w` was replaced at exit `0`, and its mode was copied onto the new content, leaving the protection in place over data the user had protected. Every neighboring tool refuses here: shell redirection, `tee`, `cp`, and `sed` all fail, and an editor asks for a bang. sqly now refuses too, at exit `4`, naming the file and its mode; a `.save` covering several files refuses before any of them is touched. The question is asked by opening the file, so an ACL, an owner that is not this user, and a read-only mount are caught as well as the permission bits.

* An Excel export writes SQL NULL as a blank cell. A workbook has no null, but it does have "no cell", which is what a reader shows as blank and what every tool that opens the file tells apart from a cell holding the empty string. Writing the empty string for both said the column had a value in every row. Parquet has preserved NULL since the fix for its own version of this; excel now says as much as a workbook can.

* A write-back keeps the source's text encoding. `.save` wrote UTF-8 whatever the file was read as, so `sqly --encoding shift-jis` plus `.save --in-place` converted the user's Shift-JIS file without a word, and their own next run of the same command decoded the new bytes as Shift-JIS and returned mojibake at exit `0`. The encoding is preserved the way compression already was. A value the encoding cannot write refuses the save at exit `4`, naming the encoding and leaving the file as it was, rather than writing a substitute character. `--output` and `.dump` are unchanged: they create new files rather than rewriting one the session read, and a new file is UTF-8.

* `--encoding` refuses bytes that are not valid in the encoding it names. rc8 made a text input that is not UTF-8 fail the import, and naming a legacy encoding turned that off: the decoders follow the WHATWG rule and substitute `U+FFFD` for input the encoding has no meaning for, so a file that was never valid Shift-JIS loaded as replacement characters at exit `0` with nothing on stderr — the corruption the UTF-8 check exists to stop, arriving through the door the flag opened. Naming an encoding now changes which bytes are valid rather than whether they are checked. A byte that begins nothing in Shift-JIS, EUC-JP, or ISO-2022-JP is refused, as is a UTF-16 code unit cut in half or a surrogate with no partner; the exit code is `3` and the message names the encoding. A `U+FFFD` a UTF-16 file really holds still loads, because that encoding can write one.

* A script the `.save` preflight refuses exits `2` rather than `1`. The check runs before the first statement, so nothing has run when it fires — no statement feedback is printed and no destination is created — and `2` is the code for a script that was not accepted, where `1` means a statement ran and failed. It reported `1` because the refusal was a bare error, which is the code anything unclassified falls through to.

* An export refuses a header it could not read back. rc8 made one rule of the duplicate header on the way in, and csv, tsv, and excel still wrote one on the way out: `SELECT * FROM a JOIN b ON a.id = b.id` names `id` twice, so the export reported success and the file it produced failed to load with `duplicate column name`. json, jsonl, ltsv, and parquet already refused it, which is what made this a difference between formats rather than one answer. All of them refuse it now, at exit `4`, leaving the destination as it was. The rule is the import's own: names are compared with surrounding whitespace removed, and again with ASCII case ignored, so `x` beside ` x` and `a` beside `A` are each refused, while `ä` beside `Ä` still exports because SQLite tells those apart. LTSV gains the case half it was missing.

### Dependencies

* filesql v0.42.0. It stops a sampled type decision from damaging a value it never saw — a zero-padded code or an integer past int64 arriving after the first 1000 rows was rewritten by SQLite's affinity, so `007` loaded as `7` — and it refuses an ACH or Fedwire write-back value too wide for its fixed-width record instead of cutting it to fit. It also reports what a skipping import dropped, which is what the `--row-mismatch skip` warning above is built on. Since then it has fixed three imports and widened the dialect translation, each described under Bug Fixes above: a file whose lines end with a lone carriage return, a TSV holding a double quote, a Parquet double holding an infinity, and MySQL's operator family with BigQuery's `SAFE.` prefix. v0.41.1 adds four more of the same shape: a quoted value padded with whitespace keeps its spaces instead of losing them to numeric affinity, LTSV labels that are one column to SQLite are refused by name rather than at a failed CREATE TABLE, and the `frame` package keeps a zero-padded code and writes TSV literally. v0.42.0 carries one import fix sqly sees — an Excel date cell arrives as an ISO date rather than as whatever its number format rendered, so such a column sorts chronologically and compares against a date literal — alongside seven `frame` package fixes sqly does not use.

* filesql v0.43.1. It fixes an ACH and Fedwire write-back that could reach for the wrong file. The structure a dump needs lived in a process-global map keyed by the file's base name alone, so `/a/payment.ach` and `/b/payment.ach` were one key: loading the second replaced the first, and a `.save` of the first file's tables applied them to the second file's structure — an error when the shapes disagreed, and silently wrong output when they lined up. Each database now records its own sources inside itself, so two of them cannot collide and a rolled-back import discards the metadata with the tables it describes. Two consequences reach sqly, both written up on the formats page: a `.save` back to `.ach` or `.fed` needs the file the tables were imported from to still be readable, and table names beginning with `_filesql_` are reserved by the library, so an import that would land under the prefix is refused at exit `3`.

* prompt v0.0.19. Five of its fixes are visible in sqly's shell: input that filled a terminal row exactly no longer erases the line above the prompt (the bottom of the result you are reading), a prompt prefix wider than the terminal no longer leaves a torn copy of itself behind on every line, Ctrl+R redraws its search in place instead of stacking a block per keystroke, a statement recalled from history comes back as it was submitted (a multi-line statement is one entry, and its leading spaces survive), and a completion menu wider than the terminal no longer leaves rows of itself above the prompt.

### Documentation

* What a write-back does and does not preserve is written down. `.save` keeps a source's compression and encoding but not its bytes: every row it writes ends `\n` and a field is quoted only when CSV requires it, so a saved CRLF file comes back LF and needless quotes are dropped. The formats page says so beside the note that promises the other two, and points at `--output` for a source whose bytes must survive. The line-terminator half is tracked as [filesql#269](https://github.com/nao1215/filesql/issues/269).

* What `--output` does with a destination extension is written down in full. The formats page said the extension "must agree with the chosen format", which holds only for an extension sqly knows: an unknown one is written as given, so `--output report.txt` holds CSV, and a path with no extension gets the format's own, so `--output report` writes `report.csv`. All three are pinned by tests now.

## [v1.0.0-rc8](https://github.com/nao1215/sqly/compare/v1.0.0-rc7...v1.0.0-rc8) (2026-08-07)

The final release candidate for v1.0.0. Everything below is a change from rc7.

The theme of this one is the file as it is on disk rather than as it was meant
to be. A CSV exported from Excel in Japan is Shift-JIS, and SQLite stores TEXT
as UTF-8, so loading one as the other built a table that queried wrong and said
nothing about it; those bytes are refused now, and the error names the flag that
reads them. A duplicate header is judged by one rule whatever format carried it,
and the refusal says which column it means. And a file named `query_result_...`
imports like any other — a prefix sqly had reserved for itself, kept hidden long
after it stopped writing anything under it.

### Breaking Changes

* A text input holding a byte sequence that is not valid UTF-8 is refused instead of loaded as mojibake. SQLite stores TEXT as UTF-8, so a Shift-JIS CSV — the ordinary export from Excel in Japan — went in as bytes that are not characters: `LENGTH` counted wrong, `LIKE` and `UPPER` worked on fragments, the replacement character appeared in output, and the run exited 0 with nothing on stderr. The file now fails the import with exit 3, and the error names the byte that is not UTF-8 and the flag that reads the file: `--encoding shift-jis`, or the same advice spelled as a restart when `.import` was typed into a running session, where the encoding can no longer be chosen. It is a check on the bytes rather than on which encoding a file is in, so a Shift-JIS file whose content is entirely ASCII still loads: sqly does not guess the encoding, because nothing in the bytes says which one it is and a wrong guess is the same corruption in a different shape. Binary containers are unaffected: Parquet and Excel state their own encoding, and ACH and Fedwire are fixed-width records. Fixed in filesql v0.36.0.

### Bug Fixes

* A header is refused by the same rule whatever format it arrived in. Names are compared with surrounding whitespace removed, which is what reading a CSV has always done, and a workbook was the exception twice: `name` beside ` name ` became two columns there and a duplicate everywhere else, while a header repeating a name exactly reached SQLite and came back as `SQL logic error: duplicate column name` wrapped in a database-operation error rather than as sqly's own refusal. Fixed in filesql v0.37.0.
* A duplicate column name says which column it means. A header with two unnamed columns (`a,,`) was refused with `filesql: duplicate column name:` and nothing after the colon, so neither the name nor its position was recoverable from the message. It reads `duplicate column name: "" (column 3)` now, and a name that collides only after trimming shows the whitespace that did it. Fixed in filesql v0.36.0.

* A file whose name begins with `query_result_` imports like any other. sqly once materialized results into tables of that name and filtered the prefix out of every listing; the materializing was removed and the filter was not, so it could only reach tables the user owned. Because import decides whether a file produced anything by comparing the table list before and after loading, such a file did not merely go missing from `.tables` and `--inspect` — the import failed outright with "the file has no rows to import" over a file full of rows, and `.save` had no table to write back. `CREATE TABLE query_result_x` had the same result from inside the session. SQLite's own `sqlite_` tables are still excluded, and that prefix is reserved by the engine, so no imported file can land under it.

### Refactoring

* The session has one table listing instead of two. Import counted tables through the filesql adapter's own `sqlite_master` query while `.tables`, `.save`, and `--inspect` went through the repository's, so two listings that had to agree were free not to — which is how one of them came to hide a name prefix the other showed. The adapter's copy is gone and every caller reads the repository.
* Eight members no production code called are gone from the usecase and repository interfaces: `QueryStream` through all four layers, left behind when the commands that streamed rows were removed; `Exec` on `QueryUsecase`, since `ExecSQL` is the only entry point for a statement the user typed; `SanitizeForSQL`, `RowMismatchPolicy`, and `GetTableNames` on `ImportUsecase`; and `Table.Valid` with `IsEmptyName`, `IsEmptyHeader`, `IsEmptyRecords`, and `IsSameHeaderColumnName`, a validation path nothing invoked. The `domain` package held only the errors that validation returned, so it is gone too.
* `ExcelSheetUsecase` is part of `ImportUsecase` rather than an interface embedded in it. Nothing consumed it on its own: what a workbook held and which sheets an import took is a question only importing raises.

### Dependencies
* filesql v0.37.1: upgraded from v0.35.2 across three releases, for the invalid-UTF-8 refusal and the duplicate column name that says which column it means (v0.36.0), the workbook header judged by the rule every other format uses (v0.37.0), and the removal of the second XLSX loader that let the rule differ by format in the first place (v0.37.1).
* modernc.org/sqlite v1.56.0: upgraded from v1.55.0.
* github.com/santhosh-tekuri/jsonschema/v6 v6.0.3: upgraded from v6.0.2.

## [v1.0.0-rc7](https://github.com/nao1215/sqly/compare/v1.0.0-rc6...v1.0.0-rc7) (2026-08-07)

A release candidate for v1.0.0, not the final one. Everything below is a change
from rc6.

The theme of this one is taking things away. Three surfaces a user could reach
are gone, each because something else already answered the same question:
`.header`, which printed the column names `.describe` prints; `.mode excel` and
`.mode parquet`, which set a mode that then rendered CSV; and the SQLite
database behind the shell's history, which is now a text file one entry per
line. Inside, six pieces of scaffolding went the same way — interfaces with one
implementation and nobody substituting them, an export that wrote every byte
twice, and functions whose last caller had already been deleted. Two documents
went too: the internal design prose, and the migration guide that restated these
notes.

### Breaking Changes
* Command history is a text file, and the variable that points at it is `SQLY_HISTORY_PATH`. It was a second SQLite database named by `SQLY_HISTORY_DB_PATH`, which bought nothing a file does not: history is an append-only log, and an append to a file opened `O_APPEND` is atomic per write, so two sqly processes interleave whole lines instead of one of them waiting on a lock and then disabling history. The default moves from `history.db` to `history` under the config directory, one entry per line with newlines escaped, so a session's history is readable with anything. An existing `history.db` is not read; the shell starts with an empty history. Unwritable still means one warning and a run that continues.
* `.mode` no longer takes `excel` or `parquet`. Neither can be rendered to a terminal, so selecting one left the session printing CSV under another name — the banner had to say "active only when executing .dump, otherwise same as csv mode", and a pipeline reading that output depended on excel meaning csv. `.mode excel` now exits `2` and names the modes a screen can show. Writing either format is unchanged: in a display mode `.dump TABLE out.xlsx` takes the format from the extension, a mode that names a different format still rejects the destination, and `--output-format excel --output FILE` still names a file format for the flag that writes a file.
* `.header` is removed; `.describe` is what it was a subset of. It printed a table's column names, which is the `name` column `.describe` already prints, so the shell answered one question two ways and carried a rendering path nothing else used. `.header` now reports "no such sqly command" and exits `1`. A script reading the names out of a structured mode reads the `name` key rather than `column`.

### Documentation
* The migration guide is gone; a breaking change is described once, in its Breaking Changes entry here. `doc/migration.md` restated those entries as before-and-after commands, which meant every breaking change was written twice and kept in step by hand — and by drift tests that existed because it drifted. Each entry above already says what changed and what a caller does about it. The dead pointers to the guide are removed from the README, the site's front page, and the frozen rc3 through rc6 sections.
* The internal design documents are gone: `doc/architecture.md` and `doc/design_overview.md`. Prose about internal layering has to be maintained by hand beside the code it describes, and the design overview showed the cost of not doing it — it described format-specific models that filesql replaced, and embedded three images the repository does not hold, while the public about page still sent readers to it. The layering is checked by go-arch-lint against `.go-arch-lint.yml`, where the rationale for each edge now lives; the about page links that file. `doc/build_and_test.md` and `doc/cookbook.md` stay: one is how to build, the other is the cookbook page's source.

### Refactoring
* Four functions the pre-v1.0.0 removals left behind are gone: `SingleQuote` (its caller was the INSERT builder, removed with the row-by-row write path), `PrintModes` (completion reads the selectable list since `.mode` lost excel and parquet), `NewHeader` (a bare conversion no production code called), and `writableExportTarget` (a wrapper over `exportTargetFor`, which is what the shell calls). Their tests moved to the surface that survives rather than being dropped.
* The history stack is one interface again. `HistoryRepository` and `HistoryUsecase` declared the same three methods, with an interactor forwarding each call unchanged and nothing ever substituting either; the file-backed history now satisfies the usecase directly. `History` loses the id and the table conversion it needed as a SQLite row — a file's order is its order. The shell also preloads only the newest entries the prompt keeps, rather than handing over the whole file for the prompt to trim.
* FileSQLAdapter carries only the methods sqly calls. Its Query, Exec, GetTableHeader, and LoadFile were reached by tests alone: the session runs SQL through the memory repository and imports through LoadFiles. The adapter's Query held a second scan loop over the same rows, so the two readers could have diverged with only a user to notice.
* Export formats are serializer functions rather than five interfaces. CSVRepository, TSVRepository, LTSVRepository, ExcelRepository, and FileRepository each had one implementation, one consumer, and one method, and nothing ever substituted them. A registry keyed by export format replaces the switch whose default branch wrote CSV, so a format added to the model and not wired up is now reported rather than written in the wrong format under the right extension.
* An export writes its bytes once. Every caller of DumpTable already hands it a scratch path that its own staging and rename commit from, so serializing into a second temporary file in the OS temp directory and copying it across wrote every exported byte twice and made the export depend on free space in two filesystems.
* SQLite3Repository no longer declares CreateTable and Insert. Imports have gone through filesql since the loader was replaced, so the row-by-row write path they backed, including the statement builders behind it, was reachable only from tests while still obliging every implementation and mock to carry it.

## [v1.0.0-rc6](https://github.com/nao1215/sqly/compare/v1.0.0-rc5...v1.0.0-rc6) (2026-08-06)

A release candidate for v1.0.0, not the final one. Everything below is a change
from rc5.

The theme of this one is what sqly writes and what it says about it, found by
running the rc5 binary rather than by reading it. Two paths lost rows and said
nothing: a one-column CSV or TSV result piped to a file, and an Excel export
whose last row was empty. A column could be declared a type its own values were
not stored as. Three refusals reported an exit code that named the wrong thing
to fix, and four messages described sqly's insides rather than the query.


### Breaking Changes

* An `--output` destination that cannot carry its compression exits `4`, not `1`. `--output out.csv.bz2` (bzip2 has no writer), `out.parquet.gz`, and `out.xlsx.gz` all reported a refusal the exit code then misdescribed: nothing had run, and `1` is the code for a statement that ran and failed. The neighboring refusal, a mode contradicting the destination extension, was already classified; this one fell through unclassified.
* Exporting to Excel refuses a result whose last row is empty in every column. A workbook stores cells, not rows, so such a row leaves nothing behind to mark where it was and a reader counting rows stops at the last one with a value: three rows were written, two read back, and the export reported success. Only the tail is at risk, since an empty row with data after it is found by the rows following it. It now exits `4` and says what to export instead. csv, tsv, json, and parquet carry the row unchanged.
* A non-finite number is spelled the same way in every output format. `--output-format json` wrote `"Infinity"`, which the reference documents, while `csv`, `table`, and the rest wrote Go's own `+Inf` — one value with two spellings from one query, neither of which any other tool produces. All formats now use `Infinity`, `-Infinity`, and `NaN`.
* Exporting to Excel refuses a value holding bytes that are not valid UTF-8, where it used to change it. rc5 made XLSX refuse a character XML cannot carry, and this is the same substitution arriving through the door that guard could not watch: ranging over a Go string decodes each invalid byte as `U+FFFD` before the check sees it, so the value passed and the writer wrote `U+FFFD` in its place. The export succeeded, the file appeared, and the byte was gone. It now exits `4` and names the likely cause, which is a file read with the wrong `--encoding`. csv and tsv still carry such values through unchanged.
* A `.save` that cannot write its destination exits `4`, not `1`. The exit-code table has always said a destination that could not be written is a `4`, and said it holds for a `.save` inside a script exactly as on its own — but only the checks a save runs before writing were classified. The write itself returned unclassified errors, so a read-only directory, an unwritable source, or a failed move all exited `1`, which is the code for a statement that ran and failed. A wrapper that checked for `1` to mean "the SQL was wrong" was told that about a full disk.
* `--output` refuses a destination that is not a regular file. The write stages a scratch file beside the destination and renames it into place, and a rename replaces a name rather than what it points at: aimed at a named pipe it unlinked the pipe, left a regular file where it had been, and reported success while the reader blocked on the other end received nothing at all. Aimed at `/dev/null` it tried to create the scratch file in `/dev` and failed with a permission error naming a path nobody wrote. A named pipe, device, or socket destination now exits `4` and is left exactly as it was. Writing to a stream is what stdout is for.
* `--sql` refuses a statement that parses to nothing. An empty `--sql` was already refused by the flag parser, which sees the string; whitespace, a bare semicolon, or a comment got through it, ran nothing, and exited `0` — indistinguishable from a run that worked, which is what `sqly --sql "$QUERY"` does when `QUERY` is unset. It now exits `2`, as `--sql-file` already did for the same content.
* A dialect rewrite keeps the caller's result column name. SQLite names an unaliased result column after the text of the expression that produced it, so translating the expression renamed the column: `--dialect postgresql --sql "SELECT amt::text"` produced the JSON key and CSV header `postgresql_cast(amt, 'text')`. MySQL's `/` and `CONCAT` and GoogleSQL's `SAFE_CAST` did the same. The label is now the expression as written. A consumer keyed on the old internal name has to change; one that used `AS` is unaffected. Fixed in filesql v0.35.0.

### Bug Fixes

* `.import` says when it makes a table mean a different file. Importing a file whose table name is taken is allowed on purpose — it is how a table picked up from a directory becomes one the session names directly — but it also rebinds the name and drops whatever the session had done to the old table. `UPDATE` reported its rows, the import replaced the table, and a later `.save` found nothing changed and wrote nothing: every step succeeded and the edit was gone. Naming two such files at once on the command line was already refused; reaching the same state one `.import` at a time said nothing at all. A re-import of the same file is a reload and stays quiet.
* `.header` and `.dump` report a missing table in sqly's words. Both handed back SQLite's, so a typo in a table name was answered with `SQL logic error: no such table: nope (1)` — an error number and an engine the user never addressed. `.describe` and `.schema` already said `no such table: nope`, and now all four agree.
* `--output` with several result sets no longer advises a format that also fails. The message was shared with the stdout version of the error, where switching to `table`, `vertical`, or `markdown` is a way out. It is not one here: `--output` writes one file whichever format it holds, so a reader who followed the advice hit the identical error. It now suggests dropping `--output`.
* Table mode decides column alignment from the values, not the column name. It also guessed from the name by substring, so `message` and `package` were right aligned because they contain `age`, and `total_label` because it contains `total`. The guess never made a column of numbers align that the values would not have aligned anyway.
* A parquet export names a duplicated column itself. The refusal came from the temporary database sqly stages the file through, so `SELECT 1 AS x, 2 AS x` was answered with `create staging table: SQL logic error: duplicate column name: x (1)`, describing a database the user never opened. It now names the column and how to alias it.
* A column's declared type matches what is stored in it. `1_000` and `0x1p4` are numbers to Go's parser and not to SQLite's numeric affinity, so the column was declared `REAL` and every value in it stored as text; a datetime alongside a number did the same. `--inspect` reported `REAL` where `typeof()` said `text`, which a consumer reading the schema to plan a numeric comparison had no way to detect. Fixed in filesql v0.35.2.
* A one-column CSV or TSV result keeps its empty rows when it is piped, not only when it is exported. The rule that a record of one empty field is written `""` was applied on the `--output` path and not on stdout: `sqly --output-format csv --sql "SELECT v FROM t" > out.csv` over three rows whose middle value was empty wrote a blank line, and a blank line is not a record, so it read back as two rows and said nothing. `.mode csv` and `.mode tsv` wrote the same blank line. There is one delimited writer now, used by every destination, so the two cannot drift apart again. A row of several columns is unaffected: its delimiters already say how many fields there are.
* A `.save` that finds nothing to write leaves the affected-row counts where they belong. The counts are buffered during a run that ends in `.save` so a failure during write-back cannot leave stdout claiming rows reached disk, and every path that ends the save released them except the one that had nothing to save. They were drained at the end of the run instead, so `UPDATE` as the second statement of a script printed its count after the fourth statement's result.
* A Parquet export writes the number sqly displayed. A column that mixes numbers with anything else is staged as text, which is what keeps `007` and `1.00` intact, but the driver's own value was bound into it and SQLite rendered it: a result printed as `100000` came back from the file as `100000.0`. Such a column now carries the text that was displayed.
* A Parquet file's declared column types survive the import. Parquet states the type of every column in its own schema and sqly discarded it, so every column arrived as TEXT and carried TEXT affinity into every comparison: `MAX(price)` returned the lexicographically largest value, `ORDER BY price` sorted digit strings, and `WHERE price > 100` compared `100` as `'100'` and matched rows below it. The same rows loaded from CSV, where types are inferred from the values, answered correctly — so one dataset gave two answers depending on its format, with no error and no warning. Fixed in filesql v0.35.0.
* A Parquet export carries the types of the result it exports. Every column was written as `STRING`, so the next tool — which reads the schema, because that is what a typed format is for — received numbers as digit strings. A column is written as `INTEGER` or `REAL` when every value in it is; a column that mixes types stays `STRING`, because SQLite types values rather than columns.
* An export that cannot write a value writes nothing at all. LTSV checked each value as it wrote it, so a tab in row 500 left 499 rows on stdout and then failed, and JSON opened its array before encoding the first row. Either way the reader on the other end had already taken a truncated document for a complete one, and only the exit code said otherwise. Writing to a file was always atomic because it stages; stdout was the destination without the guarantee.
* A syntax error in a script that contains `.save` is reported as a syntax error. The preflight that refuses statements a save cannot persist treated every statement it did not recognize as a schema change, so a typo was explained as one — `.save cannot persist "SELEC bad": it changes schema` — and the real SQL error never reached the user. The refused statements are named now, and anything else is left to run and to fail with SQLite's own message.
* A statement after the last `.save` no longer blocks the save. The preflight examined every statement in the script, including the ones a save cannot reach, so a script that saved and then built a scratch table was refused outright and the save it asked for never happened.
* A write-back refusal says which of the three reasons it hit. A source can be unwritable because sqly cannot write that format, because bzip2 has no writer, or because a compressed Parquet cannot be rebuilt — and all three said `write-back to data.csv.bz2 is not supported (use csv, tsv, ltsv, or parquet)`. That advice is right for a JSON source and wrong for the other two: a `.bz2` CSV is already a CSV and a `.parquet.gz` is already a Parquet, so the reader was told to use what they already had. Each reason now names itself and points at `.dump`, which is what actually works for all three.
* Checking a JSON export before writing it no longer encodes every row twice. The check that keeps a failed export from leaving a partial document on stdout was written as a trial encoding, which made a 200,000-row `--output-format json` run about a fifth slower. It is a type test now, sharing one decision with the writer so the two cannot disagree about which values are writable.
* A query whose rewritten expression already carried an implicit alias runs again. `--dialect mysql --sql "SELECT CONCAT(a,b) z"` came back as `strict_concat(a,b) z AS "CONCAT(a,b) z"` — two names for one column — and failed with a syntax error near `AS`. The label pass read a closing parenthesis as an operator and so took the name after it for part of the expression. Only the implicit form was affected; `AS z` was always detected. Fixed in filesql v0.35.1.
* A dot-command written wrong exits `2`, whichever command it is and whichever way it is wrong. Only `.import` did, and its own comment said it was keeping malformed input "in the usage class with every other 'you typed it wrong'" — which none of the others were: a missing argument, an extra one, an option a command does not have, and an argument given to a command that takes none all exited `1`, the code for a statement that ran and failed. Nothing ran in any of them. What a command does once its arguments are accepted is classified on its own, so `.cd` to a directory that is not there still exits `1` and a `.save` that cannot write still exits `4`. `.dialect` was missed in that pass — `.dialect oracle` and `.dialect a b` kept exiting `1` — and follows now.
* A remote server can no longer rename the table a URL produces. The staged download took its name from a `Content-Disposition` header or from wherever a redirect landed, in preference to the URL the caller typed, so `sqly --allow-remote --sql "SELECT * FROM sales" https://host/sales.csv` answered "no such table: sales" — and which name to use instead was only discoverable by running `--inspect` first. `--inspect` said so itself, reporting `source` as the URL and `name` as the server's choice. A URL that names a supported file now names the table; where the URL carries no filename, the server's hints still decide, as before.
* A table-name collision involving standard input names it `stdin`. The staged `--stdin-format` dataset lives at a random temp path, and the collision quoted that path: an answer nobody could act on, since the reader cannot rename that file and it is gone by the time they read about it. Import failures already scrubbed it; the checks that run before the import now do too.

### Documentation

* The `.save DIR` permission claim is corrected. README and the getting-started page said the permissions of each source are preserved "either way"; that holds for `.save --in-place`, which overwrites a file that already has them, but a copy into a directory is a new file and is created `0600`. The reference page always said so.
* The rc5 note no longer claims json carries a value unchanged. JSON is Unicode by definition and cannot hold a byte that is not valid UTF-8; `encoding/json` writes `U+FFFD` for one, which is deliberate — it is the signal that a file was read with the wrong `--encoding` — but it is not "unchanged". csv and tsv are the formats that carry such bytes exactly.

## [v1.0.0-rc5](https://github.com/nao1215/sqly/compare/v1.0.0-rc4...v1.0.0-rc5) (2026-08-06)

A release candidate for v1.0.0, not the final one. Everything below is a change
from rc4.

The theme of this one is the interactive shell, and what an export does with a
value it cannot write. Ctrl-C could not reach a running statement and ended the
session instead of the line; a ";" was a terminator wherever it appeared, so a
trigger could not be typed and a trailing comment was never submitted; a line
holding several statements printed one of their results. Two exports answered an
unwritable value by failing with a parser error nobody could act on, or by
changing the value and saying nothing. And a URL's password was printed back on
both streams.

### Breaking Changes

* Exporting to Excel refuses a value XLSX cannot carry, where it used to change it. XLSX is XML, and XML 1.0 has no way to write a control character other than tab, newline, and carriage return, nor the noncharacters `U+FFFE` and `U+FFFF`; the writer substituted `U+FFFD` for the rest, so the export succeeded, the file appeared, and the byte was gone. It now exits `4`, names the character, and leaves the destination exactly as it was — the contract every other format already followed, and the one the exit-code table already documented. A pipeline that exported such data and got a file now gets a failure; csv and tsv carry the same values unchanged.
* A password carried in a remote URL is redacted in everything sqly prints. `--inspect` wrote it into the `source` field on stdout — the document people commit, attach, and paste — and every message about a download repeated the URL as given on stderr, alongside Go's own error, which redacts it. All of them now show `user:xxxxx@`. A program that read `source` and re-fetched from it has to keep the URL it passed in; the redaction is a display of the source, not a handle to it. Command history still records the line as typed, so the up-arrow returns something runnable.

### Bug Fixes

* Exporting to Parquet no longer fails on a value SQLite cannot parse as a literal. The export stages the result in a temporary database, and it built that INSERT as SQL text with every value quoted into it, so a NUL byte — which ends a statement as far as SQLite's tokenizer is concerned — left the literal unclosed: a CSV carrying one exported to every other format and failed Parquet with `unrecognized token`, naming a token nobody typed. Values are bound now, which also parses the statement once for the export instead of once per row.
* A result printed for a statement typed across a continuation line keeps its last line. The prompt that followed it erased one row per row the entry had occupied, so a two-line statement ate the last line of its own result — a table lost its bottom border, while the same query typed on one line kept it. Fixed in prompt v0.0.17.
* Ctrl-C now stops a statement that is already running. The prompt holds the terminal in raw mode, where Ctrl-C is a byte rather than a signal, and between prompts nothing was reading it: the key could not reach a running statement at all. It waited in the input buffer while the query ran to completion, however long that took, and was then read as the next line. A canceled statement rolls back and the session carries on, so canceling is not a failure and the session still exits 0. Needs prompt v0.0.16.
* Ctrl-C now throws away the line being typed instead of the session. It ended the shell with exit code 1, so a half-typed query could not be abandoned, and pressing it to give up on a long-running statement took the shell down once that statement finished — along with anything typed after it. It now clears the line and prints a fresh prompt, the way sqlite3, psql, and mysql answer it.
* A ";" now ends a statement only where SQLite ends one. The shell asked whether the buffer ended with ";", so it submitted a fragment for every ";" inside a string literal (`SELECT 'a;`), inside a quoted identifier, or inside a trigger body, where it separates the body's own statements — `CREATE TRIGGER ... BEGIN ... END;` could not be typed at all — and it refused to submit a finished statement carrying a trailing comment (`SELECT 1; -- note`), leaving the prompt waiting for nothing. The same scripts always ran through `--sql-file`, which uses the scanner the interactive shell now uses.
* A line holding more than one statement now runs all of them and prints every result. The whole line went to the engine as a single query, which ran the statements but kept one result: pasting `SELECT 'a'; SELECT 'b';` printed one table and silently dropped the other, and `CREATE TABLE ...; INSERT ...; SELECT ...;` printed no rows at all.
* Pasting SQL that contains a TAB no longer loses it. The TAB ran completion instead of reaching the buffer, so it vanished from the pasted text and, when a candidate matched, rewrote the word it followed. Pasted CRLF line breaks no longer become blank lines, and a control byte carried in pasted text no longer ends the session. Fixed in prompt v0.0.15.
* Pressing Escape no longer eats what is typed next. The prompt consumed up to three runes after it whatever they were, so Escape followed by `SELECT 1` ran `ECT 1`; Escape now closes the completion popup, which nothing could dismiss before. Fixed in prompt v0.0.15.
* Editing an earlier line of a buffered statement no longer walks the prompt up the screen. Pressing the left arrow across a line break, and every keystroke after it, redrew the prompt one row too high and took a line of scrollback with it: after a dozen keystrokes sqly's own banner was gone. The prompt library erased the block it had drawn by moving up its height, which is where the cursor is only while it sits on the last line. Fixed in prompt v0.0.14, which also stopped measuring the terminal by writing to it and reading the reply back out of the input stream — a keystroke typed during a redraw could be consumed with that reply, and a terminal that did not answer cost 100ms of every redraw. sqly's interactive pty suite runs in a second where it took over a minute.

## [v1.0.0-rc4](https://github.com/nao1215/sqly/compare/v1.0.0-rc3...v1.0.0-rc4) (2026-08-05)

A release candidate for v1.0.0, not the final one. Everything below is a change
from rc3.

The theme of this one is what reaches stdout and what a message claims. A
session setting was printing control lines into the data a program parses, a
statement was being refused for quoting that belongs to the shell rather than to
SQL, a single-statement run reported itself as a batch, and a flag that could
not apply was accepted and discarded. Each is a case where sqly said or did
something other than what the caller asked for.

### Breaking Changes

* `.dialect` writes its lines to stderr instead of stdout. `dialect set to mysql` and `current dialect: mysql (available: ...)` landed on stdout, so a script that named its dialect broke every machine-readable format it was run under: `sqly --output-format json --script-file d.sqly t.csv | jq .` failed to parse, and csv, tsv, ltsv, and jsonl carried the same line as if it were data. sqly's own contract says a format a program parses keeps stdout to data alone, and `.mode` and `.row-mismatch` already followed it. A script that captured stdout to read the confirmation has to read stderr.
* `.mode` and `.row-mismatch` with no argument report the setting in effect and succeed, where they used to fail the run. The three session settings — `.dialect`, `.mode`, `.row-mismatch` — now answer an argument-less call the same way: one line on stderr, exit 0. The failure existed to stop a script that meant `.mode csv` from continuing silently in the wrong mode; a mode name that is wrong is still rejected by name, and what the failure actually caught was someone asking a question. A script that relied on a bare `.mode` failing now continues.
* `--inspect` refuses an explicit `--dialect` with exit 2. `--dialect` translates user SQL and `--inspect` runs none, so the flag was accepted and discarded in silence — the same silent discard `--output-format` was already rejected for. The default nobody typed stays silent, so `sqly --inspect data.csv` is unchanged.
* `.mode` accepts a format name in any case, and with surrounding whitespace: `.mode CSV` selects csv where it used to fail with `invalid output mode "CSV"`. `--output-format` already normalized both, so the same setting was reachable through one spelling on the command line and not through it in the shell. The banner names the mode rather than the string that was typed, so `.mode CSV` and `.mode csv` print the same line. A script that relied on `.mode CSV` failing now runs it.
* `excel_sheets[].source` in the `--inspect` report is now an absolute path, the same string as the `tables[].source` of the tables that workbook produced. It used to be the path as it was typed, so `sqly --inspect book.xlsx` named one file `/abs/path/book.xlsx` in one array and `book.xlsx` in the other, and a consumer holding both could not tell they were the same workbook. A remote workbook still reports the URL it was downloaded from, unchanged. The order of `excel_sheets` follows the normalized value, and a workbook imported twice under two spellings is now one source in the report rather than two. `schema_version` stays `1`: the field's type and meaning are what the schema always described, and this is the implementation catching up to its sister field. A consumer that keyed on the relative path breaks.
* Four kinds of failure that exited `1` now exit `2` or `4`. `1` means a statement ran and failed, so it told a wrapper to fix the SQL; none of these four is a SQL problem. An `--output` or `.dump` destination that is one of the run's source files exits `4` (the collision the exit-code table already named). A value or column set the chosen output format cannot represent — a tab inside an LTSV field, two result columns of the same name in JSON — exits `4`, whether the result was going to a file or to stdout, because what has to change is the format or the destination. An `--output-format` that contradicts the destination's extension exits `2`, and is now decided from the command line before any input is read, so a run with a missing input still reports the conflict rather than the missing file; the same contradiction inside a script, where `.mode` is session state, exits `2` at the statement that hits it. `--inspect` with nothing to inspect exits `2`, which is where an agent expanding an empty file list into `sqly --inspect $FILES` lands. A wrapper that branched on `1` for any of these has to branch on the new code.

### Bug Fixes

* A SQL statement whose comment holds an apostrophe runs. `sqly --sql "SELECT 1 AS x -- don't panic" t.csv` failed with `unterminated single quote in command` and exit 1, and so did `/* it's fine */` and a `-- say "hi`. Every input was being split by the shell's own quoting rules before anything looked at whether it was a dot-command, so SQL was judged by a grammar that is not SQL's: an apostrophe inside a comment is text, and the statement never reached SQLite. Only a line beginning with `.` is parsed that way now, and a dot-command with an unterminated quote is still rejected exactly as before. The same statement failed on every path — `--sql`, `--sql-file`, `--script-file`, a piped script, and the prompt — and runs on all of them.
* The command history records a line before it is parsed, so an input sqly rejected is still one up-arrow away. It used to be recorded only after parsing, which lost exactly the lines most worth recalling.
* A `--sql` failure is reported as one statement failing, not as a batch. `--sql` accepts exactly one statement, and its failure was framed by `batch statement 1 failed at line 1:` and closed with `batch stopped: statement failed` — two lines about a script the user did not write, with the statement text printed twice between them. The report is now the error itself. A `--sql-file`, a `--script-file`, and a piped script still name the statement number and its line, because with several statements there is no other way back to the one that stopped the run.
* The dialect warning is printed at the first statement it applies to, rather than at startup. It reached runs that translate nothing — a `--script-file` of only dot-commands — and in the interactive shell it arrived before sqly's own banner. The documentation described the deferred behavior; the implementation now matches it.
* `.dump` writes through the same steps as `--output` and `.save`, so an existing destination is either the old file or the new one. Serializing was already safe — the exporter stages into a temp file and opens the destination only once that has succeeded, so a format that rejects a value part-way leaves the file alone — but the step after it was not: that temp file is in the OS temp directory, so reaching the destination was always a truncate and a copy, and a copy that fails half way (a full disk, an I/O error) left the destination holding whatever had reached it, with nothing to put back. The last step is now a rename inside the destination's own directory, with a copy only where the platform refuses a rename and a backup taken before it runs.
* The `affected is N row(s)` count is reported in every output format. In `csv`, `tsv`, `ltsv`, `json`, and `jsonl` it was not printed at all — a run that changed rows said nothing about how many. It goes to stderr in those formats, so stdout still carries only the data a program parses, and stays on stdout in `table`, `vertical`, and `markdown`, where a person is reading it.
* An import failure is reported once. It was printed by the import and then printed again by whoever received the error, so every failing run said the same sentence twice — and a script's failure said it twice with the line number attached to only one of them. The line number is unaffected.
* A failing import names the file once and the loading library once. `failed to import file rm.csv: load file "rm.csv": filesql: parsing failed: failed to stream file rm.csv: filesql: column count mismatch: row 2 has 2 fields, want 3` had `rm.csv` in it three times, because both sqly and filesql wrapped the failure with the path. The path now travels beside the error as a value instead of inside its text. The filesql half is fixed upstream in v0.33.0, not worked around here.
* A `file://` URL is told to drop the prefix rather than to download something. "Download the file first and pass its local path" is advice that cannot be followed for a file that is already on this machine: `cannot import file:///etc/hostname: sqly takes local paths directly; drop the "file://" prefix and pass /etc/hostname`. Every other scheme keeps the advice that does apply to it.

### New Features

* A query against a table the session does not have now lists the ones it does: `hint: this session has no table "staf". Available tables: ident, staff.` followed by a link to the table naming rules. sqly's premise is that a file is a table, but the name is derived rather than given — a hyphen becomes an underscore, a leading digit gains a prefix, a workbook sheet becomes `file_sheet` — so the failure a caller hits most was the one that said least. Twenty names are listed, then `... (N total)`. A session holding nothing is told how to get a table instead. A missing column gets a line of the same kind, naming `.describe TABLE` and `sqly --inspect FILE`. Both go to stderr, next to the error they explain; stdout stays empty and the exit code stays `1`.
* An import stopped by the default `--row-mismatch error` names the two policies that would have let it through. `pad` already named the flag it was refusing; the default, which is what everybody meets first, said only that the field counts differed. An `.import` typed into a running session is offered `.row-mismatch` instead, because a flag can only be passed when the process starts.

### Documentation

* `examples/join.sql` and `examples/data/regions.jsonl` make the cross-format join runnable from a clone: one query over a CSV and a JSONL, with the JSONL's fields read out with `json_extract`. It is sqly's headline capability and had no executable example. `examples/README.md` also shows `--inspect` piped into `jq`, which is what the report exists for and which nothing in `examples/` or the cookbook demonstrated. The E2E suite runs every command the file shows, including the join's exact output and the fact that it fails on a missing table when given only one of its two inputs.
* The reference documents the rules `--output` follows: a known format extension picks the format, an unknown one is written to exactly as typed, and a path with no extension gets the format's own appended. `--output report` writes `report.csv`, which is what an agent reading `report` afterwards needs to know. The path actually written is on stderr, and reading it beats reproducing the rules.
* CSV and TSV have no headerless mode, and the formats page now says so. The first line is always the header, so a headerless file loads with its first row consumed as column names — a row disappears and the columns are named after its values. `--inspect` reports the same thing, so it answers with one row fewer than the file holds. No flag and no heuristic was added: the workaround is to put a header in front of the data, and it is shown in the formats page and the cookbook.
* The formats page lists the two inputs sqly refuses outright — a CSV whose header repeats a name, and a file of zero bytes — with their exit code, and contrasts them with a header-only file, which is a table with no rows.
* Getting `--encoding` wrong is a successful run, and the formats page now says so. Bytes the decoder cannot read become `U+FFFD`, the query runs, and the process exits `0` with nothing on stderr. The replacement character is the signal, and `sqly --output-format json` writes it escaped so a script can grep for it.
* The dialects page states that a translated expression is labeled with the SQLite form it was rewritten into — `postgresql_cast(salary, 'text')` as a JSON key — and recommends `AS` for output something else will read.
* The reference documents `--`, which ends the flags and is the only way to name a file whose name begins with `-`.
* The dialects page's rejection example is asserted against the binary. The page showed the error alone, without the translation warning that comes before it.
* The shell page describes the session settings as one family: what an argument-less call reports, and why every one of those lines goes to stderr.
* The reference's `--sql-file` example quoted a message sqly stopped printing several releases ago, advertising a recipe that no longer applies. Every message quoted verbatim in the documentation is now pinned by an E2E scenario that asserts the binary prints that exact string, and a drift test requires both to hold the same text, so neither can move alone.

### Maintenance

* Completion answers for the command being typed. Every dot-command's argument was completed against everything sqly knows — `.dialect` and TAB offered fifty-five candidates, `.dialect m` answered `markdown` beside `mysql`, `.mode m` answered `mysql` beside `markdown`, and `.row-mismatch s` answered `SELECT` and `SET` but never `skip`, which was not in the candidate set at all. A command's argument now completes against that command's values: dialects after `.dialect`, formats after `.mode`, the three policies after `.row-mismatch`, table names after `.header`, `.schema`, `.describe`, and `.dump`. A command that takes no argument offers nothing rather than falling back to SQL keywords.
* Completion candidates come back in a fixed order. The helper commands live in a map and were ranged over directly, so Go's map iteration reshuffled the list between two calls in one process: `.cd` led the candidates on one keystroke and trailed them on the next.
* The dialect names are read from one list. They were written out by hand in the shell's completion, in `.dialect`'s own constants, and in the `--dialect` error message, while `.dialect`'s error already derived its list, so a dialect added upstream reached only the copies someone remembered. All of them now come from `dialect.Dialects()`, and the spelling a person reads (`PostgreSQL`, not `postgresql`) comes from `Dialect.DisplayName()`, added in filesql v0.34.0 so it lives beside the list it belongs to. A test fails if any dialect is missing from any of the three lists.
* A handful of wrappers that only forwarded are gone: `mode.displayName` and `mode.AllowsMultipleResults` both called straight through to the embedded `PrintMode`, `Table.printExcel` called `printCSV`, and `CommandList.hasCmdPrefix` was a method that never touched its receiver. The `sort` package calls become `slices`.
* Path completion reads a directory once. A path can be typed two ways — backslash-escaped, or inside quotes — and each had its own copy of the whole thing: decode the prefix, split it at the last separator, read the directory, skip hidden entries unless a dot was typed, keep what matches, render a suggestion. Only the first and last steps actually differ between them, so those are what is left separate; the directory read and the filtering happen in one function, and a rule added to completion cannot now apply to one style and not the other. The two prefix splitters, and the two loops that decode a backslash escape, merge the same way. `filterHasPrefix` also loses the parameter it was documented to ignore.
* `.mode` and `--output-format` resolve a format name the same way, from one registry in `domain/model`. A name like `csv` used to be written down in four unrelated places — the `--output-format` parser, the `.mode` parser, `PrintMode.String`, and the shell's completion list — and nothing made them agree, so a format could exist for one question and not another with the build staying green. `--stdin-format` had the same split in two: the set the flag validated against lived in `config` with a comment asking the reader to keep it in step with the extension map in `shell`, and a name in the first but not the second passed validation and then failed to import for a reason that named neither the flag nor the format. Both are single tables now, and the help text, the error messages, and the completion list are read off them.
* The walk over SQL text that tells code from string literals, quoted identifiers, and comments existed five times, in two packages. Every question sqly asks before running a statement needs it — where does this statement end, does this INSERT have a RETURNING clause, which verb do these CTEs feed, is the next line still inside a comment — and each answer had its own copy, so a fix to the way one handled a bracket-quoted identifier or a doubled backtick reached that copy alone. They are now one `domain/sqltext` package, a dependency-free leaf beside `domain/cleanup`, and the five call sites are three to ten lines each. The scanner also stopped converting every statement to `[]rune` first: everything it looks for is ASCII, and UTF-8 never puts an ASCII byte inside a multi-byte sequence, so it reads bytes and hands callers byte offsets they can slice with. Behavior is unchanged, and the test cases the five copies each carried are now one suite over the one scanner.
* Around 250 lines that nothing in sqly reached are gone, along with the tests that were keeping them alive. `interactor.SQL` carried four keyword tables and twelve predicates (`isDDL`, `isSelect`, `isWithCTE`, ...) that no caller ever asked; the one question the interactor does ask, "does this statement return rows", reads the statement itself, so the type now holds nothing. Also removed: `shell/extension.go` in full, the `importFile`/`importDirectory` wrappers over `runImport`, `scriptModifiesData`, `CommandList.sortCommandNameKey`, `model.NewTextCell`, `model.NullCell`, `model.NewRecord`, `filesql.NewDecompressingReaderForFile`, `infrastructure.ErrNoLabel` with the LTSV splitter that raised it, and `cleanup.Context`, which described a policy for cleanup after cancellation that no cleanup site had ever adopted. `config.NewInMemHistoryDB` moved to `testutil`, where the only thing that wants history it can throw away already lives. Nothing user-visible changes: no flag, no output, no exit code.

## [v1.0.0-rc3](https://github.com/nao1215/sqly/compare/v1.0.0-rc2...v1.0.0-rc3) (2026-08-05)

A release candidate for v1.0.0, not the final one. Everything below is a change
from rc2.

The theme is the same in all four changes. sqly is run by wrappers, CI jobs, and
LLM agents at least as often as by people, and three of its defaults did
something the caller had not asked for: `--inspect` printed the data when it was
asked for the schema, a URL on the command line was fetched because it was
written there, and a non-SQLite dialect ran on SQLite without saying so. None of
the three is a silent change now — the old command either fails loudly or says
one line on stderr.

### Breaking Changes

* `--inspect` is schema-only by default. `--inspect-sample` now defaults to `0` instead of `5`, so a report describes what a file holds without printing what is in it. `--inspect` is the command something reaches for when it has been handed a file nobody has read yet, and "tell me what this is" answering with the contents is a leak nobody asked for. Row data comes back with `--inspect-sample N`, capped at `N` rows per table. `sample_rows` is still present and still an array — `[]` rather than absent or `null` — so a consumer that iterates it sees zero rows instead of failing on a missing key. `--inspect` and `--inspect-sample 0` now produce byte-identical documents.
* A negative `--inspect-sample` is rejected while the command line is parsed rather than when the report is built, so it exits `2` having read nothing instead of exiting `1` after the import had already happened. There is no new upper bound: a count larger than the table is the table.
* Remote input is default-deny. An `http` or `https` URL is downloaded only when the session was given `--allow-remote`; without it the run exits `2` before any HTTP request is made, having imported nothing, created no temporary directory, and written nothing to stdout. The capability covers every entry point — positional arguments to a query, `--sql-file`, `--script-file`, `--inspect`, and the interactive shell, plus `.import URL` typed at the prompt, piped in, or read from a `--script-file`. A script is checked whole before its first statement runs, so a refused script has executed nothing; at the prompt the command fails and the session continues unchanged. A session started with the flag keeps the capability for the `.import` commands typed later in it, and passing it on a run with no URL is not an error. `--allow-remote` is an explicit network capability, **not a sandbox and not an SSRF defense**: it decides whether sqly makes a request, not where the request may go, and it lifts none of the existing limits (http/https only, five redirects, the redirect-scheme check, the header and transfer timeouts, the 2 GiB body cap). What it gives a wrapper that fixes sqly's argument list is a way to turn sqly's own downloading off; it is no defense against a caller that can add flags itself.
* The `--inspect` report gained two top-level fields, `schema_version` (the JSON number `1`) and `sqly_version` (the string `sqly --version` prints). They are additive, so a consumer reading only `tables` is unaffected, but the document a program parses is no longer the same shape it was. Branch on `schema_version`; report `sqly_version`. Because `sqly_version` moves between releases, the report's bytes are not stable across versions — what is stable is that the same binary, the same inputs, and the same options produce the same bytes.

### New Features

* `--allow-remote` grants this session permission to download the `http(s)` input it is given. It is session state seeded from the flag, not a package-level switch or an environment variable, so a capability granted to one invocation cannot leak into another.
* Choosing a non-SQLite `--dialect` now says what that means, once, on stderr: `Warning: PostgreSQL syntax is translated to SQLite; execution uses SQLite semantics, not PostgreSQL semantics.` Choosing `--dialect postgresql` looks like choosing PostgreSQL and is not — the syntax is rewritten and the engine underneath is SQLite, so a query SQLite accepts runs and one whose meaning differs between the engines answers differently with nothing to say so. It is printed at most once per session: before the first statement of a `--dialect` run, or at the moment `.dialect` switches in the shell. Switching back to SQLite and out again does not repeat it. `sqlite`, `--help`, `--version`, `--inspect`, and a rejected command line print nothing, stdout is never touched, and no exit code changes. A wrapper that treats any stderr output as a failure is the one thing to check.
* The `--inspect` contract has a formal JSON Schema (Draft 2020-12), published at [`/sqly/schema/inspect-v1.schema.json`](https://nao1215.github.io/sqly/schema/inspect-v1.schema.json). It is the single canonical copy — there is no second copy in the repository or in the documentation to drift from it — and sqly's own tests validate real `--inspect` output against it, at the unit level and against the shipped binary. The compatibility policy that decides when `schema_version` moves is stated on the reference page: additive fields do not move it, and a v1 consumer must ignore fields it does not know.

### Documentation

* A migration guide, `doc/migration.md`, gives the before and after for each breaking change as commands to copy, and says what happens to a caller that changes nothing.
* The reference documents the inspect report's two version fields, the schema-only default, the compatibility policy, and the link to the formal schema, and no longer says the sample defaults to five anywhere.
* The formats page documents remote input as default-deny and gains "What `--allow-remote` is not", which names the six things the capability does not protect against — sandboxing, localhost, private ranges, cloud metadata endpoints, DNS rebinding, and proxies — because a reader who takes it for an SSRF defense is worse off than one who has never heard of it.
* The dialects page keeps "This is translation, not emulation" and now states the runtime warning alongside it: once per session, on stderr, never on stdout.
* Every documented `http(s)` example in the README, the reference, the formats page, the getting-started page, and the cookbook passes `--allow-remote`, and the recorded HTTP demo was re-rendered with it. A drift test walks every documented sqly command and fails on a URL invocation that omits the flag, so a stale example cannot be added later.
* The GoReleaser release header and the nFPM and Homebrew package descriptions name the formats sqly actually reads. They advertised CSV, TSV, LTSV, JSON, and "Microsoft Excel™" while sqly had read JSONL, Parquet, ACH, Fedwire, and eight compression wrappers for several releases.
* The v0.30.0 benchmark on the about page and in the README is marked as a historical measurement rather than a performance guarantee for the current release. It has not been re-measured; saying so is more honest than a number nobody has checked since.

### Maintenance

* Drift tests derive each new claim from the implementation rather than from a copy of the documentation: the schema version is compared across the code, the JSON Schema, and the reference page; the inspect default is compared across the parser, `--help`, and the reference; and the Pages deploy verification checks the live site for the new contracts.

## [v1.0.0-rc2](https://github.com/nao1215/sqly/compare/v1.0.0-rc1...v1.0.0-rc2) (2026-08-04)

A release candidate for v1.0.0, not the final one. Everything below is a change
from rc1.

### Breaking Changes
* Added `--script-file FILE`, and `--sql-file` now points at it when it rejects a dot-command. `--sql-file` still holds SQL only; a script that mixes SQL and dot-commands has an entry point of its own instead of only working when piped. `--sql`, `--sql-file`, and `--script-file` are mutually exclusive, and `--output` does not apply to `--script-file` — a script writes files with `.dump`, where the destination means something.
* An unsupported `--stdin-format` value is rejected while parsing rather than when stdin is staged, so it exits `2` as a usage error instead of `1` after the run had started.
* Exit codes now classify the failure instead of reporting every one as `1`. A bad command line or an unrunnable script exits `2`, an input that could not be read exits `3`, a destination that could not be written exits `4`, and a statement that ran and failed still exits `1`. A wrapper that only checks for non-zero is unaffected; one that checks for `1` specifically has to widen the check. The class is decided from the failure itself, so a `.save` that cannot write exits `4` inside a script exactly as it does on its own.
* `.save --in-place` refuses a symlinked source unless `--follow-symlinks` is given. Following a link is still the only correct way to write through one — a rename would replace the link and leave the real file holding the old rows — but it overwrites a path the user never typed, which can sit outside the directory they are working in. The refusal names the link and what it resolves to; the opt-in prints the resolved path to stderr before writing. `.save DIR` is unaffected and rejects the option as meaningless there.
* An Excel workbook now contributes only the sheets it shows. A hidden sheet usually holds the spreadsheet's own working-out rather than data anyone meant to publish, and turning it into a queryable table surprises whoever opens a file they did not build. `--include-hidden-sheets` imports them, hidden and very hidden alike, and is a session setting a later `.import` keeps. An import that skipped sheets says how many, on stderr, without naming them; `--inspect` names them. A run whose inputs are all known and hold no workbook rejects the flag, as `--encoding` and `--row-mismatch` already do; a shell with no inputs accepts it, because the `.import` it is for has not happened yet.
* SIGTERM now exits `143` where it used to exit `130`. `130` is `128+SIGINT` and `143` is `128+SIGTERM`, which is what a shell reports for a process each signal killed. Reporting both as `130` made "someone pressed Ctrl-C" indistinguishable from "the surrounding system took the run away" — a canceled CI job, a service manager shutting down, a `timeout` giving up — which is exactly the distinction a wrapper deciding whether to retry needs. SIGINT is unchanged.

* Several inputs are now one import rather than a sequence of them. Each path used to load in its own transaction, so a malformed file in the middle left the ones before it in the database and stopped the ones after it from being read: the run exited non-zero while the session held part of what was asked for, with no way to tell which part. Either every input loads or none of them does, and a failed import needs nothing undone — fix the file and run the same command again. This applies to file arguments, a directory argument, `.import` inside a session, and a mix of local files and URLs alike; a download that succeeded before a later failure is rolled back with the rest and its temporary file removed. A startup import that fails no longer opens an interactive shell onto an empty database, and a partial import — which could report "1 of 2 inputs imported" — can no longer happen.
* Two inputs that would create the same table are refused before anything loads, naming both sources and the table they share. Picking one would drop the other's rows in silence. Files in different directories are different inputs even when they share a base name; a file named twice, or named alongside the directory holding it, is one input and is read once.
* `SanitizeForSQL` keeps letters, digits, and combining marks by Unicode category instead of ASCII only, matching what filesql actually names tables. sqly works out table names in advance to detect collisions, and the old rule collapsed every non-Latin file name to the fallback `sheet` — so two files named in Japanese looked like a collision while filesql was loading both under their own names.

### New Features
* SIGINT and SIGTERM cancel the run instead of killing the process. The query is canceled, the deferred cleanup runs, and the temp directories a download or a staged stdin dataset created are removed. A second signal ends the run outright. The interactive shell is unaffected: the prompt reads Ctrl-C as a keystroke.
* `--inspect` reports an `excel_sheets` array for a run whose inputs include a workbook: every sheet, whether the workbook shows it, whether this run imported it, and the table it became. It is the only place a hidden sheet is named. The field is additive and absent when no workbook was read, so a consumer of `tables` sees what it always saw.
* `examples/` holds two files that run against a clone as shown: `report.sql` for `--sql-file` and `update.sqly` for `--script-file`. The E2E suite runs both, so the commands in the README and the cookbook cannot go stale while looking correct.

### Maintenance
* Google Wire is gone. The generated initializer has been replaced by a hand-written composition root in `di/di.go` that calls the same constructors in the same order and releases the same resources; `di.NewShell` keeps its signature, and no CLI or runtime behavior changes. Wire's repository is archived, and sqly's graph is a single path of about a dozen constructors — small enough that generating it cost a build-time dependency on an unmaintained project and bought nothing. `github.com/google/wire` is out of `go.mod`, `tools.go`, `make tools`, and the architecture rules. Contributors no longer regenerate anything after changing how the application is wired; `make generate` remains, and now covers only the gomock doubles it always also covered.
* The composition root has tests of its own: the failure paths close what they had already opened, the success cleanup closes both databases in the reverse of the order they were opened, and an import lands in the same database the query reads — which is the one wiring mistake a type checker cannot catch.
* The corrupt-fixture generator checks its own output more honestly. A truncated JSONL fixture is now verified line by line, since a healthy JSONL file is also invalid when read as one JSON document and the old whole-file check could not tell the two apart. The invalid-parquet fixture keeps a valid `PAR1` header and breaks only the footer, which is what its name claims and what a reader that sniffs the first four bytes would wave through. A `CorruptKind` with no verification case now fails instead of being written out as a fixture nobody checks.

### Documentation
* The formats page opens with a capability matrix: what each format can do for reading, stdin, URLs, compression, tables per file, query results, write-back, and types. The formats differ in all of those, and the page previously described them one at a time.
* The reference documents `--help` and `--version`, which it had never listed despite promising every flag, and gained a section comparing `--sql-file`, `--script-file`, and a piped script.
* CONTRIBUTING points at `website/content/` instead of the mkdocs tree that was replaced by Hugo, and the bug report template asks for the command, the exit code, and `sqly --version` instead of browser steps and a Go version.
* A drift test derives the flag list from the parser and fails when the reference documents a flag that does not exist, or omits one that does. Another fails if the README goes back to stating a flag count; it said twelve while there were thirteen.
* The formats page says what the 2 GiB download limit does not cover. It bounds the HTTP response body and nothing else: a compressed input expands after it lands, an XLSX file is a ZIP whose sheet XML is far larger than the archive, and every imported row then goes into an in-memory SQLite database. Row count, column count, field size, and CPU time are not bounded at all. A URL well inside 2 GiB can still exhaust memory, so a remote input is documented as untrusted data to run where an over-large import is survivable.
* The cookbook's Excel and script recipes match what sqly does. "Every sheet is imported" is gone, and "pipe a script that has dot-commands" is now `--script-file`. The exit-code recipe lists the codes rather than saying "non-zero", and the write-back recipe covers the symlink refusal and `--follow-symlinks`.
* The cookbook gains "Multiple files are one import" and the reference gains "Multiple inputs", both stating that a failed import commits nothing and that colliding table names are refused rather than resolved. The README links to the recipe in one line rather than repeating it.
* The cookbook states the atomicity guarantee as a promise about what is committed rather than about what is touched. It used to promise that a failing input stopped the later ones from being read at all, which is more than sqly does: inputs are resolved before the load, so a later URL may already have been downloaded when an earlier file turns out to be unreadable. What holds is that no table and no session metadata is committed, the temporary resources are cleaned up, and the session is left exactly as it was.
* README's "Libraries used" is now "Libraries and tools used", and lists [atago](https://github.com/nao1215/atago): the end-to-end runner that drives the real `sqly` binary from the plain-YAML specs in `e2e/atago/`, run locally with `make test-e2e` and in CI on Linux, macOS, and Windows. It is a development tool rather than something sqly links, which is what the old heading implied.
* Drift tests cover the new claims: both script flags appear in the README with a link to `examples/`, the reference documents `--include-hidden-sheets`, the formats and cookbook pages state the visible-only default, the documented signal codes match `128+SIGINT` and `128+SIGTERM`, the download limit is described as a body limit, and the example files exist, are linked, and are run by the E2E suite. Flag names are matched as whole tokens, so `--sql-file` can no longer stand in for a missing `--sql`.

### v1.0.0 CLI Surface
The surface as of this candidate. It is not frozen yet — the final RC freezes it. This supersedes the list under v1.0.0-rc1, which is left as the record of what that tag promised.

* Input: positional paths (files, directories, `http(s)` URLs), `--stdin-format FORMAT`, `--stdin-table NAME`, `--encoding ENCODING`, `--row-mismatch error|skip|pad`, `--include-hidden-sheets`.
* Query: `--sql/-s SQL`, `--sql-file/-f FILE`, `--script-file FILE`, `--dialect sqlite|mysql|postgresql|googlesql`.
* Output: `--output/-o FILE`, `--output-format table|vertical|csv|tsv|ltsv|json|jsonl|markdown|excel|parquet`.
* Inspection: `--inspect`, `--inspect-sample N`.
* General: `--help/-h`, `--version/-v`.
* Exit codes: `0` success, `1` a statement failed, `2` usage, `3` input, `4` output, `130` SIGINT, `143` SIGTERM.

## [v1.0.0-rc1](https://github.com/nao1215/sqly/compare/v0.31.0...v1.0.0-rc1) (2026-08-04)

The release candidate for v1.0.0. The command surface below is what v1.0.0 commits to; this tag exists so it can be used before it is frozen.

### Breaking Changes
* Removed the standalone `--profile` and `--compare` workflows and their format/key/table flags. Use SQL for data-quality checks and table differences.
* Removed the legacy output flags `--csv`/`-c`, `--tsv`/`-t`, `--ltsv`/`-l`, `--json`/`-j`, `--ndjson`/`-n`, `--excel`/`-e`, `--markdown`/`-m`, `--parquet`/`-p`, and `--vertical`. Use the single `--output-format FORMAT` option instead.
* Removed `--json-typed` and `--ndjson-typed`, and the `.mode json-typed` / `.mode ndjson-typed` shell modes that went with them; `--output-format json` and `--output-format jsonl` preserve SQLite's native INTEGER/REAL/TEXT/NULL values. SQLite has no boolean type, so TRUE/FALSE literals and boolean expressions are emitted as integer JSON numbers `1`/`0`, while TEXT values such as `"true"` remain strings. Zero-padded values remain strings.
* Removed the import cache: `--cache`, and the `--cache-clear` that went with it. A snapshot the user has to name, place, and delete is not part of what sqly does — read files, run SQL, write the result — and it silently did nothing for a piped dataset, a URL, or ACH/Fedwire input. Repeated queries over one import belong in the shell or in a `--sql-file` script, which import once and answer many times.
* Renamed malformed-row policy `fill` to `pad`. `pad` fills short CSV/TSV rows and rejects long rows instead of truncating them.
* Renamed `--stdin FORMAT` to `--stdin-format FORMAT` and `--stdin-name NAME` to `--stdin-table NAME`. The old names read as booleans, or said nothing about the table they name.
* Removed the write-back flags. `--save`, `--save-dir`, `--save-tables`, `--save-in-place`, and `--force` are gone; write-back is the shell's `.save DIR` and `.save --in-place`, which work at the prompt and in a piped or `--sql-file` script alike. Overwriting the files you are reading is the one thing sqly does that cannot be undone, and it belongs after the statements that changed something rather than in the flags that start the run. Removing it from the flag surface also removed eight exclusivity rules that existed only to keep it out of trouble.
* Removed `--sheet`. Every sheet of a workbook is imported, and the table to query — `file_sheet` — is the same either way, so the flag only chose how much work to do while its meaning across several workbooks was ambiguous (a workbook without the sheet was skipped with a warning rather than failing).
* Replaced `--import-mode stop|skip|pad` with `--row-mismatch error|skip|pad`, and the shell command `.import-mode` with `.row-mismatch`. "Import mode" said nothing about what it decides, and `stop` read as "stop and keep what you have" when it means the import fails. The policy applies to CSV and TSV only, which the flag description and the docs now say. The old names and the old `stop` value are rejected, not aliased.
* Removed the `-i` shorthand for `--inspect`; `-i` means in-place in most Unix tools. The long form is unchanged. (`-S` for `--sheet` went with `--sheet`.)
* Renamed the newline-delimited JSON format from `ndjson` to `jsonl`, in `--output-format`, `.mode`, and the `.dump`/`--output` extension it prefers. sqly reads `.jsonl` files and called the input format `jsonl` already; one concept now has one name. A `.ndjson` destination is still recognized and kept as written.
* `--output-format excel` and `--output-format parquet` are now rejected for a `--sql`/`--sql-file` run with no `--output`. They are binary container formats with no on-screen form, and the run used to print CSV instead.
* A script may return several result sets only in a format that can separate them: `table`, `vertical`, and `markdown` print them in order with a blank line between. `csv`, `tsv`, `ltsv`, `json`, and `jsonl` reject the run instead, because two CSV bodies concatenated are one CSV whose third line is a header, and two JSON arrays back to back are not a JSON document. Nothing is printed before the rejection.
* An import option the user typed that cannot apply to any input of the run is now an error. `--row-mismatch` needs a CSV or TSV input; `--encoding` needs a text input. A flag that is accepted and then silently ignored is indistinguishable from one that worked.
* `.save` rejects an argument beginning with `-` that is not `--in-place`. `.save --force`, the spelling this command used before, created a directory named `--force` and reported success while leaving the sources it was asked to overwrite alone.
* `--output` requires its destination's parent directory to exist, and says so before importing anything rather than after the query has run.

### Fixed
* A dot-command after any SQL statement was invisible to the classifiers that decide what a script does. They accumulated every line into one buffer and asked whether it was at a statement boundary, which stopped being true after the first statement ended — so `.save` and `.import` in the natural position, at the end of a script, were never seen. The buffer is now drained as statements complete.
* `--sql-file` no longer hangs when stdin is an open pipe with nothing on it. It peeked at stdin to warn about piped SQL it would ignore, and that peek blocks forever on a pipe that is never written to or closed. A CLI that can hang is worse than one that ignores an input it was never going to read.
* An in-place write-back preserves the source file's permissions. It writes through a temporary file created `0600` and renames it into place, so saving a `0644` CSV silently made it owner-only.
* A failed rollback is reported alongside the failure that caused it, instead of being dropped. When a save fails part-way and a file cannot be restored, that file is now named: it holds content from a run that reported an error.
* A save plans destinations case-insensitively, so two tables whose names differ only in case are rejected everywhere rather than silently overwriting each other on macOS and Windows.
* `CREATE TEMP TABLE` no longer blocks a script that saves. Scratch space is never written back, so building some and then saving the imported tables is allowed; a persistent `CREATE TABLE` is still rejected, because write-back would silently not include it.
* An existing `--output` destination survives a failed write. The result is serialized beside the destination and moved into place, and where the platform refuses that move — Windows will not rename over a file another handle holds open — the fallback copies over the destination, which truncates it before writing. A full disk on the third block therefore left the previous file empty or half-written. The destination is now copied aside the moment the fallback is chosen and put back if the copy fails, and a restore that fails too is reported alongside the failure that caused it.
* A `.save` that fails part-way restores the file it was writing, not only the ones before it. The rollback covered the targets already committed and left out the one whose own commit had just failed — the likeliest of all of them to be holding half a table, for the same reason as above.
* `.save --in-place` writes through a symlinked source instead of replacing it. A rename replaces the name, not the file behind it, so the link became a regular file, the file it pointed at kept the old rows, and sqly reported "Saved". Every step of the write except the rename already followed the link.
* `.save DIR` and `.save --in-place` no longer disable each other. One fingerprint answered both "what does the source file hold?" and "did this session change this table?", and the two stop agreeing the moment anything is written: `UPDATE; .save out; .save --in-place` left the source with its old rows, and reversing the two commands wrote no export at all. They are separate records now, so either order does both jobs, and a read-only or net-zero session still writes nothing.
* Re-importing a file no longer changes what a differently-named table belongs to. Ownership was guessed from the table name — the file's base name plus anything starting with it — so a directory holding `sample.xlsx` and `sample_test.csv` let the workbook claim `sample_test`, and re-importing the workbook made a file the session must not write suddenly writable. Ownership is read from the record made at import.
* `SELECT 9e999` no longer fails the whole JSON output with an encoder error after the opening bracket is already on stdout. An infinity and a NaN are written as the JSON strings `"Infinity"`, `"-Infinity"`, and `"NaN"`, as PostgreSQL's `row_to_json` writes them.
* A BLOB that is not valid UTF-8 is base64-encoded instead of being turned into U+FFFD replacement characters. JSON has no way to hold bytes, and a string that looks fine and cannot be decoded back is worse than one that is obviously encoded.
* An Excel workbook whose sheets map to one table name is refused instead of keeping only the last of them. `Q1 sales` and `Q1.sales` both sanitize to `book_Q1_sales`, so the second sheet replaced the first and its rows were gone with the import reporting success. Fixed in filesql v0.31.0, which resolves every sheet's table name before creating any of them.
* A workbook with no usable sheet says so, instead of reporting a collision with an input that does not exist.
* A script line may be any length. A line over a megabyte was rejected as "not a SQL script", which protected nothing — the script is already a string by then — while breaking a dump's multi-row `INSERT`, a base64 literal, and a minified query.

### Migration Notes
* Replace `--profile`/`--compare` invocations with explicit SQL queries.
* Replace `--json-typed`/`--ndjson-typed` with `--output-format json`/`--output-format jsonl`.
* Replace every removed output flag listed above with `--output-format FORMAT` (for example, `--csv` becomes `--output-format csv`).
* Replace `--import-mode`/`.import-mode` with `--row-mismatch`/`.row-mismatch`, and the policy names `stop` and `fill` with `error` and `pad`. A long row now fails so its extra fields are not silently lost.
* Replace the write-back flags with `.save` in a script: `sqly --sql "UPDATE ..." --save-in-place f.csv` becomes `printf "UPDATE ...;\n.save --in-place\n" | sqly f.csv`.
* Drop `--sheet`; query the sheet's table (`file_sheet`) instead.

### v1.0.0 CLI Surface
The command surface below is what v1.0.0 commits to. Anything not listed is not part of the guarantee.

* Input: positional paths (files, directories, `http(s)` URLs), `--stdin-format FORMAT`, `--stdin-table NAME`, `--encoding ENCODING`, `--row-mismatch error|skip|pad`, `--include-hidden-sheets`.
* Query: `--sql/-s SQL`, `--sql-file/-f FILE`, `--dialect sqlite|mysql|postgresql|googlesql`.
* Output: `--output/-o FILE`, `--output-format table|vertical|csv|tsv|ltsv|json|jsonl|markdown|excel|parquet`.
* Inspection: `--inspect`, `--inspect-sample N`.
* General: `--help/-h`, `--version/-v`.
* `--help` prints those five groups under those names, so the option list and this contract stay the same shape.
* Write-back is not a flag: it is `.save DIR` and `.save --in-place` in the shell, available interactively and in a piped or `--sql-file` script.
* Shell commands: `.cd`, `.clear`, `.describe`, `.dialect`, `.dump`, `.exit`, `.header`, `.help`, `.import`, `.ls`, `.mode`, `.pwd`, `.row-mismatch`, `.save`, `.schema`, `.tables`.
* `sqly` has no subcommands. A positional `help` or `version` is rejected with a pointer to `--help` / `--version`.
* `--sql` runs exactly one statement. `--sql-file` runs every statement and prints every result in order; `--output` requires the run to produce exactly one result and rejects zero or several rather than writing whichever came last.

### Data Contract
* JSON and JSONL output preserves the value's SQLite type: INTEGER and REAL are JSON numbers, TEXT is a JSON string, and SQL NULL is JSON `null`, distinct from the empty string `""`. Text that looks like a number or a boolean stays text, so `"123"`, `"true"`, and `"00123"` are emitted as strings with their leading zeros intact. SQLite has no boolean type, so TRUE/FALSE literals and boolean expressions are INTEGER `1`/`0`.
* A cell's string form and its JSON form are derived from one stored value, so table, CSV, TSV, LTSV, Markdown, JSON, JSONL, and Parquet output of the same result can no longer disagree with each other.

### Maintenance
* Multi-file imports now run as one SQLite transaction. If a later input fails, tables created or replaced by earlier inputs are rolled back while existing tables and views remain unchanged. The filesql dependency includes the caller-transaction loading API required to keep all supported formats in that transaction.
* ACH and Fedwire write-back registries are published only after that transaction commits. A failed import — a bad input, a failed commit, or a rollback that itself failed — leaves no registry entry, so `.dump` can no longer be offered a file it would reconstruct from tables that were rolled away. When several inputs claim the same base name, publication follows input order, so the last one wins.
* A transaction that fails to roll back now reports that failure alongside the error that caused the rollback, instead of discarding it. Both are reachable with `errors.Is` and `errors.As`. Commit, rollback, and their error handling live in one helper shared by the import path and the session query repository, so no call site carries its own cleanup rule.
* A failed commit is reported as a failed commit and nothing else. `database/sql` ends a transaction when `Commit` is called, so the rollback that used to follow could only report `sql.ErrTxDone` — a second failure describing nothing the user could act on. Canceling a query is likewise no longer reported as a broken transaction.
* Cleanup failures are no longer lost when the operation they follow also failed. Closing a staging file, removing a temporary file, and closing an output file all report their failure alongside the original error, so "the export failed and a temporary file is still on disk" is not reported as though only the first happened. The rule lives in one place (`errors.Is(err, cleanup.ErrCleanup)` identifies it).
* Updated the filesql dependency to v0.31.0, which refuses a workbook whose sheets map to one table name. v0.30.4 and earlier in this line: v0.30.2 stages ACH/Fedwire registry entries for publication after commit, rejects long rows under the `pad` policy instead of truncating them, and handles empty JSON/JSONL in the same streaming pass. v0.30.3 applies the same transaction and cleanup rules inside the loader: a failed rollback is reported rather than discarded, a load's transaction is ended exactly once, and a rollback reporting `sql.ErrTxDone` under a canceled context is treated as cancellation rather than a broken transaction. v0.30.4 extends that to the writers: closing a compressing writer, closing an XLSX load's insert statement, and removing an atomic write's staged file all report their failure instead of dropping it.
* Synchronized the CLI, shell help, E2E specifications, website reference, and cookbook with the reduced surface.

### Migration (Go API)
CLI users are unaffected by everything in this section. It applies only to code importing `github.com/nao1215/sqly/domain/model`.

Reading rows — the accessors no longer hand out the table's storage:

```go
// before
for _, record := range table.Records() {
    fmt.Println(record[0])
}

// after: same, but Records() is now a copy — fine, and safe to modify
for _, record := range table.Records() {
    fmt.Println(record[0])
}

// after: no copy, for walking a large result
for _, row := range table.Rows {
    fmt.Println(row.At(0))
}
```

Building a query result — the setters are gone:

```go
// before
t := model.NewTable("t", header, records)
t.SetNulls(nulls)
t.SetJSONValues(values)

// after
t, err := model.NewTableFromCells("t", header, [][]model.Cell{
    {model.NewCell(int64(42)), model.NewTextCell("00123"), model.NullCell()},
})
```

Inspecting errors — a failure may now carry a cleanup failure alongside its cause, so match on identity rather than on text:

```go
// before
if strings.Contains(err.Error(), "rollback") { ... }

// after
if errors.Is(err, cleanup.ErrCleanup) { ... }
```

### Public API
`domain/model` is importable, so these changes affect anyone using it as a library:
* Added `model.Cell` (with `NewCell`, `NewTextCell`, `NullCell`, `IsNull`, `Value`, `String`), `model.NewTableFromCells`, and `model.ErrCellShapeMismatch`. A query result is now one value per cell rather than a display string plus parallel side-tables.
* Removed `Table.SetNulls` and `Table.SetJSONValues`. They injected two more two-dimensional slices after construction, with no check that their shape matched the records, so a mismatch surfaced part-way through an already-written output stream. `NewTableFromCells` rejects a shape mismatch up front and copies what it is given, so a caller cannot mutate a Table after building it.
* Removed `Table.SetJSONTyped` along with the typed-JSON flags.
* No public accessor hands out storage a caller can write through. `Table.Records()` and `Table.Header()` return copies; `Table.Row` and `Table.Rows` return a `RecordView`, which reads but cannot write, so a large result is still walked without copying it. Previously any of them let one assignment make the same result print one value as CSV and another as JSON.
* Added `model.RecordView` (`Len`, `At`, `AppendTo`, `Record`), `Table.Rows` (a range-over-func iterator), `Table.Row`, `Table.RowCount`, `Table.ValueAt`, `Table.ColumnCount`, `Table.ColumnName`, and `Table.Columns`. The zero-copy readers exist so making the copying accessors safe costs nothing on the output path.
* `infrastructure.GenerateInsertStatement` takes a `model.RecordView` instead of a `model.Record`.
* `Table.WithName` clones the header and the row slices, so renaming a column on one table cannot rename it on the other.

## [v0.31.0](https://github.com/nao1215/sqly/compare/v0.30.0...v0.31.0) (2026-07-30)

### New Features
* Vertical Output Mode For Rows Too Wide To Read: `--vertical`, and `.mode vertical` in the shell, print one column per line in a block per record instead of laying the record out along the line. Every other mode fails at the width sqly was written for — a 300-column row is a single 2700-character line as a table, as CSV, as TSV, and as LTSV alike, so no terminal shows it and the column holding the bad value has no name beside it to search for. Vertical spends vertical space, which a terminal scrolls, and puts the name and the value on one short line, so the bad column is one `grep` away. The layout follows psql's expanded output: a numbered record rule, then names left-aligned in a gutter measured in terminal cells, so a full-width Japanese header lines up with an ASCII one. It names no file format, so `.dump` and `--output` take the format from the destination's extension exactly as they do in table mode.

### Bug Fixes
* A One-Column Export Lost Its Empty Rows: in CSV and TSV a record of one empty field, written plainly, is a blank line — and a blank line is not a record, so a reader skips it. `sqly --csv --sql "SELECT v FROM t" --output out.csv` over three rows whose middle value was empty wrote three rows and read back as two, and the export reported success both times. Such a record is now written as `""`, which says "one field, and it is empty". A row of several columns is unaffected, because its delimiters already say how many fields there are. filesql had the same bug on its own dump path and is fixed in v0.29.0, taken below.

* Unfetchable URL Scheme Unreported On Windows: an input written as a URL sqly cannot download named its scheme only when the filesystem answered "does not exist", which is what Unix returns. Windows rejects `s3://bucket/data.csv` as an invalid filename instead — it becomes `.\s3:\bucket\data.csv`, and the drive-letter colon is refused before anything is looked up — so the platform where the raw error is least readable was the one that never got the explanation. The scheme is now checked before the error kind. Found by the Windows suite added below, on its first run.

* Human-Readable Reports Carried Less Than The Machine-Readable Ones: `--compare-format text` printed only counts — "1 added, 1 removed, 1 modified" — while the JSON it was summarizing held the whole diff, so the format meant for a person could not answer "what changed?". It now lists the key of every added, removed, and modified row, and for a modified row the columns whose values differ, so a wide table does not reprint what stayed the same. `--profile-format text` printed every column in file order, so the columns with warnings were buried in the wide CSVs sqly exists for: profiling a 300-column file put the one bad column on line 154 of 303. It now leads with a count and the flagged columns, then still lists every column; a table with nothing wrong says "no warnings" instead of an empty heading.

* LTSV Values Lost The Whitespace Around Them: filesql's LTSV reader trimmed the value, while its LTSV writer wrote it and CSV kept its own, so a value written with two spaces on each side loaded without them: a write-back through LTSV lost the spaces, and the same value read from an LTSV file and a CSV file disagreed. Requires filesql v0.29.0, which also fixes an XLSX export failing for a table whose name Excel cannot use as a sheet name — longer than 31 characters, or holding one of `: \\ / ? * [ ]`, which a name derived from an ordinary file name often is.

* LTSV Columns Came Back In A Different Order Every Run: an LTSV file has no header line, so a table's columns are the labels its records carry, and filesql built that list by ranging over a map. `SELECT * FROM access` on the same 12-column file answered with the columns in a different order on every invocation of sqly, so no query written against column positions was reliable, no export of an LTSV source was reproducible, and a write-back reordered the file's own labels. Requires filesql v0.27.0, which keeps the labels in the order they first appear. That release also fixes the write path sqly exports through: a BLOB was written as the decimal bytes of a Go slice, a `DATE`/`DATETIME`/`TIMESTAMP` column was written in Go's default time layout, an XLSX export could not be written at all, an emptied table could not be written to Parquet, and a dump could write outside the directory it was given.

### Documentation
* sqly Reads And Writes A Pipe, And The Docs Now Say So: the input side was documented — `--stdin`, an `http(s)` argument — but the output side was one line in the last section of the cookbook, so nothing said sqly is a filter rather than a destination. A new [Pipe data out](https://nao1215.github.io/sqly/cookbook/#pipe-data-out) recipe covers `--ndjson` feeding `jq` one object per line, filtering in SQL so `jq` only sees the rows that matter, `--tsv` for `cut`/`awk`/`sort -k`, sqly sitting in the middle of a pipe, a compressed source needing no decompression stage, and the shell rule that a pipeline's status is its last command's — so a failing sqly in `sqly ... | cat` stops nothing unless the shell has `pipefail`. The front page and README now lead with a pipeline instead of burying it.

* Why The Four Dialect Divergences Stay: the dialect page listed the cases where a translated query silently gets SQLite's answer, but only collation said why it was not fixed, which left the other three reading as unfinished work. The page now states the rule — a divergence is fixed when it can be fixed for every dialect that has an opinion about it, because a rewrite that gives one dialect its answer and leaves the others on SQLite's replaces one divergence you can look up with three that depend on which `--dialect` you passed — and gives each of the four its own reason.

### Testing
* atago v0.18.0, Where A Flaky Scenario Fails The Run: the E2E runner already scoped `--retry-failed` to the interactive-pty pass, because retrying every spec had let a real nondeterminism bug surface as "flaky, PASSED". atago now refuses a flaky verdict by default, so the pty pass says out loud that its instability is expected — its sessions lose keystrokes when starved of CPU — by passing `--allow-flaky`. The rest of the suite gets the strict verdict with nothing to opt into.

* Shell Pipelines: `e2e/atago/pipelines.atago.yaml` adds 9 scenarios running real pipelines through the binary — a fixture served over HTTP and piped into `--stdin`, `--ndjson` into `jq`, `--tsv` into `cut`/`sort`/`head`, sqly as a middle stage, `set -e` stopping on a failed query, the pipefail caveat, and the exit code gating a script. Nothing asserted the output side before this.

* Lone Empty Field: `e2e/atago/lone_empty_field.atago.yaml` adds 4 scenarios exporting a one-column result with an empty value to CSV and to TSV, reading each back, and checking that a multi-column row of empty values still writes its delimiters.

* LTSV Column Order: `e2e/atago/ltsv_column_order.atago.yaml` adds 5 scenarios pinning the column order of an LTSV source through `SELECT *`, an export to CSV, and a write-back. Each runs the same command several times, because a single run cannot tell a stable order from a lucky draw.

* Local HTTP Fixtures Wait For Their Port: the three scenarios that serve a fixture over `python3 -m http.server` slept a fixed 0.2s before connecting. A loaded runner can take longer than that to bind a socket, and the client then failed with "connection refused" — a failure that says nothing about sqly, and which the retries above were absorbing. Each now polls the port and proceeds as soon as it answers, which is also faster than the sleep it replaces.

* Retries No Longer Mask A Nondeterministic Failure: `scripts/run_e2e.sh` retried every failed scenario three times and reported a recovered one as flaky, which does not fail the run. That is the right trade for the interactive-shell pty specs, whose keystrokes can be lost when the sessions are starved of CPU, and the wrong one everywhere else: the LTSV order bug above surfaced as "flaky, PASSED" instead of a failure. Only the pty pass retries now.

* Human-Readable Reports: `e2e/atago/human_reports.atago.yaml` adds 7 scenarios pinning the detail lines, that only differing columns are shown for a modified row, that a change between a SQL NULL and the literal string "NULL" is still reported, that an unchanged comparison stays quiet, and that a clean profile says "no warnings".

* Windows Behavior Coverage: the full atago suite now runs on Windows, not only the interactive-shell pty specs. sqly ships for Windows, but 611 of its 622 scenarios had never executed there — write-back, caching, path handling, every format, and the dialects were asserted on Linux and macOS alone. Windows-only defects are invisible from a Unix run: `os.Rename` refuses to replace a file another handle still has open, which is every in-place save. Every scenario skipped there carries a reason, and all of them stay covered on Linux and macOS. Two kinds of thing get skipped: a scenario needing a POSIX shell or a tool that comes with one (an `http.server` fixture, `gzip(1)`, `jq`, `set -e`, command substitution — on Windows `shell: true` is not a POSIX shell, so a `;` arrives as a literal argument), and SQL that needs both quote characters at once, which atago cannot express on Windows because it keeps a backslash literal so a `C:\` path survives.

## [v0.30.0](https://github.com/nao1215/sqly/compare/v0.29.0...v0.30.0) (2026-07-29)

### Bug Fixes
* MySQL And GoogleSQL `CONCAT` Swallowed A NULL: both dialects return NULL when any argument is NULL, but SQLite's own `concat()` treats a NULL as an empty string and the call was passed straight through. `CONCAT(first_name, middle_name)` with a NULL middle name answered the first name where MySQL and BigQuery answer NULL, so a query written to detect incomplete rows reported them as complete. PostgreSQL's `concat()` genuinely does ignore NULLs and is unchanged, as is `CONCAT_WS`. Requires filesql v0.25.0.

* Setting A Mode To Its Current Value Failed: `.mode` and `.import-mode` now accept the value already in effect as a no-op instead of returning an error. In a batch script an error is fatal, so a script that set a mode defensively died on a line that changed nothing, and the natural combination of the flag with the script — `sqly --csv data.csv` fed a script opening with `.mode csv`, or `--import-mode skip` with a script restating `skip` — exited 1 before running a single query. A name that is not a mode at all is still rejected, and a real switch still prints its banner.

* Half-Saved Session On A Failed Write-Back: a `--save`, `--save-tables`, or `.save` run covering several files is now all-or-nothing. Targets were written to their destinations one at a time, and some failures only surface while a file is being encoded (the ACH and Fedwire writers validate as they encode, so a value the format cannot hold is rejected mid-write), so an earlier file was already overwritten when a later one failed. The run exited 1 with the session half-persisted and nothing saying which files had changed, and `--save-tables` had already created part of its output. Every target is now written to a scratch path beside its destination, every destination that already exists is copied aside, and only then are the targets moved into place; a commit that fails partway restores the destinations it already replaced, so a failed save leaves every destination and every source as it was. Windows refuses to rename over a file another handle still has open, which is every in-place save, so there the staged bytes are copied over it instead. A rejected ACH or Fedwire write also no longer damages the file it was overwriting, which used to come back empty (ACH) or deleted (Fedwire). Requires filesql v0.23.0.

* Buffered Statement Looked Like A Hung Shell: the interactive shell now shows `...>` while it is collecting the rest of a statement. A query typed without a trailing `;` is buffered until it is complete, but the cursor simply dropped to a bare line with nothing in front of it, so there was no way to tell the shell was waiting for more input rather than stuck; the recovery (press Enter on a blank line) was documented but invisible at the moment it was needed. A dot-command is always complete and still runs on the first Enter. Requires prompt v0.0.12.

* Non-Latin File Names Lost Their Table Name: a file named in Japanese, Chinese, Korean, Cyrillic, or accented Latin now keeps its name as the table name. Table names were sanitized against the ASCII letter range, so every other letter was deleted: `売上.csv` and `Данные.csv` both became `sheet`, `café.csv` became `caf`, and an Excel sheet named `Café` became `data_Caf`. Two such files in one run therefore collided on the same fallback name and the second failed to import, so the run exited 1 with one of the inputs missing. Table names are always emitted double-quoted, so the restriction bought nothing. Punctuation, symbols, and quotes are still removed, a name starting with a digit still gets a `sheet_` prefix, and a name left with nothing still falls back to `sheet`. Requires filesql v0.22.0.

* Unfetchable URL Reported As A Missing File: an input written as a URL sqly cannot download now names the scheme instead of claiming the path does not exist. `sqly --sql ... s3://bucket/data.csv` (and `file://`, `ftp://`, `gs://`, and the rest) fell through to the local-path branch and failed with "path does not exist", which reads as a typo in the URL rather than a scheme sqly was never going to fetch. It now says only http and https are downloaded, names the scheme it found, and points at passing a local path. A local file name that merely contains a colon, and a Windows drive path, are still treated as paths.

* Silent Read-Only Write-Back: a `--save` or `--save-tables` run whose SQL changed no row now says so on stderr instead of exiting 0 in silence. Write-back deliberately skips a run that left every table as imported, so a read-only query, an `EXPLAIN`, or an `UPDATE` matching no row wrote no file and printed nothing; `--save-tables` in particular looked like it had written the directory, and the absence was only noticed later. It reports "no table data changed in this session; nothing to save", the same note the `.save` command already printed for the same case, and still exits 0.

* SQL Dialect Semantics: queries written in MySQL, PostgreSQL, or GoogleSQL now evaluate the way those dialects do, not the way SQLite happens to. The divergences were silent, so a query returned a plausible wrong answer rather than an error. `5/2` was 2 instead of 2.5 under MySQL and GoogleSQL, and every average or ratio came out truncated. `LIKE` matched case-insensitively under PostgreSQL and GoogleSQL, so a filter caught rows it should not have, and `ILIKE` was indistinguishable from it. A cast applied SQLite's type affinity: `CAST(1.9 AS SIGNED)` truncated to 1 where every dialect rounds to 2, `CAST('abc' AS INTEGER)` answered 0 where PostgreSQL and GoogleSQL raise, `'true'::boolean` collapsed to 0, an invalid date or UUID or JSON document passed straight through, and `SAFE_CAST` returned that fallback instead of the NULL it exists to produce. `DATE_ADD('2026-01-31', INTERVAL 1 MONTH)` rolled forward to 2026-03-03 instead of clamping to 2026-02-28, and adding a day to a date grew a `00:00:00`. MySQL `||` concatenated instead of meaning OR, `HEX(255)` returned the bytes of the string `255`, and `GROUP_CONCAT(x ORDER BY y SEPARATOR '|')` silently joined with a comma. Requires filesql v0.20.0.
* SQL Dialect Coverage: the constructs that used to fail outright now work. Shared: `LEAST`, `GREATEST`. MySQL: `TIMESTAMPDIFF`, `TIMESTAMPADD`, `LAST_DAY`, `UNIX_TIMESTAMP`, `FROM_UNIXTIME`, `MONTHNAME`, `DAYNAME`, `REVERSE`, `FIND_IN_SET`, `FIELD`, `ELT`, `ANY_VALUE`, `STD`, `<=>`, typed date literals, `CURRENT_DATE()`, `POSITION(x IN y)`, `SUBSTRING(x FROM n)`, and the `WEEK`, `QUARTER`, and negative `INTERVAL` forms. PostgreSQL: `x + INTERVAL '1 day'` (its only date arithmetic), typed date literals, `MD5`, `ASCII`, `CHR`, `TRANSLATE`, `SIMILAR TO`, `^`, numeric `TO_CHAR`, four-argument `REGEXP_REPLACE`, `BOOL_AND`, `BOOL_OR`, and the `STDDEV`/`VARIANCE` family. GoogleSQL: `DATE_TRUNC(value, PART)` with `TIMESTAMP_TRUNC` and `DATETIME_TRUNC`, `FORMAT_DATE`, `PARSE_DATE`, `CURRENT_DATE()`, `COUNTIF`, `LOGICAL_AND`, `LOGICAL_OR`, `UNIX_SECONDS`, `TO_HEX`, `IS_NAN`, the `SAFE_` arithmetic family, and `EXTRACT(DATE FROM ...)`.

* SQL Dialect Rewrites In Complex Expressions: a window or filter clause no longer breaks the operator rewrites. `SUM(x) OVER (ORDER BY id) / 2` failed to parse under MySQL and GoogleSQL, and `x / COUNT(*) OVER ()` attached the `OVER` clause to the division helper instead of the count; `FILTER (WHERE ...)` and `OVER window_name` had the same problem. `DATE_ADD(d, INTERVAL n DAY)` with a column or an expression as the amount is valid MySQL and was rejected as a non-literal. Requires filesql v0.21.0.
* SQL Dialect String And JSON Coverage: MySQL `LENGTH` counts bytes rather than characters, as MySQL does, and `CHAR_LENGTH` counts characters. New: MySQL `ORD`, `JSON_UNQUOTE`, and `TRIM(BOTH 'x' FROM s)`; PostgreSQL `BTRIM`, `OVERLAY(x PLACING y FROM n FOR m)`, `JSONB_ARRAY_LENGTH`, and `CHAR_LENGTH`; GoogleSQL `JSON_VALUE`, `JSON_QUERY`, `BYTE_LENGTH`, and `CHAR_LENGTH`. `UNION DISTINCT` works under MySQL and GoogleSQL, and a PostgreSQL array literal is now rejected by name instead of failing on the bracket.

### Testing
* Human-Readable Reports: `e2e/atago/human_reports.atago.yaml` adds 6 scenarios pinning the detail lines, that only differing columns are shown for a modified row, that an unchanged comparison stays quiet, and that a clean profile says "no warnings" rather than printing an empty heading.
* Mode Idempotence: `e2e/atago/mode_idempotent.atago.yaml` adds 7 scenarios covering `.mode` and `.import-mode` set to their current value, both alone and combined with the flag that already selected it, and pins that an unknown name is still rejected.
* Dialect CONCAT: `dialect_cross.atago.yaml` adds a scenario asserting the NULL-propagating result under MySQL and GoogleSQL, that PostgreSQL still ignores a NULL, and that `CONCAT_WS` is untouched.
* Dialect Limits: `e2e/atago/dialect_limits.atago.yaml` pins what `--dialect` does not do — a construct rejected by name, and the four divergences where SQLite's answer differs from the source dialect's with no error (MySQL's case-insensitive collation, `ONLY_FULL_GROUP_BY`, 1-based `SUBSTR`, and casting a boolean to text). Every claim on the dialects page comes from a scenario here, and `TestDialectsPage_PassThroughClaimsAreSpecified` fails if the page ever documents one the spec does not assert.
* Dialect Rewrites In Complex Expressions: `e2e/atago/dialect_windows.atago.yaml` adds 11 scenarios guarding the operator rewrites in the positions that are easy to get wrong, including windowed and filtered aggregates, named windows, `CASE`, subqueries, and `GROUP BY`.
* Dialect E2E: 58 atago scenarios in `e2e/atago/dialect_cross.atago.yaml`, `dialect_mysql.atago.yaml`, `dialect_postgresql.atago.yaml`, and `dialect_googlesql.atago.yaml` pin the semantics above from the CLI, each with a description of what SQLite would have done instead. `dialect.atago.yaml` adds 12 more covering chained `::` casts, `E''` and dollar-quoted strings, json operators, `~*`, `LATERAL` rejection, backtick names containing a space, `#` comments and raw strings, negative `DATE_DIFF`, `SELECT * EXCEPT` and `ARRAY<>` rejection, dialect name aliases, and a batch script stopped by a translate error.
* Known Limitations: `e2e/atago/known_bugs/` records dialect behavior sqly cannot reproduce as executable specs that are expected to fail, run by `scripts/run_known_bugs.sh` and kept out of CI.
* Non-Latin Table Names: `e2e/atago/unicode_table_names.atago.yaml` adds 9 scenarios covering a Japanese, Cyrillic, and accented-Latin file name queried by its own name, two Japanese-named files joined in one run (the collision that used to drop an input), the name reported by `--inspect` and `.tables`, write-back to a Japanese-named source, a non-Latin Excel sheet name, and the characters that must still be dropped or fall back.
* Write-Back Atomicity: `e2e/atago/writeback_atomicity.atago.yaml` adds 6 scenarios covering a failing second ACH set, a failing Fedwire set after a successful ACH one, the failing source staying importable, and the `--save-tables` equivalents, each asserting an empty `changes` delta so a stray or half-written file fails the scenario. Two success scenarios pin that the staging did not stop a real save from writing every target.
* Continuation Prompt: `pty.atago.yaml` adds two pty scenarios driving the real REPL, one asserting that an incomplete statement shows the continuation marker and then runs both lines as one statement, and one asserting that a dot-command never opens a continuation.
* Unfetchable URL Schemes: `http_import.atago.yaml` adds three scenarios covering `s3://`, `file://`, and a local file name containing a colon, and `TestUnfetchableURLScheme` pins the detector against Windows drive paths, uppercase schemes, and plain relative paths.
* Read-Only Write-Back: `save.atago.yaml` adds three scenarios pinning that a read-only `--save-tables`, a read-only `--save --force`, and a zero-row `UPDATE` each report "nothing to save" and leave the workdir untouched, asserted through `changes` so a stray file would fail.

### Documentation
* SQL Dialects Page: a new page documents `--dialect` as translation rather than emulation, splitting a query's three possible fates — translated, rejected by name, or passed through to SQLite where the answer can differ silently — and listing the known divergences with the command that shows each one. The cookbook and the README point at it.
* Documentation Site: `https://nao1215.github.io/sqly/` is rebuilt as a Hugo site under `website/`, replacing the MkDocs setup. It leads with runnable commands rather than prose: a front page you can paste from, a getting-started page, a cookbook of copyable one-liners indexed by task, and reference pages for the shell, the formats, and every flag. `doc/cookbook.md` is the source of the cookbook page, so it reads on GitHub too, and the developer docs move to `doc/architecture.md`, `doc/design_overview.md`, and `doc/build_and_test.md`.
* README: cut from 730 lines to 250. The reference material moved to the site; what stays is the pitch, a thirty-second example, a block of one-liner recipes, install, and the benchmark. The table-name rules were wrong about non-ASCII names and are corrected, and one-line notes record the contracts the fixes above establish: a run changing no row writes no file, only http and https URLs are downloaded, a multi-file save is all-or-nothing, and the shell prompt shows `...>` while buffering.
* Documentation Drift Guards: four tests keep the docs from outliving the implementation. `TestDocs_EveryDocumentedInvocationParses` runs all 94 `sqly ...` commands shown in the README, the cookbook, and every site page through the real argument parser, so a renamed flag or an invalid enum value cannot stay advertised by the ~80 commands no suite executes. `TestDocs_EveryDocumentedShellCommandExists` checks the shell page's dot-command tables against the commands the shell registers, in both directions. `TestCookbook_EverySectionIsExercised` requires every cookbook section to name the atago spec that runs it, so a new recipe cannot arrive without deciding where it is tested. `TestSite_InternalLinksResolve` resolves every site-internal link and image, which Hugo does not do. The tokenizer the first guard relies on has its own table tests, so a command it mis-splits (and would therefore check wrongly or skip) fails loudly.
* Cookbook E2E: `e2e/atago/cookbook.atago.yaml` runs 15 of the documented commands against the real binary, including the thirty-second example pinned to its exact output, so a copied command cannot silently stop working.

### Dependencies
* filesql v0.25.0: upgraded from v0.21.0 across four releases, for the non-Latin table-name fix (v0.22.0), the staged ACH/Fedwire write (v0.23.0), the staged table dump plus the compressed-dump close error (v0.24.0), and NULL-propagating `CONCAT` under MySQL and GoogleSQL (v0.25.0).
* prompt v0.0.12: upgraded from v0.0.11 for `WithContinuationPrefix`.

## [v0.29.0](https://github.com/nao1215/sqly/compare/v0.28.0...v0.29.0) (2026-07-29)

### New Features
* SQL Dialect Support: `--dialect` (and `.dialect` in the interactive shell) now let you write queries in MySQL, PostgreSQL, or GoogleSQL (BigQuery / Cloud Spanner) instead of SQLite; sqly translates them to SQLite before running. Loading files always uses SQLite, so only the queries you write are affected. Translation is best-effort compatibility: common incompatibilities (identifier quoting, `DATE_ADD`, `EXTRACT`, `::`/`SAFE_CAST` casts, `ILIKE`, `SPLIT_PART`, `SAFE_DIVIDE`, and more) are rewritten or backed by helper functions, constructs with no SQLite equivalent (for example `QUALIFY` or `DISTINCT ON`) fail with a clear error, and everything else is passed through. Requires filesql v0.19.0.

### Testing
* Dialect E2E: `dialect.atago.yaml` adds 20 atago scenarios covering each dialect's rewrite rules and helper functions, double-quoted-string handling, unknown-dialect and unsupported-construct errors, `.dialect` inside a shell session, "loading always stays SQLite", and complex queries (joins, aggregation with `HAVING`, CTEs, window functions, correlated subqueries, and nested translations).

### Documentation
* README: a new "Query in another SQL dialect: --dialect" section documents `--dialect` and `.dialect`, and the coverage treemap was regenerated.

## [v0.28.0](https://github.com/nao1215/sqly/compare/v0.27.4...v0.28.0) (2026-07-20)

### New Features
* Text Import Encoding Selection: `--encoding` now chooses how CSV, TSV, LTSV, JSON, and JSONL inputs without a Unicode BOM are decoded before parsing. `utf-8` remains the default, and sqly now accepts Shift-JIS (including common aliases such as `cp932`), EUC-JP, ISO-2022-JP, UTF-16LE, and UTF-16BE on demand.

### Bug Fixes
* Silent HTTP/HTTPS Imports: importing a supported file over HTTP or HTTPS no longer writes download status lines to stderr. Redirected or wrapped runs now stay free of `Downloading ...` / `Downloaded ...` noise even across redirects, while the remote file still imports through both CLI arguments and `.import`.
* Legacy Encoding With BOM Handling: explicit text decoding now composes with Unicode BOM detection instead of bypassing it. A BOM-prefixed text file is still decoded by its BOM, while a BOM-less legacy-encoded file can be decoded by `--encoding`.

### Testing
* HTTP Import E2E Silence: the atago HTTP-import specs now assert that successful remote imports stay quiet on stderr, covering both direct CLI input URLs and batch-mode `.import` of a remote URL.
* Text Encoding E2E: `encoding.atago.yaml` now covers an explicit Shift-JIS import through `--encoding`, alongside the existing UTF-8 BOM and UTF-16 fixtures.

### Documentation
* README / Demo Refresh: the HTTP import README example and VHS demo were refreshed for the silent-download behavior, and the import docs now briefly note `--encoding` and automatic BOM handling.

## [v0.27.4](https://github.com/nao1215/sqly/compare/v0.27.3...v0.27.4) (2026-07-19)

### Testing
* atago Snapshot Coverage: the binary E2E suite adds exact-output snapshot specs for the main non-interactive workflows that were previously pinned only by the in-process `main_test` goldens: CSV/TSV/LTSV output, Excel-to-CSV import, multiline `--sql-file`, and numeric sort order. This keeps the CLI contract covered through the real built binary while expanding atago's share of the output-regression surface.
* Golden Framework Removal: the large forked `golden` test package is removed. The remaining package-level fixture comparisons now use a tiny shared helper, and the real CLI/shell formatting snapshots stay in atago where diffs are exercised against the built binary instead of an in-process harness.

### Documentation
* Release Prep: refreshed the README shell snippet, benchmark caption, and shell docs to `v0.27.4`, removed editor/assistant-specific instruction files from the repository root, and updated the architecture doc to describe the new test layout (`e2e` + `testutil` instead of the old `golden` package).

### Dependencies
* filesql v0.18.0: upgraded from v0.17.2.
* atago v0.11.0: bumped the CI E2E and combined-coverage workflows from v0.8.0, and aligned the local install guidance and runner error message with that pin.

## [v0.27.3](https://github.com/nao1215/sqly/compare/v0.27.2...v0.27.3) (2026-07-08)

### Bug Fixes
* Interactive Input Lost on Windows: keystrokes typed right after a re-rendered prompt in the interactive shell are no longer dropped on a Windows ConPTY. The prompt library entered raw mode on os.Stdin while reading input through a different handle (CONIN$ on Windows), so on a ConPTY input delivered in the render-to-read window went to an ungoverned handle and could be lost, leaving a command typed but never executed and the session hung. prompt v0.0.11 routes raw mode through the read handle on Windows (while keeping the proven os.Stdin path on Unix), closing the gap.

### Testing
* Interactive-shell pty E2E on Windows: the atago pty specs that drive sqly's readline REPL now run on Windows ConPTY too. They were skipped there while the raw-mode handle mismatch made the ConPTY sessions drop input; prompt v0.0.11 fixes that, so the `skip: { os: windows }` guards are removed and the interactive path is verified end to end on every platform. Each command still submits with a trailing CR in the same send (`"...\r"`), the pty runs with a wide terminal (cols: 220) so the long CI sandbox path in the prompt cannot push a command onto the ConPTY last-column pending-wrap, and `make test-e2e` runs the pty specs in their own `--parallel 1` pass so they get uncontended CPU.

### Dependencies
* prompt v0.0.11: upgraded from v0.0.9 for the Windows raw-mode read-handle fix (github.com/nao1215/prompt/issues/13).
* atago v0.8.0: bumped the E2E runner from v0.7.0 in the e2e and coverage workflows.

## [v0.27.2](https://github.com/nao1215/sqly/compare/v0.27.1...v0.27.2) (2026-07-07)

### Bug Fixes
* Interactive Input Lost Between Lines: keystrokes typed right after a result, or a script piped into the interactive shell all at once, are no longer dropped. The shell re-acquired raw mode on every prompt, so input buffered while the terminal briefly returned to cooked mode between lines could be lost and the session would hang; automated interactive runs needed retries to absorb it. The shell now keeps the terminal in raw mode for the whole session (prompt v0.0.9 WithPersistentRawMode), which also disables the terminal's own newline translation, so command output is routed through a CRLF writer while the shell is interactive to keep result tables aligned.

### Testing
* Rapid Interactive Input E2E: the Go pty smoke suite adds a scenario that writes several queries plus the exit command to the interactive shell as a single burst and asserts every result appears and the session exits cleanly, so a regression that drops buffered input surfaces as a missing marker or a hang. It also asserts raw mode is entered exactly once per session.

### Dependencies
* prompt v0.0.9: upgraded from v0.0.8 for WithPersistentRawMode.

## [v0.27.1](https://github.com/nao1215/sqly/compare/v0.27.0...v0.27.1) (2026-07-06)

### Bug Fixes
* Unicode BOM on Import: a CSV, TSV, LTSV, JSON, or JSONL file that begins with a Unicode byte-order mark now imports correctly. A UTF-8 BOM (written by Excel, Notepad, and PowerShell) used to stay attached to the first column name, so `SELECT name` failed with `no such column`, and it broke JSON/JSONL parsing outright; a UTF-16 file surfaced as a column-count mismatch. The reader now strips a UTF-8 BOM and transcodes UTF-16 (LE or BE) to UTF-8 before parsing. A non-Unicode legacy encoding without a BOM (for example Shift-JIS) still passes through as raw bytes. Requires filesql v0.17.1.
* Zero-Padded Code Preservation: a numeric code with leading zeros (a ZIP code, product ID, or bank code such as `02134`) no longer imports as `2134`. Type inference classified a column as INTEGER when every value looked like an integer, dropping the leading zeros; a zero-padded integer literal is now typed as TEXT so the exact digits survive. A lone `0` stays an integer. Requires filesql v0.17.2.
* Excel Export Sheet Name: exporting a table whose name exceeds 31 characters or contains `: \ / ? * [ ]` to `.xlsx` no longer fails with a raw excelize error and no output file. Because a table name comes from the source filename, a long filename made every `--output file.xlsx` and `.dump` to Excel fail. The table name is now adapted to Excel's worksheet-name rules (forbidden characters replaced, length capped at 31) before the sheet is written.

### Testing
* Unicode BOM E2E: the atago suite adds `encoding.atago.yaml`, which imports base64-encoded UTF-8 BOM and UTF-16 LE fixtures across CSV, TSV, LTSV, JSON, and JSONL and asserts the first column is queryable by its plain name. The self-hosted atago runner in CI is pinned to v0.5.1 so the base64 fixtures are supported.
* Zero-Padded Code E2E: the atago suite adds `zero_padded_codes.atago.yaml`, which imports zip-code and mixed code columns and asserts the leading zeros survive, the column is typed as text, and a lone `0` stays an integer.
* Excel Sheet Name E2E: the atago suite adds `excel_sheet_name.atago.yaml`, which exports a table whose name exceeds 31 characters to `.xlsx` and asserts the export succeeds and the workbook re-imports.
* Join Semantics E2E: the atago suite adds `join.atago.yaml`, covering join shapes the earlier suites did not: a LEFT JOIN keeping an unmatched row as NULL, a CROSS JOIN's cartesian product, a self-join, and a three-way join with aggregation.
* Shell Navigate E2E: `helpers.atago.yaml` gains binary-level coverage for previously untested shell command paths: `.ls` listing a directory and failing on a missing one, `.cd` with no argument moving to home and failing on a missing directory, `.header` on a missing table, and `.tables` guiding an empty session toward `.import`.
* Compression Round-trip E2E: the atago suite adds `compression_roundtrip.atago.yaml`, which writes a CSV through each writable codec the earlier suites did not cover (xz, zstd, zlib, snappy, s2, lz4) and re-imports the file, so a successful round-trip confirms sqly compresses and decompresses each codec. gzip round-trip and bzip2 write rejection remain covered by `export_inference.atago.yaml`.
* Write-back Format E2E: the atago suite adds `writeback_formats.atago.yaml`, which updates a row and writes it back in place with `.save --force` for TSV, LTSV, and Parquet sources, then re-imports to confirm the change persisted and the untouched row survived. `save.atago.yaml` already covers CSV and the ACH/Fedwire native formats.
* Format-under-Compression E2E: the atago suite adds `format_compression.atago.yaml`, which round-trips TSV and LTSV through gzip and TSV through zstd, exercising each non-CSV format writer and its parser behind the decompression reader. JSON/JSONL/NDJSON under gzip stay covered by the existing suites, and CSV across every codec by `compression_roundtrip.atago.yaml`.
* Coverage Expansion: combined unit and self-hosted E2E statement coverage rose from about 83 percent to 94 percent. New tests exercise error-propagation paths across the shell, filesql, memory, and persistence layers (closed-database returns, usecase failures surfaced through the command handlers, malformed and missing inputs) plus edge cases in compare, inspect, profile, dump, schema, and the cd/ls commands. The octocov acceptable threshold moved from 80 percent to 90 percent, and cd.go and ls.go are no longer excluded from the coverage report now that they are directly tested.
* Snapshot and PTY E2E: the atago suite now pins sqly's formatted output (the table and markdown result tables, `.schema`, and `.tables`) to committed golden files, so an unintended formatting change fails with a diff. New pty scenarios drive the interactive shell in a real pseudo-terminal to verify the REPL path that batch input never reaches: a query prints its result table, and a `.mode` switch is reflected in the live prompt. The pty scenarios are POSIX-only and skip on Windows.

## [v0.27.0](https://github.com/nao1215/sqly/compare/v0.26.0...v0.27.0) (2026-07-04)

### New Features
* Import Mode for Ragged Rows: the new `--import-mode` flag and `.import-mode` shell command choose how a CSV/TSV row whose field count differs from the header is imported. `stop` (default) aborts the import and reports the mismatch, `skip` drops the ragged rows and imports the rest, and `fill` keeps every row by padding short rows with empty values and truncating long rows to the header width. The shell command changes the policy for later `.import` runs in the same session.

### Bug Fixes
* Malformed Row Data Loss: a CSV/TSV row whose field count differs from the header no longer imports as a silently empty table. filesql reported the mismatch as a field-count error that the loader masked with an empty single-column table, dropping both the malformed row and the well-formed rows before it. The default `stop` policy now fails the import with the column-count mismatch, and `--import-mode skip`/`fill` offer the non-fatal alternatives. Requires filesql v0.17.0.

## [v0.26.0](https://github.com/nao1215/sqly/compare/v0.25.0...v0.26.0) (2026-06-29)

### Performance
* Profile Memory Bound: `--profile` now streams each table's rows one at a time through the per-column accumulators instead of materializing the whole `SELECT *` result in Go first, so large CSV or Parquet inputs no longer hold a second full copy in memory. JSON and text output are unchanged; distinct counting still keeps a per-column distinct set, as the exact count requires.
* Keyed Compare Memory: `--compare --compare-key` now computes the key sets and the changed rows in SQLite (a NULL-safe keyed join finds the modified rows) and materializes only the added, removed, and modified rows, instead of loading both full tables into Go keyed maps. Unchanged rows never enter Go, so large keyed diffs stay memory-bounded by the size of the diff. JSON and text report output is unchanged.

### New Features
* Task-Oriented Help: interactive `.help` now groups commands by purpose (Session, Navigate, Inspect, Import / Export) and shows a minimal usage suffix for each (`.import PATH...`, `.dump TABLE FILE`, `.save DIR`). The destructive in-place overwrite `.save --force` is listed on its own labeled line, distinct from the non-destructive exports. No commands were added.
* SQL File Output: `--sql-file` can now export to `--output` when the script produces exactly one result set, so a saved SQL script works in the same automation pipelines as `--sql`. Setup statements may run first, the single result is written to the file with stdout left clean, and a script that yields no result set or more than one is rejected with a clear error.

### Bug Fixes
* Sheet Miss Diagnostics: `--sheet` failures are now diagnostic. A non-Excel input plus `--sheet` says the flag applies only to Excel inputs and how to recover, distinct from a sheet that is missing from an Excel input. A sheet miss names the workbook(s) that were checked (single and multi-workbook) and suggests re-importing without `--sheet` to list the available sheet names.
* Empty Session Save Guidance: saving an empty session now explains how to proceed instead of the bare `no tables to save`. An interactive `.save` is told to run `.import FILE` first, and a non-interactive `--save` run with no input files is told to pass input files. A read-only session with tables but no changes still reports that nothing changed and exits 0.
* Partial Import Startup Message: when a startup import partially succeeds, the interactive shell now reports the state (`sqly started with partial data: N of M inputs imported, K failed (listed above). The imported tables are ready to query.`) instead of the bare `one or more inputs failed to import`, which made a working session look broken. Non-interactive modes still exit non-zero.
* Import Error Detail: a fatal or partial import failure now carries the failing count and first failing path in the returned top-level error (for example `all 2 import(s) failed: path does not exist: a.csv (+1 more)`), so wrappers and logs that surface only the final error keep the context. The full per-input list still prints to stderr, and partial failures remain detectable via `errors.Is` against the partial-import sentinel.
* Batch Failure Location: a failing statement in batch stdin or `--sql-file` now reports its start position (`at line N`, or `at lines N-M` for a multiline statement) with a bounded one-line preview instead of the full body, so failures in long scripts are easy to locate. Fail-fast behavior and the non-zero exit status are unchanged.
* CLI Error Wording: an invalid command line (unknown flag, conflicting output modes, malformed flag value) is now printed as-is instead of under "failed to initialize sqly shell", which misleadingly implied the interactive shell failed. The shell-start prefix now applies only to genuine startup failures. Exit codes are unchanged.
* Parquet Export Null Fidelity: a SQL `NULL` exported to Parquet now reloads as `NULL` instead of an empty string, so `NULL` and `""` stay distinguishable in machine-readable output. The staging insert emits SQL `NULL` for null cells, and filesql v0.15.0 preserves the null through the Parquet write and reload.
* Parquet Export Text Fidelity: parquet export now stages every column as TEXT, so numeric-looking text the session holds (leading-zero codes like `007`, decimal strings like `1.00`) survives the round-trip verbatim instead of being coerced to a number by the staging column's affinity.
* Profile Blank Distinct Count: `--profile` now counts the blank string as a real distinct value, so `distinct_count` stays consistent with `blank_count` instead of dropping blanks and understating cardinality for categorical columns that mix blanks with real values.
* Profile Padded Null Placeholders: `--profile` now matches null-like placeholders such as `NULL` and `N/A` on the trimmed value, so a padded token like `" NULL "` raises both the null-placeholder warning and the whitespace warning instead of only the whitespace one.
* Consistent Numeric Contract: `--profile` and table-mode right alignment now share one numeric predicate, so a comma-formatted value like `"1,000"` is classified as numeric by both. Profiling previously reported `numeric_count = 0` for a column that table output right-aligned as numbers.
* Case-Insensitive Compare Key: `--compare-key` now resolves the key column case-insensitively, so `--compare-key ID` matches a column imported as `id`, following SQLite identifier semantics instead of requiring an exact case match.
* Case-Insensitive Compare Tables: `--compare-tables` now resolves table names case-insensitively, so `--compare-tables "USER,IDENTIFIER"` matches the tables imported as `user` and `identifier`. The report shows the canonical stored names.
* Case-Insensitive Schema Lookup: `.schema` now matches the stored object name case-insensitively, so `.schema V` returns the stored `CREATE VIEW v ...` (and a constrained table's real DDL) instead of falling back to a synthesized `CREATE TABLE`, following SQLite identifier semantics.
* Cache Artifacts Not Imported: when `--cache` points inside a directory that is also imported, sqly's own cache database and manifest sidecar are no longer treated as dataset inputs. The manifest is not imported as a stray table, and the second run is a warm cache hit instead of a cold re-import.
* Cache Signature Scope: the directory cache signature now includes only files sqly would import, so changing an unsupported sibling file such as a `.txt` note no longer invalidates the cache. Changing a supported input still invalidates it.

### Documentation
* Demo Asset Drift Check: a `TestDemoAssetsInSync` docs-sync test now guards the README demo GIFs without rendering them. It fails when a `doc/vhs/*.tape` declares an `Output` GIF that is missing (a tape changed or added without `make demo`) or when the README embeds a GIF no tape produces. Contributor docs explain when to rerun `make demo`, and the `--sql-file --output` workflow is documented as intentionally without a GIF.
* Install Methods: the README and installation docs now cover aqua (`aqua g -i nao1215/sqly`) and mise (`mise use aqua:nao1215/sqly`), which install sqly through the aqua standard registry.
* Helper Command Docs: the `.dump` and `.save` reference now matches current behavior. `.dump` in table mode infers the output format from the destination extension (TSV for `out.tsv`), falling back to CSV only for an unknown extension; `.save` documents native ACH/Fedwire whole-set write-back. A docs-sync test guards these descriptions.

### Dependencies
* `github.com/creack/pty`: added as a test dependency for PTY-backed end-to-end tests that drive the real interactive shell.
* `github.com/nao1215/filesql`: v0.14.0 to v0.16.0. v0.15.0 preserves SQL `NULL` through a Parquet round-trip. v0.16.0 makes the `*sql.DB` returned by `Open` safe to share across goroutines (a uniquely named shared-cache in-memory database instead of one reused connection) and makes `ReadOnlyDB` actually enforce read-only access on the `Query`/`QueryRow` and `Exec` paths.

## [v0.25.0](https://github.com/nao1215/sqly/compare/v0.24.0...v0.25.0) (2026-06-28)

### New Features
* Flag-Driven Subcommand Hint: `sqly help` and `sqly version` (and case variants) now print a short hint pointing at `--help`/`--version` and exit non-zero, instead of a confusing "path does not exist" import error. A real file or directory named `help`/`version` still imports. The help text and docs also note that sqly has no subcommands.
* Sheet-Name Completion: `.import WORKBOOK.xlsx --sheet` (and the joined `--sheet=` form) now tab-completes the workbook's sheet names. Names with spaces or non-ASCII characters are completed in a quoted or backslash-escaped form that stays a single argument, and sheet completion does not fall back to file-path suggestions.
* Path Completion for More Helpers: tab completion now completes filesystem paths for `.cd`, `.ls`, `.dump`, and `.save`, not only `.import`. `.cd` and `.save` offer directories only, `.ls` offers files and directories, and `.dump` completes the destination path after the table-name argument.

### Bug Fixes
* Graceful Terminal Startup: the interactive shell now creates its prompt session before printing the welcome banner, so when no usable terminal is available (for example `/dev/tty` cannot be opened in some PTY wrappers or headless containers) it fails with a clear "cannot start the interactive shell: no usable terminal" message that points at the non-interactive modes, instead of printing the banner and then crashing.
* Quiet Report-Only Runs: a successful directory import no longer prints its `Successfully imported ...` progress banner to stderr during `--inspect`, `--profile`, and `--compare`, so a clean report-only run is quiet on stderr and the structured report is its only output. Import warnings and errors still print.
* Fail Fast on Missing Helper Arguments: `.schema`, `.header`, `.describe`, `.dump`, `.import`, `.mode`, and `.save` now return an error when a required argument is missing, instead of printing usage and exiting 0. A batch script stops on the failure and exits non-zero, so it no longer silently skips an intended helper command. The usage text is attached to the error, so an interactive user still sees it on stderr.
* Stable --stdin Error: a failing `--stdin` import now reports the input as `stdin (--stdin FORMAT)` instead of leaking the random internal staging temp path (for example `/tmp/sqly-stdin-*/stdin.csv`), so the message reads as a stdin problem and stays the same across runs.
* No Silent No-Op: a non-interactive run with no TTY and no statements (empty or comment-only stdin, and no `--sql`/`--sql-file`) now prints a hint and exits non-zero instead of exiting 0 silently, so headless wrappers and CI no longer mistake a no-op invocation for a completed query. An empty `--save`/`--save-tables` batch still leaves source files untouched.
* Reject Empty --sql: an explicit empty `--sql ""` now fails fast with a clear validation error instead of silently running no query and exiting 0, matching the other string flags that already reject explicit empty values.
* Leaner Keyed Compare: `--compare --compare-key` now converts each side to its keyed rows and releases the raw table before loading the other, so the two full tables are no longer held in memory at the same time.
* Single-Pass Profiling: `--profile` now aggregates each column's statistics in a single pass over the rows instead of copying the whole table into a per-column values and nulls slice, so its memory no longer scales with columns times rows.
* Single-Insert History Writes: each interactive command's history write is now a single insert that relies on SQLite AUTOINCREMENT, instead of scanning the entire history table to compute the next id. History preloading at startup still reads the table once.
* Cached Completion Metadata: interactive SQL completion now caches table and column suggestions keyed on the current table-name set, so it no longer queries every table's header on each keystroke. A line still typing a dot-command skips schema lookups entirely, and the cache refreshes when the table set changes or after an import.
* No Stray Config Directory: `NewConfig` no longer creates the default XDG config directory when `SQLY_HISTORY_DB_PATH` is set, so routing history elsewhere has no unnecessary filesystem side effect.
* Clear-Screen Control Key: the interactive shell now binds the documented Ctrl+L key to clear the screen, redrawing the prompt with the current input preserved.
* History Control Keys: the interactive shell now binds the documented Ctrl+P and Ctrl+N keys to previous/next command history, matching the arrow keys.
* Cursor Movement Control Keys: the interactive shell now binds the documented Ctrl+F and Ctrl+B keys to move the cursor forward/backward one character, matching the arrow keys.
* Cursor-Aware Completion: tab completion now uses the text before the cursor instead of the whole line, so moving the cursor back into an earlier path, table name, or SQL identifier and pressing TAB completes the token under the cursor rather than the line ending.
* Home-Path Import Completion: `.import` tab completion now expands a leading `~/` to the home directory for the lookup while keeping the suggestion rendered as `~/file.csv`, so home-directory paths complete the same way relative and absolute paths do. The accepted `~/...` argument is expanded again at import time.
* Directory Import Completion: `.import` tab completion offers directory candidates with a trailing slash, so a directory import target (for example `datadir/`) is discoverable and can be accepted and imported directly, not just descended into. Regression tests lock the directory-candidate behavior in.
* Hidden Path Import Completion: `.import` tab completion stays hidden-by-default but now traverses a hidden directory once its prefix is typed explicitly, so `.import .secret/` lists the importable files inside it. Regression tests lock this in for both hidden files and nested hidden directories.
* Quoted Import Completion: `.import` tab completion now works while typing a quoted path. After an opening quote (for example `.import "my`), completion matches the path fragment inside the quote and keeps it quoted, closing the quote on a file and leaving it open on a directory so the accepted command stays a single argument.
* Home-Directory Expansion in Helpers: `.cd`, `.ls`, `.import`, `.dump`, and `.save` now expand a leading `~` (and `~/...`) to the user's home directory before running, so `.cd ~` and `.import ~/data.csv` work instead of failing on a literal `~`. Forms like `~user` or a `~` later in the path are left untouched.
* Boundary-Safe Home Abbreviation: the prompt now replaces the home directory with `~` only when the working directory equals home or is a real descendant at a path-separator boundary. A sibling such as `/home/nao2` (or `C:\Users\nao-backup` on Windows) that merely shares a byte prefix with `/home/nao` is no longer rewritten into a misleading `~2`.
* Strict Helper Argument Validation: `.pwd`, `.clear`, and `.exit` now reject unexpected trailing arguments with a clear error instead of ignoring them, matching the other helper commands. A typo such as `.exit now` no longer silently terminates a batch run with status 0.
* Batch-Safe Clear: `.clear` now emits its ANSI clear-screen escapes only in an interactive TTY session. In batch mode (piped stdin) it is a no-op, so machine-readable stdout such as `--json`, `--ndjson`, and `--csv` is no longer corrupted by control sequences.
* Multi-line Interactive SQL: the shell now buffers a SQL statement across lines and submits on Enter only when it ends with `;`, so a typed or pasted multi-line statement (for example `SELECT ... UNION ALL SELECT ...;`) runs once instead of executing each line separately. Dot-commands stay single-line, and pressing Enter on a blank line force-runs a query typed without `;`.
* Idempotent SQLite Driver Registration: `config.InitSQLite3()` now guards driver registration with a package-level `sync.Once` instead of a function-local one, so calling it more than once no longer panics with `sql: Register called twice for driver sqlite3`.
* Prefix-Scoped Import Completion: `.import` tab completion now reads only the directory named by the typed path prefix instead of walking the whole working tree on every keystroke. Directories are offered with a trailing slash so the path can be completed one level at a time, keeping latency proportional to the targeted subtree rather than repository size.
* Space-Safe Import Completion: `.import` tab completion now backslash-escapes spaces and shell-special characters, so accepting a path like `my data.csv` inserts `my\ data.csv` and reaches `.import` as a single argument. Escaping (not quoting) is used so the suggestion still prefix-matches the typed word.
* Completion Into Space-Containing Directories: `.import` tab completion now descends into a directory whose name contains a space. The escaped prefix (for example `my\ dir/`) is decoded to read the real directory while the escaped form is kept on each suggestion, so nested files complete and still round-trip through the command parser.
* Windows-Style Path Completion: `.import` tab completion now keeps backslash-separated path prefixes completable. Suggestions normalize separators to `/`, so a prefix like `C:\dir\fi` is matched against the slash-normalized suggestions instead of being filtered out. Absolute and `./`/`../` paths still complete from the typed directory.
* Compare Input Order: `--compare` without `--compare-tables` now keeps the left/right direction in the order the inputs were given on the command line, instead of sorting the table names alphabetically.
* Typed JSON Mode Shell UX: switching to `.mode json-typed`/`.mode ndjson-typed` now shows the typed mode name in the prompt label and the `.mode` current-mode banner instead of the plain `json`/`ndjson`, and `.mode` lists both typed variants.
* Content-Aware Import Cache Key: `--cache` now keys invalidation on each input file's path, size, and a SHA-256 content hash instead of path, size, and modification time. A source rewritten in place with different but same-length content and its original mtime restored is now detected and the cache rebuilt, so a warm run can no longer return stale rows for a modified file.
* Clean Ctrl-D Exit: pressing Ctrl-D (EOF) in the interactive shell now exits cleanly like `.exit` instead of printing a raw `EOF` line. Both EOF spellings the prompt library reports (Ctrl-D on an empty line and a closed input stream) are treated as a normal termination.
* Symlink-Resolved System-Path Guard: import path validation now rejects a symlink whose canonical target is a blocked system location (such as a link to `/etc/hosts`), not only a directly typed system path. It also normalizes the macOS `/private` prefix, while standard Unix pseudo-files (`/dev/stdin`, `/proc/self/fd/*`) keep importing.

### Documentation
* Docs-Sync Guardrail: a new test asserts that every `make <target>` command shown in the contributor docs (`README.md`, `CONTRIBUTING.md`, `doc/pages/markdown/build_and_test.md`) is a real Makefile target, so a stale setup instruction is caught in CI. It also fixes the stale `make install tools` command in `build_and_test.md`, which is now `make tools`.
* Pull-Request Template: add `.github/pull_request_template.md` with a short checklist (tests, lint, docs, CHANGELOG, cross-platform impact) so the project's change bar is reinforced when a PR is opened.
* Hermetic E2E Harness: the ShellSpec suite now runs through `scripts/run_e2e.sh`, which builds the binary and runs the suite in a throwaway temp-backed HOME and config sandbox, so it never reads or writes the developer's real config directory and local and CI runs are identical. `make test-e2e` and the CI job invoke the suite only through this wrapper.
* Windows Binary Smoke Coverage: a new pure-Go smoke harness (`e2e/smoke_test.go`, run with `make smoke` or the `smoke` build tag) builds the real binary and drives helper commands, output formats, stdout/stderr separation, and the startup hint through stdin and flags. A CI job runs it on Windows, macOS, and Linux, giving Windows the binary-level coverage the shell-based suite cannot.
* Release Artifact Smoke Coverage: a new CI workflow builds GoReleaser artifacts in snapshot mode (no publishing, signing, or SBOM) on every PR and push, then `scripts/smoke_artifacts.sh` checks that the expected archives and OS packages exist, the host archive extracts, and the extracted binary runs, so packaging regressions are caught before a release tag is cut.

### Dependencies
* Prompt: upgrade `github.com/nao1215/prompt` to v0.0.8 for the `ActionClearScreen` key action that backs the Ctrl+L clear-screen binding.
* Prompt: upgrade `github.com/nao1215/prompt` to v0.0.7 for the `WithIsComplete` multiline submit predicate and the `WithWordEscape` option that lets completion treat backslash-escaped whitespace as part of a word.

## [v0.24.0](https://github.com/nao1215/sqly/compare/v0.23.0...v0.24.0) (2026-06-06)

### Features
* Opt-In Import Cache: `--cache PATH` snapshots the imported tables to a standalone SQLite file so a repeated run against unchanged inputs reloads from it instead of re-parsing large source files. The cache key is each input file's path, size, and modification time (expanded recursively for directories), so it invalidates automatically when a source changes. `--cache-clear` forces a cold rebuild, and a cache that is unavailable or unwritable falls back cleanly to a cold import with a warning instead of failing the query. Caching is skipped for `--stdin` datasets and for ACH/Fedwire inputs (whose write-back needs the live import registry).
* CLI-First Profile Workflow: a top-level `--profile` mode prints a machine-readable data-quality report for every imported table, so users who received unfamiliar data can understand it before writing SQL. Each report covers per-table row and column counts and, per column, null and blank counts, distinct and numeric counts, and safe warnings for mixed numeric/non-numeric values, null-like placeholder text (`NULL`, `N/A`, ...), and leading or trailing whitespace. JSON is the default automation contract; `--profile-format text` prints a human-readable summary. It works for files, directories, stdin datasets, and multi-table imports.
* CLI-First Compare Workflow: a top-level `--compare` mode diffs two imported tables without entering the interactive shell. It reports schema differences (columns unique to each side and type changes), a row-count delta, and—when `--compare-key COL` is given—keyed row differences (added, removed, and modified rows). JSON is the default automation contract; `--compare-format text` prints a human-readable summary. The two tables are the pair of imported tables, or an explicit `--compare-tables "left,right"`. Clear errors are returned for a missing key column, a non-unique key, a missing named table, or an ambiguous import that did not produce exactly two tables.
* Native ACH and Fedwire Write-Back: `--save`/`--save-tables` (and interactive `.save`) now reconstruct a complete `.ach`/`.fed` file from its imported table set after in-session `UPDATE`s, using filesql's native ACH/Fedwire writers. The whole related table set is rewritten together into one valid file, and write-back validates that the required companion tables (for ACH, the file-header, batches, and entries tables) are present, failing with an explicit error when the set is incomplete. The single-table `--output`/`.dump` path still rejects `.ach`/`.fed`, since those formats require a coordinated record set. Adding or removing records is not supported by the native reconstruction; only updates to existing rows are persisted.
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
* Write-Back Skips Unchanged Imported Tables: `.save`, `.save DIR`, and the non-interactive `--save`/`--save-tables` runs now persist only a file-backed imported table whose content changed. A session that touched only a TEMP or SQL-created scratch table, or that made net-zero edits that cancel out, no longer rewrites an untouched source, fails on an unwritable JSONL import, or aborts on a scratch table that has no source file. Each import records a content fingerprint, and write-back compares against it instead of relying on a coarse session-wide changed flag.

### Documentation
* Add README demos for cross-format JOIN (Parquet and CSV), --output format conversion (JSON, Parquet, Excel), and directory import across formats, recorded with VHS.

## [v0.22.0](https://github.com/nao1215/sqly/compare/v0.21.0...v0.22.0) (2026-06-01)

### Breaking Changes
* Direct --sql Runs One Statement: direct `--sql` (and `--sql --output`) now rejects multi-statement input instead of silently running every statement and keeping only the last result set.
* Save Mode Rejects PRAGMA: a non-interactive `--save`/`--save-tables` run now rejects a setter, command, or rowset PRAGMA, since a PRAGMA side effect lives only in the in-memory session and has no file write-back representation.
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
* Write-Back Rejects Schema-Only Runs: A non-interactive `--save`/`--save-tables` run now fails up front when the SQL changes schema or runs a maintenance statement (ALTER, DROP, REINDEX, ANALYZE, CREATE/DROP of a table/view/index/trigger, including `CREATE TABLE AS SELECT`), since write-back can only persist `INSERT`/`UPDATE`/`DELETE` on imported tables. Previously such a run exited 0 and reported success while leaving the source unchanged.

### Bug Fixes
* Neutral Result Message For Non-DML: A DDL, PRAGMA, or maintenance statement now reports `statement executed successfully` instead of a misleading `affected is N row(s)` count.
* PRAGMA On The Exec Path: A setter PRAGMA (`PRAGMA user_version = 1`) and a no-row command PRAGMA (`PRAGMA incremental_vacuum`) now run successfully instead of failing with a "no records" error.
* Batch .import Under Save Flags: A batch or `--sql-file` script that imports its own input with `.import` and then modifies it is now allowed under `--save`/`--save-tables`; write-back is validated after the import runs.
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
* Inspect And Dependent-Flag Validation: `--inspect` rejects a conflicting output mode flag such as `--csv` or `--parquet`. `--stdin-name` requires `--stdin`, `--inspect-sample` requires `--inspect`, and `--force` requires `--save`/`--save-tables`, instead of being silently ignored. A `--stdin-name` that is a SQLite keyword is rejected since it is not queryable as a bare table name, and an imported file whose name sanitizes to a keyword now warns that the table must be quoted.
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
* Explicit Empty Flag Values Rejected: `--output`, `--sql-file`, `--save-tables`, and `--stdin` now reject an explicit empty value instead of treating the flag as absent. `.import` likewise rejects an empty `--sheet`, in both the `--sheet ""` and `--sheet=` forms.
* Comment-Only SQL Files Rejected: A `--sql-file` that contains only comments now fails like an empty file, since it has no executable SQL.
* Conflicting Output Mode Flags Rejected: Passing more than one output mode flag (for example `--csv --json`) now fails instead of applying an undocumented precedence.
* Output For Non-Rowset DML: `--output` is now rejected for a DML statement that produces no rows (an `INSERT`/`UPDATE`/`DELETE` without `RETURNING`), instead of being silently ignored.
* Save Flags With sql-file On A Terminal: `--save` and `--save-tables` now work with `--sql-file` even when stdin is a terminal.
* Stdin Routing: `--sql-file` now rejects non-empty piped stdin instead of silently dropping it, pointing at `--stdin` for dataset input. A `--stdin` dataset run with no query now fails instead of importing and discarding the data.
* UTF-8 BOM In Scripts: A leading UTF-8 BOM is now stripped from `--sql-file` scripts and batch stdin, so BOM-prefixed files from Windows editors and export tools parse like plain UTF-8.
* Sheet Flag On Unreadable Directories: `--sheet` validation now surfaces the real directory access error instead of misclassifying an unreadable directory as a non-Excel input.
* Multi-Workbook Sheet Filter: In a multi-workbook or directory import, a workbook that lacks the requested `--sheet` is now skipped instead of failing the whole import, so matching workbooks still load. The run fails only when no workbook contains the sheet.
* Directory Import Provenance: Directory imports now record each table's source file even when the basename is sanitized or the file yields several tables (Excel, ACH, Fedwire), so `--inspect` reports the file rather than the directory path.
* Directory Import Collisions: Two files in a directory tree that map to the same table name (duplicate basenames from different subdirectories, or sanitized-name collisions) are now rejected instead of one silently overwriting the other.
* Directory Re-Import: Re-importing a directory that overwrites an existing table is now reported as a successful overwrite instead of `No supported files found`, and the table's source is re-pointed to the directory file so `.save --force` can no longer write the directory rows back into the original source file.
* Write-Back Safety: `--save-tables` now rejects a destination that resolves to the source file or already exists in the destination directory, and validates all targets before writing any, so a failure leaves no partial output. `--output` now rejects a destination that aliases an imported source file. A read-only query no longer triggers write-back under `--save`/`--save-tables`, and a run that fails during write-back no longer prints a DML success count to stdout.

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
* Stdin Dataset Source And Name Safety: `--inspect` now reports a stable `stdin` source for a piped `--stdin` dataset instead of leaking the ephemeral staging temp path. `--save`/`--save-tables` reject a stdin-backed table up front instead of failing late while trying to write to a deleted temp file. `--stdin-name` is validated and rejects empty or path-like values, so it can no longer stage files outside the temp directory.
* Import Failure Handling: When an explicitly requested input fails to import, non-interactive runs now exit non-zero instead of continuing on the partially imported subset. This covers query mode (`--sql`/`--sql-file`), `--inspect`, and the batch `.import` command (which also stops later commands). Import diagnostics now always go to stderr, so stdout stays reserved for query results and the `--inspect` JSON report. The interactive shell still starts after a partial import, with a warning, since the loaded tables remain usable.
* Batch Fail-Fast: Batch mode (piped stdin and `--sql-file`) now stops at the first failed statement or helper command instead of continuing. Later statements no longer run, so their output cannot leak into a pipeline the process then reports as failed, and side-effecting commands such as `.save` and `.dump` placed after a failure no longer execute. The run still exits non-zero.
* Empty Batch No Write-Back: An empty batch (for example empty piped stdin) no longer triggers `--save`/`--save-tables` write-back. With nothing executed, source files are left untouched and the run is a no-op.
* Sheet Flag Validation For Directories And Empty Values: `--sheet` is now rejected when a directory input contains no Excel files, and when it is given an explicit empty value (`--sheet ""`). Both previously slipped past validation and were silently ignored. This applies to the CLI flag and the `.import` command.
* Batch Identifier Quoting: Batch statement splitting now recognizes SQLite bracket-quoted (`[ ... ]`) and backtick-quoted (`` `...` ``) identifiers, so a semicolon inside them no longer splits a statement. This matches the existing handling of single-quoted strings, double-quoted identifiers, and comments.
* File-Output Status On Stderr: Status lines for file-output operations (`--output`, `.dump`, and `.save`/`--save`/`--save-tables`) now go to stderr instead of stdout. When all data is written to files, stdout stays empty, matching `--inspect` and letting scripts rely on an empty stdout for success.
* Mode-Change Banner On Stderr: The `.mode` change banner now goes to stderr instead of stdout. In batch mode, switching to `.mode json` or `.mode ndjson` no longer prints a human-readable banner ahead of the machine-readable payload, so stdout stays parseable.
* Directory Output Targets: `--output` and `.dump` now reject a destination that already exists as a directory with a clear error, instead of silently writing to a sibling file such as `dir.csv`.
* Output Path Preservation: `--output` and `.dump` no longer rewrite a destination with an unknown extension to a sibling `.csv` file. The CSV fallback now writes to the exact path given (for example `--output out.unknown` writes to `out.unknown`), instead of silently creating `out.csv`.
* Inspect Flag Conflicts: `--inspect` now rejects conflicting action and side-effecting flags (`--sql`, `--sql-file`, `--output`, `--save`, `--save-tables`) with a clear error instead of silently discarding them.
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
* Write-Back: Persist DML changes to files with explicit, opt-in flags and the `.save` command, so edits no longer vanish with the in-memory session. `--save-tables DIR` writes each table into DIR, preserving each source's format and compression and leaving the originals untouched. `--save` overwrites the source files in place and requires `--force`. In the interactive shell, `.save DIR` and `.save --force` do the same. Only single-source csv/tsv/ltsv/parquet tables are written; tables from a directory import, multi-table sources (Excel, ACH, Fedwire), and SQL-created tables are rejected with a clear error before anything is written. The save flags apply after `--sql` and batch runs; without them a session stays in-memory only.
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
