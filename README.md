<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-4-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  

# sqly

sqly runs SQL against CSV, TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH, and Fedwire files. It imports them into an [SQLite3](https://www.sqlite.org/index.html) in-memory database, so joins, CTEs, and aggregates all work. Compressed files (`.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, `.lz4`) are read transparently.

Run a query directly, or open the interactive shell with completion and history.

![demo](./doc/img/demo.gif)

```shell
sqly --sql "SELECT * FROM data" data.csv.gz
sqly --sql "SELECT user_name, position FROM user JOIN identifier ON user.identifier = identifier.id" testdata/user.csv testdata/identifier.csv
```

## Install

```shell
go install github.com/nao1215/sqly@latest
```

```shell
brew install nao1215/tap/sqly
```

Runs on Windows, macOS, and Linux. Requires Go 1.25 or later when building from source.

## Run SQL: --sql

Pass file or directory paths as arguments; sqly imports each one and names the table after the file (so `user.csv` becomes table `user`).

```shell
$ sqly --sql "SELECT * FROM user" testdata/user.csv
+-----------+------------+------------+-----------+
| user_name | identifier | first_name | last_name |
+-----------+------------+------------+-----------+
| booker12  |          1 | Rachel     | Booker    |
| jenkins46 |          2 | Mary       | Jenkins   |
| smith79   |          3 | Jamie      | Smith     |
+-----------+------------+------------+-----------+
```

## Interactive shell

Run `sqly` without `--sql` to open the shell. It behaves like `sqlite3` or `mysql`: type SQL, or a helper command that begins with a dot. Tab completes keywords and table names, and history is kept across sessions.

![shell demo](./doc/img/shell-demo.gif)

```shell
$ sqly testdata/user.csv
sqly v0.19.0

enter "SQL query" or "sqly command that begins with a dot".
.help print usage, .exit exit sqly.

sqly:~/sqly(table)$ .help
        .cd: change directory
     .clear: clear terminal screen
  .describe: print column information of a table
      .dump: dump db table to file in a format according to output mode (default: csv)
      .exit: exit sqly
    .header: print table header
      .help: print help message
    .import: import file(s) and/or directory(ies)
        .ls: print directory contents
      .mode: change output mode
       .pwd: print current working directory
      .save: write tables back to files: .save DIR (to a directory) or .save --force (overwrite sources)
    .schema: print CREATE TABLE statement of a table
    .tables: print tables
```

History is stored in a SQLite database under the config directory. If that location is not writable, sqly disables history for the session with a warning and keeps running. Set `SQLY_HISTORY_DB_PATH` to choose another path.

## Output formats

The default is an ASCII table. Switch with a flag (`--csv`, `--tsv`, `--ltsv`, `--json`, `--ndjson`, `--markdown`), or in the shell with `.mode <name>`. Values are emitted as strings.

![formats demo](./doc/img/formats-demo.gif)

```shell
$ sqly --csv --sql "SELECT user_name, identifier FROM user LIMIT 2" testdata/user.csv
user_name,identifier
booker12,1
jenkins46,2

$ sqly --json --sql "SELECT user_name, identifier FROM user LIMIT 2" testdata/user.csv
[
  {"user_name":"booker12","identifier":"1"},
  {"user_name":"jenkins46","identifier":"2"}
]

$ sqly --ndjson --sql "SELECT user_name, identifier FROM user LIMIT 2" testdata/user.csv
{"user_name":"booker12","identifier":"1"}
{"user_name":"jenkins46","identifier":"2"}
```

Excel (`--excel`) and Parquet (`--parquet`) are export-only: they render as CSV on screen and write a real `.xlsx`/`.parquet` file through `--output` or `.dump`. Parquet needs at least one row to infer its schema.

```shell
$ sqly --parquet --output result.parquet --sql "SELECT * FROM user" testdata/user.csv
Output sql result to result.parquet (output mode=parquet)
```

## Write results to a file: --output

Redirection works on Unix; `--output` works everywhere and may appear before or after the file arguments.

```shell
$ sqly --csv --sql "SELECT * FROM user" testdata/user.csv > out.csv
$ sqly --sql "SELECT * FROM user" --output out.csv testdata/user.csv
$ sqly --sql "SELECT * FROM user" testdata/user.csv --output out.csv
```

The format and compression are inferred from the `--output` extension when no mode flag is given (the same applies to `.dump`). Text and JSON formats accept `.gz`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, and `.lz4`. A mode flag that disagrees with the extension is rejected, as are `.bz2` and compression on Parquet or Excel.

```shell
$ sqly --sql "SELECT * FROM user" --output result.ndjson.gz testdata/user.csv
```

## Pipe data in: --stdin

Piped stdin is read as commands by default (see batch mode below). Use `--stdin <format>` to treat it as a dataset instead; the format is explicit (`csv`, `tsv`, `ltsv`, `json`, `jsonl`) because a pipe has no filename. The table is `stdin` unless you set `--stdin-name`. Piped data joins file arguments.

![stdin demo](./doc/img/stdin-demo.gif)

```shell
$ cat testdata/user.csv | sqly --stdin csv --sql "SELECT user_name FROM stdin LIMIT 1"
+-----------+
| user_name |
+-----------+
| booker12  |
+-----------+

$ cat testdata/user.csv | sqly --stdin csv --sql "SELECT s.user_name, i.position FROM stdin s JOIN identifier i ON s.identifier = i.id" testdata/identifier.csv
```

## Batch mode

When stdin is not a terminal and `--stdin` is not given, sqly reads SQL and dot commands from it instead of starting the shell. A SQL statement ends at a top-level `;` and may span lines; dot commands are single-line. A failed statement exits non-zero, so batch runs are scriptable.

```shell
$ printf '.tables\nSELECT COUNT(*) FROM user\n' | sqly testdata/user.csv
+------------+
| TABLE NAME |
+------------+
| user       |
+------------+
+----------+
| COUNT(*) |
+----------+
|        3 |
+----------+
```

## Load SQL from a file: --sql-file

`--sql-file PATH` runs SQL read from a file (multiple `;`-separated statements allowed). It cannot be combined with `--sql`. Because the query comes from a file, stdin stays free for a dataset:

```shell
$ cat testdata/user.csv | sqly --stdin csv --sql-file join.sql testdata/identifier.csv
```

where `join.sql` holds:

```sql
SELECT s.user_name, i.position
FROM stdin s
JOIN identifier i ON s.identifier = i.id
ORDER BY s.identifier;
```

## Inspect tables: --inspect

`--inspect` imports the inputs, prints a JSON report of every table (name, source, columns, row count, sample rows), and exits without the shell. It is the non-interactive equivalent of `.tables` + `.schema` + `.describe`, useful for scripts and LLMs. Import progress goes to stderr, so stdout is JSON only. `--inspect-sample N` sets the sample size (default 5; `0` for schema only).

```shell
$ sqly --inspect --inspect-sample 1 testdata/identifier.csv
{
  "tables": [
    {
      "name": "identifier",
      "source": "testdata/identifier.csv",
      "row_count": 3,
      "columns": [
        {"name": "id", "type": "INTEGER", "nullable": true, "primary_key": false},
        {"name": "position", "type": "TEXT", "nullable": true, "primary_key": false}
      ],
      "sample_rows": [
        {"id": "1", "position": "developrt"}
      ]
    }
  ]
}
```

## Inspect schema: .schema and .describe

```shell
sqly:~/data(table)$ .schema user
CREATE TABLE "user" ("user_name" TEXT, "identifier" INTEGER, "first_name" TEXT, "last_name" TEXT)

sqly:~/data(table)$ .describe user
+-----+------------+---------+---------+------------+----+
| cid |    name    |  type   | notnull | dflt_value | pk |
+-----+------------+---------+---------+------------+----+
|   0 | user_name  | TEXT    |       0 |            |  0 |
|   1 | identifier | INTEGER |       0 |            |  0 |
|   2 | first_name | TEXT    |       0 |            |  0 |
|   3 | last_name  | TEXT    |       0 |            |  0 |
+-----+------------+---------+---------+------------+----+
```

## Write changes back: --save and --save-dir

A session is in-memory only: `UPDATE`/`INSERT`/`DELETE` change the loaded tables but never touch files unless you opt in. `--save-dir DIR` writes each table into DIR (preserving format, compression, and name) and leaves originals untouched. `--save` overwrites the source files in place and requires `--force`. In the shell, `.save DIR` and `.save --force` do the same.

![save demo](./doc/img/save-demo.gif)

```shell
$ sqly --sql "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1" --save-dir ./out testdata/user.csv
$ sqly --sql "DELETE FROM user WHERE identifier > 100" --save --force testdata/user.csv
```

Only tables mapping one-to-one to a single CSV, TSV, LTSV, or Parquet source can be written. Tables created by SQL, directory imports, and multi-table sources (Excel, ACH, Fedwire) are rejected with a clear error before anything is written.

## Directory import

A directory argument imports every supported file under it recursively, and you can mix files and directories.

```shell
$ sqly ./data_directory
$ sqly file1.csv ./data_directory file2.tsv --sql "SELECT * FROM users"
```

The shell `.import` command does the same:

```shell
sqly:~/data$ .import ./csv_files
Successfully imported 3 tables from directory ./csv_files: [users products orders]
sqly:~/data$ .import --sheet "Q1 Sales" report.xlsx
```

## ACH files

ACH (`.ach`) files load as several tables for easy querying:

```shell
$ printf '.tables\n' | sqly testdata/ppd-debit.ach
+-----------------------+
|      TABLE NAME       |
+-----------------------+
| ppd_debit_file_header |
| ppd_debit_batches     |
| ppd_debit_entries     |
+-----------------------+

$ sqly --sql "SELECT amount FROM ppd_debit_entries WHERE amount > 10000" testdata/ppd-debit.ach
```

`{filename}_entries` holds the entry detail records. Addenda become `{filename}_addenda`; IAT files add `_iat_batches`, `_iat_entries`, and `_iat_addenda`.

## Fedwire files

Fedwire (`.fed`) files load as a single `{filename}_message` table with all FEDWireMessage fields.

```shell
$ sqly --sql "SELECT * FROM customer_transfer_message" testdata/customer-transfer.fed
```

## Excel sheets

Each Excel sheet becomes a table named `filename_sheetname`. Pick one with `--sheet` using its original name.

```shell
$ sqly data.xlsx --sheet "A test"
```

## Table name rules

Spaces, hyphens, and dots become `_`; other special characters are removed; a name starting with a digit gets a `sheet_` prefix. Excel sheet names are sanitized the same way, with non-ASCII characters removed.

| Input | Table |
|:--|:--|
| `bug-syntax-error.csv` | `bug_syntax_error` |
| `2023-data.csv` | `sheet_2023_data` |
| `data@v2.csv` | `datav2` |
| `data.xlsx` sheet `Café` | `data_Caf` |

## Supported file formats

| Format | Extensions | Notes |
|:--|:--|:--|
| CSV | `.csv` | |
| TSV | `.tsv` | |
| LTSV | `.ltsv` | |
| JSON | `.json` | Stored in a `data` column; query with `json_extract()` |
| JSONL | `.jsonl` | Stored in a `data` column; query with `json_extract()` |
| Parquet | `.parquet` | |
| Excel | `.xlsx` | Each sheet becomes a table |
| ACH | `.ach` | Creates several tables (`_file_header`, `_batches`, `_entries`, `_addenda`) |
| Fedwire | `.fed` | Creates a single `_message` table |

CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel also read these compression extensions: `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, `.lz4` (e.g. `.csv.gz`, `.tsv.bz2`).

## Key bindings for the shell

|Key|Action|
|:--|:--|
|Ctrl + A / Ctrl + E|Beginning / end of line|
|Ctrl + P / Ctrl + N|Previous / next command|
|Ctrl + F / Ctrl + B|Forward / backward one character|
|Ctrl + D|Delete character under cursor|
|Ctrl + H|Delete character before cursor|
|Ctrl + W|Cut word before cursor|
|Ctrl + K / Ctrl + U|Cut line after / before cursor|
|Ctrl + L|Clear screen|
|TAB|Completion|
|↑ / ↓|Previous / next command|

## Benchmark

CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics. Query:

```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|Records  | Columns | Time per Operation | Memory Allocated per Operation | Allocations per Operation |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |

## Alternative tools

|Name| Description|
|:--|:--|
|[nao1215/sqluv](https://github.com/nao1215/sqluv)|Simple terminal UI for DBMS and local CSV/TSV/LTSV|
|[harelba/q](https://github.com/harelba/q)|Run SQL directly on delimited files and multi-file sqlite databases|
|[dinedal/textql](https://github.com/dinedal/textql)|Execute SQL against structured text like CSV or TSV|
|[noborus/trdsql](https://github.com/noborus/trdsql)|CLI tool that can execute SQL queries on CSV, LTSV, JSON, YAML and TBLN. Can output to various formats.|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|SQL-like query language for csv|

## Limitations (not supported)

- DDL such as CREATE
- DML such as GRANT
- TCL such as transactions

## Contributing

Thanks for taking the time to contribute! See [CONTRIBUTING.md](./CONTRIBUTING.md) for details. Contributions are not only about code; a GitHub Star also motivates development.

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## How to develop

See the [developer documentation](https://nao1215.github.io/sqly/). When adding features or fixing bugs, please write unit tests; sqly aims for unit-test coverage across all packages, as the tree map shows. The README demos are recorded with [charmbracelet/vhs](https://github.com/charmbracelet/vhs) from `doc/vhs/*.tape` (regenerate with `make demo`), and their commands are exercised end-to-end by the shellspec suite in `spec/` (`make test-e2e`).

![treemap](./doc/img/cover-tree.svg)

If you would like to report a bug or request a feature, please open a [GitHub Issue](https://github.com/nao1215/sqly/issues).

## Libraries used

- [filesql](https://github.com/nao1215/filesql) - SQL database interface for CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel files with automatic type detection and compressed file support
- [prompt](https://github.com/nao1215/prompt) - Powers the interactive shell with SQL completion and command history

## LICENSE

The sqly project is licensed under the terms of [MIT LICENSE](./LICENSE).

## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://debimate.jp/"><img src="https://avatars.githubusercontent.com/u/22737008?v=4?s=75" width="75px;" alt="CHIKAMATSU Naohiro"/><br /><sub><b>CHIKAMATSU Naohiro</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Code">💻</a> <a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Wozzardman"><img src="https://avatars.githubusercontent.com/u/128730409?v=4?s=75" width="75px;" alt="Wozzardman"/><br /><sub><b>Wozzardman</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=Wozzardman" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/edsilegxrepo"><img src="https://avatars.githubusercontent.com/u/153197739?v=4?s=75" width="75px;" alt="edsilegxrepo"/><br /><sub><b>edsilegxrepo</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=edsilegxrepo" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Marukome0743"><img src="https://avatars.githubusercontent.com/u/113521295?v=4?s=75" width="75px;" alt="まるこめ"/><br /><sub><b>まるこめ</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=Marukome0743" title="Code">💻</a></td>
    </tr>
  </tbody>
  <tfoot>
    <tr>
      <td align="center" size="13px" colspan="7">
        <img src="https://raw.githubusercontent.com/all-contributors/all-contributors-cli/1b8533af435da9854653492b1327a23a4dbd0a10/assets/logo-small.svg">
          <a href="https://all-contributors.js.org/docs/en/bot/usage">Add your contributions</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!
