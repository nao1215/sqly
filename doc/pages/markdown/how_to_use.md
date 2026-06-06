### sqly behavior

If no SQL query is specified with the `--sql` option, sqly will start the sqly shell. sqly determines the file type to be loaded from the extension when the shell starts and automatically begins importing it into the SQLite3 in-memory database. Multiple files can be loaded simultaneously. The table names will be the file names (without extensions) or the Excel sheet names. If an SQL query is specified with the `--sql` option, the SQL query result will be displayed in the terminal and sqly will exit without starting the sqly shell.

Flags may appear before or after the file and directory arguments; `sqly --csv data.csv` and `sqly data.csv --csv` are equivalent. A misplaced unknown flag fails with a parse error instead of being read as a file path.

sqly allows you to change the display mode of SQL results with options. By default, the output is in table format. The output format can be changed to csv (`--csv`), tsv (`--tsv`), ltsv (`--ltsv`), markdown (`--markdown`), json (`--json`), or ndjson (`--ndjson`). Excel (`--excel`) and Parquet (`--parquet`) are export-only: they render as csv on screen and write a file only through `.dump` or `--output`. Since the output mode can be changed while the sqly shell is running, it is easy to execute `sqly sample.csv` and then change settings or execute SQL queries within the sqly shell.

For automation-friendly output, `--json-typed` and `--ndjson-typed` (or `.mode json-typed` / `.mode ndjson-typed` in the shell) emit native JSON scalars instead of strings: a canonical JSON number becomes a number, `true`/`false` become booleans, and a SQL NULL becomes `null`. A large integer stays lossless and never regresses into scientific notation, while a value with a leading zero such as `007` remains a string. The default `--json`/`--ndjson` keep the legacy string contract. Pair `--inspect` with `--json-typed` to apply the same contract to the report's sample rows.


### sqly options

```shell
$ sqly --help
sqly - execute SQL against CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel/ACH/Fedwire with shell

[Usage]
  sqly [OPTIONS] [FILE_PATH(S)|DIRECTORY_PATH(S)]

[Example]
  - run sqly shell
    sqly
  - Execute query for csv file
    sqly --sql 'SELECT * FROM sample' ./path/to/sample.csv

[OPTIONS]
  -c, --csv             change output format to csv (default: table)
  -e, --excel           change output format to excel (default: table)
  -l, --ltsv            change output format to ltsv (default: table)
  -m, --markdown        change output format to markdown table (default: table)
  -t, --tsv             change output format to tsv (default: table)
  -j, --json            change output format to json (default: table)
  -n, --ndjson          change output format to ndjson (default: table)
  -p, --parquet         export results as parquet (export-only; use with --output or .dump)
      --json-typed      change output format to json with native scalars (numbers, booleans, nulls) instead of strings
      --ndjson-typed    change output format to ndjson with native scalars (numbers, booleans, nulls) instead of strings
  -S, --sheet string    excel sheet name you want to import
      --stdin string    treat stdin as an input dataset of this format (csv|tsv|ltsv|json|jsonl)
      --stdin-name string   table name for the --stdin dataset (default "stdin")
  -s, --sql string      sql query you want to execute
  -f, --sql-file string   path to a file with SQL to execute (multiline; cannot be used with --sql)
  -o, --output string   destination path for SQL results specified in --sql option
  -i, --inspect         print a JSON report of imported tables (schema, row counts, sample rows) and exit
      --inspect-sample int  rows to include per table in --inspect (0 for schema only) (default 5)
      --save                after the run, write each table back over its source file (requires --force)
      --save-dir string     after the run, write each table into this directory (originals untouched)
      --force               allow --save to overwrite source files in place
  -h, --help            print help message
  -v, --version         print sqly version

[LICENSE]
  MIT LICENSE - Copyright (c) 2022 CHIKAMATSU Naohiro
  https://github.com/nao1215/sqly/blob/main/LICENSE

[CONTACT]
  https://github.com/nao1215/sqly/issues

sqly runs the DB in SQLite3 in-memory mode.
So, SQL supported by sqly is the same as SQLite3 syntax.
```

### Execute sql in terminal: --sql option

`--sql` option takes an SQL statement as an optional argument.

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

`--inspect` imports the given files and directories and prints a JSON report of every table, then exits without starting the shell. The report lists each table name, its source path, the column schema, the row count, and a small sample of rows. It gives scripts and LLMs a non-interactive equivalent of `.tables`, `.schema`, and `.describe`. Import progress goes to stderr, so stdout carries only the JSON.

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

Multi-table sources map several tables to one source path: Excel sheets and ACH/Fedwire files.

