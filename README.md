<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-4-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nao1215/sqly)](https://goreportcard.com/report/github.com/nao1215/sqly)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
![demo](./doc/img/demo.gif)  

sqly is a command-line tool that executes SQL against CSV, TSV, LTSV, JSON, JSONL, Parquet, Microsoft Excel, ACH, and Fedwire files. It imports those files into an [SQLite3](https://www.sqlite.org/index.html) in-memory database. Compressed files (.gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4) are also supported. CTE (WITH clause) is available for complex queries.

sqly has an interactive shell (sqly-shell) with SQL completion and command history. You can also execute SQL directly from the command line without the shell.

```shell
sqly --sql "SELECT * FROM data" data.csv.gz
sqly --sql "SELECT * FROM logs WHERE level='ERROR'" logs.tsv.bz2
```

## How to install
### Use "go install"
```shell
go install github.com/nao1215/sqly@latest
```

### Use homebrew
```shell
brew install nao1215/tap/sqly
```

## Supported OS & go version
- Windows
- macOS
- Linux
- go1.25.0 or later

## How to use
The sqly automatically imports CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel/ACH/Fedwire files (including compressed versions for tabular formats) into the DB when you pass file paths or directory paths as arguments. You can also mix files and directories in the same command. DB table name is the same as the file name or sheet name (e.g., if you import user.csv, sqly command create the user table).

**Note**: Table names are sanitized for SQL compatibility. Spaces, hyphens (`-`), and dots (`.`) are replaced with underscores (`_`). Other special characters (e.g., `@`, `#`, `$`) are removed. If the resulting name starts with a digit, a `sheet_` prefix is added.

Examples:
- `bug-syntax-error.csv` → table `bug_syntax_error`
- `2023-data.csv` → table `sheet_2023_data`
- `data@v2.csv` → table `datav2`

### Excel Sheet Names
When importing Excel files, table names are created in the format `filename_sheetname`. Sheet names are also sanitized for SQL compatibility:
- Spaces, hyphens, and dots are replaced with underscores
- Non-ASCII characters (such as accented characters like `é`) are removed

For example:
- File `data.xlsx` with sheet `A test` → table `data_A_test`
- File `report.xlsx` with sheet `Café` → table `report_Caf`

You can specify a sheet name using the `--sheet` option with the original name (before sanitization):
```shell
$ sqly data.xlsx --sheet="A test"
$ sqly report.xlsx --sheet="Café"
```

The sqly automatically determines the file format from the file extension, including compressed files.

### ACH Files
ACH (Automated Clearing House) files (`.ach`) are loaded as multiple tables for easy querying:
- `{filename}_file_header` — file-level header (1 row)
- `{filename}_batches` — batch header information
- `{filename}_entries` — entry detail records (main transaction data)
- `{filename}_addenda` — addenda records

For IAT (International ACH Transactions), additional tables are created: `{filename}_iat_batches`, `{filename}_iat_entries`, `{filename}_iat_addenda`.

```shell
$ sqly ppd-debit.ach
$ sqly --sql "SELECT * FROM ppd_debit_entries WHERE amount > 10000" ppd-debit.ach
```

### Fedwire Files
Fedwire files (`.fed`) are loaded as a single message table:
- `{filename}_message` — flat table with all FEDWireMessage fields

```shell
$ sqly customer-transfer.fed
$ sqly --sql "SELECT * FROM customer_transfer_message" customer-transfer.fed
```

### Execute sql in terminal: --sql option
--sql option takes an SQL statement as an optional argument.

```shell
$ sqly --sql "SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id" testdata/user.csv testdata/identifier.csv
+-----------+-----------+
| user_name | position  |
+-----------+-----------+
| booker12  | developrt |
| jenkins46 | manager   |
| smith79   | neet      |
+-----------+-----------+
```

### Load SQL from a file: --sql-file option
`--sql-file PATH` runs SQL read from a file instead of from `--sql` or stdin. The file may contain multiple statements separated by `;`, and a statement may span multiple lines, following the same rules as batch stdin mode; a leading header comment is allowed. It cannot be combined with `--sql`, and a missing, unreadable, or empty file fails with a clear error.

Because the query comes from a file, stdin is free to carry a dataset. Combine it with `--stdin <format>` to join piped data:

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

### Inspect tables: --inspect option
`--inspect` imports the given files and directories and prints a JSON report of every table, then exits without starting the shell. The report lists each table name, its source path, the column schema, the row count, and a small sample of rows. It gives scripts and LLMs a non-interactive equivalent of `.tables`, `.schema`, and `.describe`. Import progress goes to stderr, so stdout carries only the JSON. Excel sheets and ACH/Fedwire files map several tables to one source path. `--inspect-sample N` sets how many sample rows each table includes (default 5); `--inspect-sample 0` produces a schema-only report for wide or multi-table sources.

```shell
$ sqly --inspect testdata/user.csv
{
  "tables": [
    {
      "name": "user",
      "source": "testdata/user.csv",
      "row_count": 3,
      "columns": [
        {"name": "user_name", "type": "TEXT", "nullable": true, "primary_key": false},
        {"name": "identifier", "type": "INTEGER", "nullable": true, "primary_key": false}
      ],
      "sample_rows": [
        {"user_name": "booker12", "identifier": "1"}
      ]
    }
  ]
}
```

### Write changes back to files: --save and --save-dir
A session is in-memory only by default: `UPDATE`/`INSERT`/`DELETE` change the loaded tables but never touch the files. Persist changes with explicit, opt-in flags. `--save-dir DIR` writes each table into DIR after the run, preserving each source's format, compression, and file name, and leaves the originals untouched. `--save` overwrites the source files in place and requires `--force`. In the interactive shell, `.save DIR` and `.save --force` do the same.

```shell
$ sqly --sql "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1" --save-dir ./out testdata/user.csv
$ sqly --sql "DELETE FROM user WHERE identifier > 100" --save --force testdata/user.csv
```

Only tables that map one to one to a single csv, tsv, ltsv, or parquet source are written, with the source's compression (for example `.csv.gz`) preserved. Tables created by SQL, tables from a directory import, and multi-table sources (Excel, ACH, Fedwire) are rejected with a clear error before anything is written.

### Batch mode: pipe commands via stdin
When standard input is not a terminal (piped or redirected), sqly reads SQL statements and shell commands from stdin instead of starting the interactive shell. SQL statements end at a top-level `;` and may span multiple lines (separate multiple statements with `;`); helper commands such as `.tables` are single-line. A single trailing statement without `;` still runs. A failed statement makes sqly exit non-zero, so batch runs are scriptable.

```shell
$ echo "SELECT * FROM user LIMIT 1" | sqly testdata/user.csv
+-----------+------------+------------+-----------+
| user_name | identifier | first_name | last_name |
+-----------+------------+------------+-----------+
| booker12  |          1 | Rachel     | Booker    |
+-----------+------------+------------+-----------+

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

# Multiline SQL terminated by ;
$ printf 'WITH x AS (\n  SELECT user_name FROM user\n)\nSELECT * FROM x;\n' | sqly testdata/user.csv
```

### Pipe data into sqly: --stdin option
By default piped stdin is read as SQL and shell commands (batch mode above). Use `--stdin <format>` to treat stdin as an input dataset instead. The format is given explicitly (`csv`, `tsv`, `ltsv`, `json`, or `jsonl`) because a pipe has no filename to detect it from. The table defaults to `stdin`; override it with `--stdin-name`. Piped data can be joined with file and directory arguments.

```shell
$ cat testdata/user.csv | sqly --stdin csv --sql "SELECT user_name FROM stdin LIMIT 1"
+-----------+
| user_name |
+-----------+
| booker12  |
+-----------+

# Join piped stdin with a file
$ cat testdata/user.csv | sqly --stdin csv --sql "SELECT s.user_name, i.position FROM stdin s JOIN identifier i ON s.identifier = i.id" testdata/identifier.csv
```

### Directory import
You can import entire directories containing supported files. The sqly automatically detects all supported files (CSV, TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH, Fedwire, including compressed versions) in the directory recursively and imports them:

```shell
# Import all files from a directory
$ sqly ./data_directory

# Mix files and directories
$ sqly file1.csv ./data_directory file2.tsv

# Use with --sql option
$ sqly ./data_directory --sql "SELECT * FROM users"
```

### Interactive shell: .import command
In the sqly shell, you can use the `.import` command to import files or directories:

```shell
sqly:~/data$ .import ./csv_files
Successfully imported 3 tables from directory ./csv_files: [users products orders]

sqly:~/data$ .import file1.csv ./directory file2.tsv
# Imports file1.csv, all files from directory, and file2.tsv

# Quote arguments that contain spaces
sqly:~/data$ .import "my data.csv"
sqly:~/data$ .import --sheet "Q1 Sales" report.xlsx

sqly:~/data$ .tables
orders
products
users
```

### Change output format
The sqly output sql query results in following formats:
- ASCII table format (default)
- CSV format (--csv option)
- TSV format (--tsv option)
- LTSV format (--ltsv option)
- JSON format (--json option)
- NDJSON format (--ndjson option)
- Parquet export (--parquet option, export-only)

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins
```

JSON and NDJSON are easy to consume from scripts and tools. Values are emitted as strings.

```shell
$ sqly --sql "SELECT user_name, identifier FROM user LIMIT 2" --json testdata/user.csv
[
  {"user_name":"booker12","identifier":"1"},
  {"user_name":"jenkins46","identifier":"2"}
]

$ sqly --sql "SELECT user_name, identifier FROM user LIMIT 2" --ndjson testdata/user.csv
{"user_name":"booker12","identifier":"1"}
{"user_name":"jenkins46","identifier":"2"}
```

In the shell, switch with `.mode json` or `.mode ndjson`. `.dump` writes the current mode to a file (`.json`/`.ndjson`).

Parquet is export-only, like Excel: `.mode parquet` (or `--parquet`) renders as CSV on screen and writes a `.parquet` file through `.dump` or `--output`. sqly can re-import the file. An empty result cannot be exported because Parquet needs at least one row to infer its schema.

```shell
$ sqly --parquet --output result.parquet --sql "SELECT * FROM user" testdata/user.csv
Output sql result to result.parquet (output mode=parquet)
```

### Run sqly shell
The sqly shell starts when you run the sqly command without the --sql option. When you execute sqly command with file path, the sqly-shell starts after importing the file into the SQLite3 in-memory database.  

```shell
$ sqly 
sqly v0.10.0

enter "SQL query" or "sqly command that begins with a dot".
.help print usage, .exit exit sqly.

sqly:~/github/github.com/nao1215/sqly(table)$ 
```
  
The sqly shell functions similarly to a common SQL client (e.g., `sqlite3` command or `mysql` command). The sqly shell has helper commands that begin with a dot. The sqly-shell also supports command history, and input completion.  

Command history is persisted to a SQLite database under the config directory. History is best-effort: if that database cannot be created or written, or stops accepting reads or writes mid-session (for example a read-only config directory in CI or a container), sqly disables history for the rest of the session with a warning and still runs the requested query or command. Set `SQLY_HISTORY_DB_PATH` to choose a writable location.

The sqly-shell has the following helper commands:

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
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
    .schema: print CREATE TABLE statement of a table
    .tables: print tables
```

### Inspect table schema
Use `.schema` to see a table's `CREATE TABLE` statement and `.describe` to list its columns. Both work for every imported format (CSV, JSON, Excel, ACH, Fedwire). In `.mode json` they emit structured output.

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

### Output sql result to file
#### For linux user 
The sqly can save SQL execution results to the file using shell redirection. The --csv option outputs SQL execution results in CSV format instead of table format.
```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### For windows user

The sqly can save SQL execution results to the file using the --output option. The --output option specifies the destination path for SQL results specified in the --sql option. Flags may come before or after the file arguments, so `--output` also works at the end of the command.

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv
$ sqly --sql "SELECT * FROM user" testdata/user.csv --output=test.csv
```

The format and compression are inferred from the `--output` path when no output mode flag is given, so the extension alone selects the writer. The same inference applies to the shell `.dump` command.

```shell
$ sqly --sql "SELECT * FROM user" --output result.parquet testdata/user.csv
$ sqly --sql "SELECT * FROM user" --output result.ndjson.gz testdata/user.csv
```

Text and JSON formats support `.gz`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, and `.lz4`. An explicit mode flag that disagrees with the path extension is rejected, as are `.bz2` and compression on Parquet or Excel.

### Key Binding for sqly-shell
|Key Binding	|Description|
|:--|:--|
|Ctrl + A	|Go to the beginning of the line (Home)|
|Ctrl + E	|Go to the end of the line (End)|
|Ctrl + P	|Previous command (Up arrow)|
|Ctrl + N	|Next command (Down arrow)|
|Ctrl + F	|Forward one character|
|Ctrl + B	|Backward one character|
|Ctrl + D	|Delete character under the cursor|
|Ctrl + H	|Delete character before the cursor (Backspace)|
|Ctrl + W	|Cut the word before the cursor to the clipboard|
|Ctrl + K	|Cut the line after the cursor to the clipboard|
|Ctrl + U	|Cut the line before the cursor to the clipboard|
|Ctrl + L	|Clear the screen|  
|TAB        |Completion|
|↑          |Previous command|
|↓          |Next command|

### Supported file formats

| Format | Extensions | Notes |
|:--|:--|:--|
| CSV | `.csv` | |
| TSV | `.tsv` | |
| LTSV | `.ltsv` | |
| JSON | `.json` | Stored in `data` column; use `json_extract()` to query |
| JSONL | `.jsonl` | Stored in `data` column; use `json_extract()` to query |
| Parquet | `.parquet` | |
| Excel | `.xlsx` | Each sheet becomes a separate table |
| ACH | `.ach` | Creates multiple tables (`_file_header`, `_batches`, `_entries`, `_addenda`) |
| Fedwire | `.fed` | Creates a single `_message` table |

CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel also support the following compression extensions: `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, `.lz4`
(e.g. `.csv.gz`, `.tsv.bz2`, `.ltsv.xz`)

## Benchmark
CPU: AMD Ryzen 5 3400G with Radeon Vega Graphics  
Execute: 
```sql
SELECT * FROM `table` WHERE `Index` BETWEEN 1000 AND 2000 ORDER BY `Index` DESC LIMIT 1000
```

|Records  | Columns | Time per Operation | Memory Allocated per Operation | Allocations per Operation |
|---------|----|-------------------|--------------------------------|---------------------------|
|100,000|   12|  1715818835 ns/op  |      441387928 B/op   |4967183 allocs/op | 
|1,000,000|   9|   11414332112 ns/op |      2767580080 B/op | 39131122 allocs/op |


## Alternative Tools
|Name| Description|
|:--|:--|
|[nao1215/sqluv](https://github.com/nao1215/sqluv)|Simple terminal UI for DBMS and local CSV/TSV/LTSV|
|[harelba/q](https://github.com/harelba/q)|Run SQL directly on delimited files and multi-file sqlite databases|
|[dinedal/textql](https://github.com/dinedal/textql)|Execute SQL against structured text like CSV or TSV|
|[noborus/trdsql](https://github.com/noborus/trdsql)|CLI tool that can execute SQL queries on CSV, LTSV, JSON, YAML and TBLN. Can output to various formats.|
|[mithrandie/csvq](https://github.com/mithrandie/csvq)|SQL-like query language for csv|


## Limitions (Not support)

- DDL such as CREATE
- DML such as GRANT
- TCL such as Transactions

## Contributing

First off, thanks for taking the time to contribute! See [CONTRIBUTING.md](./CONTRIBUTING.md) for more information. Contributions are not only related to development. For example, GitHub Star motivates me to develop! 

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

## How to develop

Please see the [document](https://nao1215.github.io/sqly/), section "Document for developers".

When adding new features or fixing bugs, please write unit tests. The sqly is unit tested for all packages as the unit test tree map below shows.

![treemap](./doc/img/cover-tree.svg)


### Contact
If you would like to send comments such as "find a bug" or "request for additional features" to the developer, please use one of the following contacts.

- [GitHub Issue](https://github.com/nao1215/sqly/issues)

## Libraries Used

**sqly** leverages powerful Go libraries to provide its functionality:
- [filesql](https://github.com/nao1215/filesql) - Provides SQL database interface for CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel files with automatic type detection and compressed file support
- [prompt](https://github.com/nao1215/prompt) - Powers the interactive shell with SQL completion and command history features

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