`--inspect-sample N` controls how many sample rows each table includes (default 5). Use `--inspect-sample 0` for a schema-only report, which avoids printing a large sample for wide sources such as Fedwire.

```shell
$ sqly --inspect --inspect-sample 0 testdata/customer-transfer.fed
```

### Write changes back to files: --save and --save-dir

By default a session is in-memory only: DML such as `UPDATE`, `INSERT`, and `DELETE` changes the loaded tables but never touches the files. Persist changes with explicit, opt-in flags.

`--save-dir DIR` writes each table into DIR after the run, preserving each source's format and compression and the source file name. The original files are not modified.

```shell
$ sqly --sql "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1" --save-dir ./out testdata/user.csv
```

`--save` overwrites the source files in place. Because it is destructive, it requires `--force`.

```shell
$ sqly --sql "DELETE FROM user WHERE identifier > 100" --save --force testdata/user.csv
```

In the interactive shell, `.save DIR` writes to a directory and `.save --force` overwrites the sources.

The save flags apply after `--sql` and batch runs. Only tables that map one to one to a single csv, tsv, ltsv, or parquet source are written, and the source's compression (for example `.csv.gz`) is preserved. Tables created by SQL, tables imported from a directory, and multi-table sources (Excel, ACH, Fedwire) are rejected with a clear error before anything is written, so a session is never partially saved.

### Batch mode: pipe commands via stdin

When stdin is not a terminal (piped or redirected), sqly reads SQL statements and helper commands from stdin instead of starting the shell. SQL statements end at a top-level `;` and may span multiple lines, so formatted queries and CTEs work; separate multiple statements with `;`. Helper commands such as `.tables` are single-line. A single trailing statement without `;` still runs. A failed statement makes sqly exit non-zero.

```shell
$ printf 'WITH x AS (\n  SELECT user_name FROM user\n)\nSELECT * FROM x;\n' | sqly testdata/user.csv
```

### Pipe data into sqly: --stdin option

By default piped stdin is read as SQL and helper commands (batch mode). Use `--stdin <format>` to treat stdin as an input dataset instead. The format is given explicitly (`csv`, `tsv`, `ltsv`, `json`, or `jsonl`) because a pipe has no filename to detect it from. The table defaults to `stdin`; override it with `--stdin-name`. Piped data can be joined with file and directory arguments.

```shell
$ cat testdata/user.csv | sqly --stdin csv --sql "SELECT user_name FROM stdin LIMIT 1"
+-----------+
| user_name |
+-----------+
| booker12  |
+-----------+
```

### Command history

The sqly shell persists command history to a SQLite database under the config directory. History is best-effort: if that database cannot be created or written, or stops accepting reads or writes mid-session (for example a read-only config directory in CI or a container), sqly disables history for the rest of the session with a warning and still runs the requested query or command. Set `SQLY_HISTORY_DB_PATH` to choose a writable location.

### Change output format

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins

$ sqly --sql "SELECT * FROM user LIMIT 2" --json testdata/user.csv 
[
   {
      "first_name": "Rachel",
      "identifier": "1",
      "last_name": "Booker",
      "user_name": "booker12"
   },
   {
      "first_name": "Mary",
      "identifier": "2",
      "last_name": "Jenkins",
      "user_name": "jenkins46"
   }
]

$ sqly --sql "SELECT * FROM user LIMIT 2" --json testdata/user.csv > user.json

$ sqly --sql "SELECT * FROM user LIMIT 2" --csv user.json 
first_name,identifier,last_name,user_name
Rachel,1,Booker,booker12
Mary,2,Jenkins,jenkins46
```

> [!WARNING]
> The support for JSON is limited. There is a possibility of discontinuing JSON support in the future.



### Output sql result to file

#### For linux user 

The sqly can save SQL execution results to the file using shell redirection. The --csv option outputs SQL execution results in CSV format instead of table format.

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### For windows user

The sqly can save SQL execution results to the file using the --output option. The --output option specifies the destination path for SQL results specified in the --sql option.

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```

### Infer export format and compression from the output path

When no output mode flag is given, sqly infers the export format and compression from the `--output` path extension, and the shell `.dump` command applies the same rules.

```shell
$ sqly --sql "SELECT * FROM user" --output result.parquet testdata/user.csv
$ sqly --sql "SELECT * FROM user" --output result.ndjson.gz testdata/user.csv
```

Text and JSON formats (csv, tsv, ltsv, json, ndjson, markdown) support the compression wrappers `.gz`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, and `.lz4`. An explicit mode flag that disagrees with the path extension is rejected instead of writing a surprising format. Bzip2 output and compression on Parquet or Excel are rejected with a clear error.
